package milon

import (
	"context"
	"fmt"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/gen"
	"github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/provider"
	"github.com/milon-labs/milon-go-sdk/types"
	"sync"
	"sync/atomic"
	"time"
)

type RpcClientImpl interface {
	GetAllPd() map[string]*provider.Provider
	GetProviderManager() *provider.IDLRegistry

	ClaimFaucet(accountSk crypto.SecretKeyer, account *crypto.Address, mode lib.AccountSignatureMode) error
	CreateAccount(accountSk crypto.SecretKeyer, pk *crypto.PublicKey) error

	BalanceOf(account *crypto.Address, token ...*crypto.Address) (uint64, error)
	ListAccountSigners(account *crypto.Address) ([]any, error)
	AccountSignerBit(account *crypto.Address) (types.Bitmap64, error)
	TokenMetadata(token *crypto.Address) (gen.TokenMetadata, error)

	GetChainHead(opts ...RequestOption) (*ChainHeadResult, error)
	SubmitTx(transaction *lib.Transaction, opts ...RequestOption) (*SubmitTxResult, error)
	SubmitTxWithSponsorIxes(tx *lib.Transaction, sponsorIx []uint8, opts ...RequestOption) (*SubmitTxResult, error)
	SubmitAndWait(tx *lib.Transaction, opts ...WaitOption) (*api.TxHistory, error)
	BatchSubmitTx(txs []*lib.Transaction, opts ...RequestOption) (*BatchSubmitTxResult, error)
	SimulateTx(transaction *lib.Transaction, opts ...RequestOption) (*SimulateTxResult, error)
	BatchSimulateTx(txs []*lib.Transaction, opts ...RequestOption) (*BatchSimulateTxResult, error)
	View(wire api.PackedInstruction, opts ...RequestOption) (*ViewSingleResult, error)
	BatchView(wires []api.PackedInstruction, opts ...RequestOption) (*ViewResult, error)
	GetAccount(accountRelaxed any, opts ...RequestOption) (*GetAccountResult, error)
	BatchGetAccount(accountRelaxedList []any, opts ...RequestOption) (*BatchGetAccountResult, error)
	EventsByTxHash(txHashRelaxed any, typeTagFilter *uint64, opts ...RequestOption) (*EventsByTxHashResult, error)
	BatchEventsByTxHash(reqList []api.EventsByTxHashReq, opts ...RequestOption) (*BatchEventsByTxHashResult, error)
	GetNodeCapability(opts ...RequestOption) (*NodeCapabilityResult, error)

	GetBlockByHeight(blockHeight uint64, opts ...RequestOption) (*GetBlockByHeightResult, error)
	BatchGetBlockByHeight(heights []uint64, opts ...RequestOption) (*BatchGetBlockByHeightResult, error)
	GetTxByHash(txHashOrTxIdRelaxed any, opts ...RequestOption) (*GetTxByHashResult, error)
	BatchGetTxByHash(txHashOrTxIdRelaxedList []any, opts ...RequestOption) (*BatchGetTxByHashResult, error)
	GetTxHistoryProof(txHashOrTxIdRelaxed any, opts ...RequestOption) (*GetTxHistoryProofResult, error)
	BatchGetTxHistoryProof(txHashOrTxIdRelaxedList []any, opts ...RequestOption) (*BatchGetTxHistoryProofResult, error)

	GetResource(rsHash api.RsHash, opts ...RequestOption) (*GetResourceResult, error)
	BatchGetResource(rsHashList []api.RsHash, opts ...RequestOption) (*BatchGetResourceResult, error)
	GetResourcePathByHash(rsHash api.RsHash, opts ...RequestOption) (*GetResourcePathByHashResult, error)
	BatchGetResourcePathByHash(rsHashList []api.RsHash, opts ...RequestOption) (*BatchGetResourcePathByHashResult, error)
	GetAccessValue(blobHash api.BlobHash, opts ...RequestOption) (*GetAccessValueResult, error)
	BatchGetAccessValue(blobHashList []api.BlobHash, opts ...RequestOption) (*BatchGetAccessValueResult, error)

	WaitForTransaction(txHashOrTxIdRelaxed any, opts ...WaitOption) (*GetTxByHashResult, error)
}

type Client struct {
	// RpcClientImpl is embedded rather than wrapped: every RPC method is
	// promoted onto Client, so client.SubmitTx(...) works without a forwarding
	// layer. Build clients with NewClient/MustNewClient; the field is only
	// assignable to substitute an implementation in tests.
	RpcClientImpl
	network Network
}

// Client promotes the entire RPC surface through the embedded implementation, so
// callers write client.SubmitTx(...) rather than client.RpcClient.SubmitTx(...).
// The assertion keeps that promise checked at compile time when the interface
// grows.
var _ RpcClientImpl = (*Client)(nil)

// ========================================
// ClientOption — for NewClient
// ========================================

type ClientOption func(*clientOptions)

type clientOptions struct {
	pollPeriod  time.Duration
	pollTimeout time.Duration
}

// NewClient creates a client bound to config. It returns an error when the
// built-in IDLs cannot be loaded or bound, so a broken IDL set is reported to
// the caller instead of crashing the process; MustNewClient is the panicking
// convenience form.
//
// The chain id and the generated instruction objects (gen.*) are process-wide:
// every call rebinds them, so a process supports a single Network. Creating a
// client for a different chain id is rejected rather than silently corrupting
// the transaction and instruction hashes of the clients already in use.
func NewClient(config Network, options ...ClientOption) (*Client, error) {
	if config.RpcUrl == "" {
		return nil, fmt.Errorf("network config requires RpcUrl")
	}
	if config.ChainId == 0 {
		return nil, fmt.Errorf("network config requires ChainId: it is mixed into every transaction hash")
	}
	if err := reserveChainID(config.ChainId); err != nil {
		return nil, err
	}
	if err := lib.SetChainId(config.ChainId); err != nil {
		return nil, err
	}

	opts := &clientOptions{
		pollPeriod:  1 * time.Second,
		pollTimeout: 30 * time.Second,
	}
	for _, opt := range options {
		if opt != nil {
			opt(opts)
		}
	}

	rpc := &rpcClientV1{
		network:           config,
		providerByIDLName: make(map[string]*provider.Provider),
		pollPeriod:        opts.pollPeriod,
		pollTimeout:       opts.pollTimeout,
	}

	if err := rpc.LoadIDLsFromData(gen.DefaultIDLs); err != nil {
		return nil, fmt.Errorf("failed to load generated IDLs: %w", err)
	}

	// GetAllPd copies the provider map; take it once and share it across the
	// resolver, the registry and the generated-app binding, which all read the
	// same construction-time snapshot.
	allPd := rpc.GetAllPd()

	rpc.typeResolver = &provider.IDLTypeResolver{
		Providers: allPd,
	}

	idlManager, err := provider.NewIDLRegistry(allPd)
	if err != nil {
		return nil, fmt.Errorf("failed to build IDL registry: %w", err)
	}
	rpc.providerManager = idlManager

	if err = gen.BindAll(allPd); err != nil {
		return nil, fmt.Errorf("failed to bind generated IDL apps: %w", err)
	}

	return &Client{
		RpcClientImpl: rpc,
		network:       config,
	}, nil
}

// MustNewClient is NewClient that panics on failure. Use it for static
// configuration (examples, package-level initialisation) where a failure is
// not recoverable.
func MustNewClient(config Network, options ...ClientOption) *Client {
	client, err := NewClient(config, options...)
	if err != nil {
		panic(err)
	}
	return client
}

var (
	chainIDMu       sync.Mutex
	reservedChainID uint64
	chainIDReserved bool
)

// reserveChainID records the process-wide chain id used when hashing
// transactions and instructions. The id is shared by every transaction, so two
// clients bound to networks with different ids would silently produce hashes
// for the wrong chain; reject that configuration.
func reserveChainID(chainID uint64) error {
	chainIDMu.Lock()
	defer chainIDMu.Unlock()

	if chainIDReserved && reservedChainID != chainID {
		return fmt.Errorf("chain id %d conflicts with the already initialised chain id %d: a process can only use one Network", chainID, reservedChainID)
	}
	reservedChainID = chainID
	chainIDReserved = true
	return nil
}

// WithClientPollPeriod sets the default polling interval for WaitForTransaction.
func WithClientPollPeriod(period time.Duration) ClientOption {
	return func(o *clientOptions) {
		o.pollPeriod = period
	}
}

// WithClientPollTimeout sets the default polling timeout for WaitForTransaction.
func WithClientPollTimeout(timeout time.Duration) ClientOption {
	return func(o *clientOptions) {
		o.pollTimeout = timeout
	}
}

// ========================================
// RequestOption — for RPC methods (type-safe)
// ========================================

type requestOptions struct {
	ctx       context.Context
	requestID lib.RequestID
}

// RequestOption configures a single RPC call.
type RequestOption func(*requestOptions)

var requestIDSeq atomic.Uint64

// nextRequestID returns a process-unique request id: millisecond timestamp in
// the high bits plus a per-millisecond sequence in the low 20 bits.
func nextRequestID() lib.RequestID {
	seq := requestIDSeq.Add(1) & 0xFFFFF
	return lib.RequestID(uint64(time.Now().UnixMilli())<<20 | seq)
}

func applyRequestOptions(opts []RequestOption) requestOptions {
	o := requestOptions{
		ctx:       context.Background(),
		requestID: nextRequestID(),
	}
	for _, fn := range opts {
		if fn != nil {
			fn(&o)
		}
	}
	// A nil context would panic on every downstream Done()/Err() call, so fall
	// back to the background context instead of letting it propagate.
	if o.ctx == nil {
		o.ctx = context.Background()
	}
	return o
}

// WithContext sets the context for the RPC call (timeout, cancellation, etc.).
func WithContext(ctx context.Context) RequestOption {
	return func(o *requestOptions) {
		o.ctx = ctx
	}
}

// WithRequestID sets a custom request ID for the RPC call.
func WithRequestID(id lib.RequestID) RequestOption {
	return func(o *requestOptions) {
		o.requestID = id
	}
}

// ========================================
// WaitOption — for WaitForTransaction (type-safe)
// ========================================

type waitOptions struct {
	ctx             context.Context
	requestID       lib.RequestID
	requestIDPinned bool // true when the caller supplied an explicit request id
	pollPeriod      time.Duration
	pollTimeout     time.Duration
}

// WaitOption configures WaitForTransaction.
type WaitOption func(*waitOptions)

func (c *rpcClientV1) applyWaitOptions(opts []WaitOption) waitOptions {
	o := waitOptions{
		ctx:         context.Background(),
		requestID:   nextRequestID(),
		pollPeriod:  c.pollPeriod,
		pollTimeout: c.pollTimeout,
	}
	for _, fn := range opts {
		if fn != nil {
			fn(&o)
		}
	}
	// A nil context would panic on every downstream Done()/Err() call, so fall
	// back to the background context instead of letting it propagate.
	if o.ctx == nil {
		o.ctx = context.Background()
	}
	return o
}

// WithWaitContext sets the context for WaitForTransaction.
func WithWaitContext(ctx context.Context) WaitOption {
	return func(o *waitOptions) {
		o.ctx = ctx
	}
}

// WithWaitRequestID sets a custom request ID for WaitForTransaction. By default
// every polling attempt uses a fresh request id; setting one explicitly pins it
// for all attempts.
func WithWaitRequestID(id lib.RequestID) WaitOption {
	return func(o *waitOptions) {
		o.requestID = id
		o.requestIDPinned = true
	}
}

// WithWaitPollPeriod sets the polling interval for WaitForTransaction.
func WithWaitPollPeriod(period time.Duration) WaitOption {
	return func(o *waitOptions) {
		o.pollPeriod = period
	}
}

// WithWaitPollTimeout sets the polling timeout for WaitForTransaction.
func WithWaitPollTimeout(timeout time.Duration) WaitOption {
	return func(o *waitOptions) {
		o.pollTimeout = timeout
	}
}
