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
	"sync/atomic"
	"time"
)

type RpcClientImpl interface {
	GetAllPd() map[string]*provider.Provider
	GetProviderManager() *provider.IDLRegistry

	ClaimFaucet(accountSk crypto.SecretKeyer, account *crypto.Address, mode lib.AccountSignatureMode) error
	CreateAccount(accountSk crypto.SecretKeyer, pk *crypto.PublicKey) error

	BalanceOf(account *crypto.Address) (uint64, error)
	ListAccountSigners(account *crypto.Address) ([]any, error)
	AccountSignerBit(account *crypto.Address) (types.Bitmap64, error)

	GetChainHead(opts ...RequestOption) (*ChainHeadResult, error)
	SubmitTx(transaction *lib.Transaction, opts ...RequestOption) error
	SubmitTxWithSponsorIxes(tx *lib.Transaction, sponsorIx []uint8, opts ...RequestOption) error
	SimulateTx(transaction *lib.Transaction, opts ...RequestOption) (*SimulateTxResult, error)
	View(wires []api.PackedInstruction, opts ...RequestOption) (*ViewResult, error)
	GetAccount(accountRelaxed any, opts ...RequestOption) (*GetAccountResult, error)
	EventsByTxHash(txHashRelaxed any, typeTagFilter *uint64, opts ...RequestOption) (*EventsByTxHashResult, error)

	GetBlockByHeight(blockHeight uint64, opts ...RequestOption) (*GetBlockByHeightResult, error)
	GetTxByHash(txHashOrTxIdRelaxed any, opts ...RequestOption) (*GetTxByHashResult, error)
	GetTxHistoryProof(txHashOrTxIdRelaxed any, opts ...RequestOption) (*GetTxHistoryProofResult, error)

	GetResource(rsHash api.RsHash, opts ...RequestOption) (*GetResourceResult, error)
	GetResourcePathByHash(rsHash api.RsHash, opts ...RequestOption) (*GetResourcePathByHashResult, error)
	BatchGetResourcePathByHash(rsHashList []api.RsHash, opts ...RequestOption) (*BatchGetResourcePathByHashResult, error)
	GetAccessValue(blobHashList []api.BlobHash, opts ...RequestOption) (*GetAccessValueResult, error)

	WaitForTransaction(txHashOrTxIdRelaxed any, opts ...WaitOption) (*GetTxByHashResult, error)
}

type Client struct {
	RpcClient RpcClientImpl
}

// ========================================
// ClientOption — for NewClient
// ========================================

type ClientOption func(*clientOptions)

type clientOptions struct {
	pollPeriod  time.Duration
	pollTimeout time.Duration
}

func NewClient(config Network, options ...ClientOption) *Client {
	lib.SetChainId(config.ChainId)

	opts := &clientOptions{
		pollPeriod:  1 * time.Second,
		pollTimeout: 30 * time.Second,
	}
	for _, opt := range options {
		opt(opts)
	}

	rpc := &rpcClientV1{
		network:           config,
		providerByIDLName: make(map[string]*provider.Provider),
		pollPeriod:        opts.pollPeriod,
		pollTimeout:       opts.pollTimeout,
	}

	if err := rpc.LoadIDLsFromData(gen.DefaultIDLs); err != nil {
		panic("failed to load generated IDLs:" + err.Error())
	}

	rpc.typeResolver = &provider.IDLTypeResolver{
		Providers: rpc.GetAllPd(),
	}

	idlManager, err := provider.NewIDLRegistry(rpc.GetAllPd())
	if err != nil {
		panic(err)
	}
	rpc.providerManager = idlManager

	if err = gen.BindAll(rpc.GetAllPd()); err != nil {
		panic(fmt.Sprintf("failed to bind generated IDL apps: %v", err))
	}

	return &Client{
		RpcClient: rpc,
	}
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
		fn(&o)
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
	ctx         context.Context
	requestID   lib.RequestID
	pollPeriod  time.Duration
	pollTimeout time.Duration
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
		fn(&o)
	}
	return o
}

// WithWaitContext sets the context for WaitForTransaction.
func WithWaitContext(ctx context.Context) WaitOption {
	return func(o *waitOptions) {
		o.ctx = ctx
	}
}

// WithWaitRequestID sets a custom request ID for WaitForTransaction.
func WithWaitRequestID(id lib.RequestID) WaitOption {
	return func(o *waitOptions) {
		o.requestID = id
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

// ========================================
// Client delegation methods
// ========================================

func (client *Client) GetAllPd() map[string]*provider.Provider {
	return client.RpcClient.GetAllPd()
}
func (client *Client) GetProviderManager() *provider.IDLRegistry {
	return client.RpcClient.GetProviderManager()
}

func (client *Client) ClaimFaucet(accountSk crypto.SecretKeyer, account *crypto.Address, mode lib.AccountSignatureMode) error {
	return client.RpcClient.ClaimFaucet(accountSk, account, mode)
}
func (client *Client) CreateAccount(accountSk crypto.SecretKeyer, pk *crypto.PublicKey) error {
	return client.RpcClient.CreateAccount(accountSk, pk)
}

func (client *Client) BalanceOf(account *crypto.Address) (uint64, error) {
	return client.RpcClient.BalanceOf(account)
}
func (client *Client) ListAccountSigners(account *crypto.Address) ([]any, error) {
	return client.RpcClient.ListAccountSigners(account)
}
func (client *Client) AccountSignerBit(account *crypto.Address) (types.Bitmap64, error) {
	return client.RpcClient.AccountSignerBit(account)
}

func (client *Client) GetChainHead(opts ...RequestOption) (*ChainHeadResult, error) {
	return client.RpcClient.GetChainHead(opts...)
}
func (client *Client) SubmitTx(transaction *lib.Transaction, opts ...RequestOption) error {
	return client.RpcClient.SubmitTx(transaction, opts...)
}
func (client *Client) SubmitTxWithSponsorIxes(tx *lib.Transaction, sponsorIx []uint8, opts ...RequestOption) error {
	return client.RpcClient.SubmitTxWithSponsorIxes(tx, sponsorIx, opts...)
}
func (client *Client) SimulateTx(transaction *lib.Transaction, opts ...RequestOption) (*SimulateTxResult, error) {
	return client.RpcClient.SimulateTx(transaction, opts...)
}
func (client *Client) View(wires []api.PackedInstruction, opts ...RequestOption) (*ViewResult, error) {
	return client.RpcClient.View(wires, opts...)
}
func (client *Client) GetAccount(accountRelaxed any, opts ...RequestOption) (*GetAccountResult, error) {
	return client.RpcClient.GetAccount(accountRelaxed, opts...)
}
func (client *Client) EventsByTxHash(txHashRelaxed any, typeTagFilter *uint64, opts ...RequestOption) (*EventsByTxHashResult, error) {
	return client.RpcClient.EventsByTxHash(txHashRelaxed, typeTagFilter, opts...)
}

func (client *Client) GetBlockByHeight(blockHeight uint64, opts ...RequestOption) (*GetBlockByHeightResult, error) {
	return client.RpcClient.GetBlockByHeight(blockHeight, opts...)
}
func (client *Client) GetTxByHash(txHashOrTxIdRelaxed any, opts ...RequestOption) (*GetTxByHashResult, error) {
	return client.RpcClient.GetTxByHash(txHashOrTxIdRelaxed, opts...)
}
func (client *Client) GetTxHistoryProof(txHashOrTxIdRelaxed any, opts ...RequestOption) (*GetTxHistoryProofResult, error) {
	return client.RpcClient.GetTxHistoryProof(txHashOrTxIdRelaxed, opts...)
}

func (client *Client) GetResource(rsHash api.RsHash, opts ...RequestOption) (*GetResourceResult, error) {
	return client.RpcClient.GetResource(rsHash, opts...)
}
func (client *Client) GetResourcePathByHash(rsHash api.RsHash, opts ...RequestOption) (*GetResourcePathByHashResult, error) {
	return client.RpcClient.GetResourcePathByHash(rsHash, opts...)
}
func (client *Client) BatchGetResourcePathByHash(rsHashList []api.RsHash, opts ...RequestOption) (*BatchGetResourcePathByHashResult, error) {
	return client.RpcClient.BatchGetResourcePathByHash(rsHashList, opts...)
}
func (client *Client) GetAccessValue(blobHashList []api.BlobHash, opts ...RequestOption) (*GetAccessValueResult, error) {
	return client.RpcClient.GetAccessValue(blobHashList, opts...)
}

func (client *Client) WaitForTransaction(txHashOrTxIdRelaxed any, opts ...WaitOption) (*GetTxByHashResult, error) {
	return client.RpcClient.WaitForTransaction(txHashOrTxIdRelaxed, opts...)
}
