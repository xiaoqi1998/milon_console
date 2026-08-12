package milon

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/gen"
	"github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/provider"
	"github.com/milon-labs/milon-go-sdk/types"
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

	GetChainHead(options ...any) (*ChainHeadResult, error)
	SubmitTx(transaction *lib.Transaction, options ...any) error
	SimulateTx(transaction *lib.Transaction, options ...any) (*SimulateTxResult, error)
	View(wires []api.PackedInstruction, options ...any) (*ViewResult, error)
	GetResource(rsHash api.RsHash, options ...any) (*GetResourceResult, error)
	GetBlockByHeight(blockHeight uint64, options ...any) (*GetBlockByHeightResult, error)
	GetTxByHash(txHash any, options ...any) (*GetTxByHashResult, error)
	GetAccount(accountRelaxed any, options ...any) (*GetAccountResult, error)
	EventsByTxHash(txHash any, typeTagFilter *uint64, options ...any) (*EventsByTxHashResult, error)
	ListResourcePath(options ...any) (*ListResourcePathResult, error)
	GetResourcePathByHash(rsHash api.RsHash, options ...any) (*GetResourcePathByHashResult, error)
	GetAccessValue(blobHashList []api.BlobHash, options ...any) (*GetAccessValueResult, error)
	BatchGetResourcePathByHash(rsHashList []api.RsHash, options ...any) (*BatchGetResourcePathByHashResult, error)

	WaitForTransaction(txHash any, options ...any) (*GetTxByHashResult, error)
}

type Client struct {
	RpcClient RpcClientImpl
}

// ClientOption configures Client with optional settings.
type ClientOption func(*clientOptions)

type clientOptions struct {
	pollPeriod  PollPeriod
	pollTimeout PollTimeout
}

// PollPeriod defines the polling interval option
type PollPeriod time.Duration

// PollTimeout defines the polling timeout option
type PollTimeout time.Duration

var (
	DefaultPollPeriod  = PollPeriod(1 * time.Second)
	DefaultPollTimeout = PollTimeout(10 * time.Second)
)

// WithPollPeriod sets the polling interval for WaitForTransaction.
func WithPollPeriod(period PollPeriod) ClientOption {
	return func(o *clientOptions) {
		o.pollPeriod = period
	}
}

// WithPollTimeout sets the polling timeout for WaitForTransaction.
func WithPollTimeout(timeout PollTimeout) ClientOption {
	return func(o *clientOptions) {
		o.pollTimeout = timeout
	}
}

func NewClient(config Network, options ...ClientOption) *Client {
	lib.SetChainId(config.ChainId)

	opts := &clientOptions{
		pollPeriod:  DefaultPollPeriod,
		pollTimeout: DefaultPollTimeout,
	}
	for _, opt := range options {
		opt(opts)
	}

	rpc := &rpcClientV1{
		network:           config,
		providerByIDLName: make(map[string]*provider.Provider),
		pollPeriod:        time.Duration(opts.pollPeriod),
		pollTimeout:       time.Duration(opts.pollTimeout),
	}

	// Use the IDLs generated inline by tools/idlgen (gen.DefaultIDLs),
	if err := rpc.LoadIDLsFromData(gen.DefaultIDLs); err != nil {
		panic("failed to load generated IDLs:" + err.Error())
	}

	// Set global TypeTagWithDataResolver
	api.SetGlobalTypeResolver(&provider.IDLTypeResolver{
		Providers: rpc.GetAllPd(),
	})

	idlManager, err := provider.NewIDLRegistry(rpc.GetAllPd())
	if err != nil {
		panic(err)
	}
	rpc.providerManager = idlManager

	// Rebind generated IDL app objects (token.ClaimFaucet, ...) to the loaded
	// providers, so every NewClient uses the latest IDL definitions.
	if err = gen.BindAll(rpc.GetAllPd()); err != nil {
		panic(fmt.Sprintf("failed to bind generated IDL apps: %v", err))
	}

	return &Client{
		RpcClient: rpc,
	}
}

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
func (client *Client) GetChainHead(options ...any) (*ChainHeadResult, error) {
	return client.RpcClient.GetChainHead(options...)
}

func (client *Client) SubmitTx(transaction *lib.Transaction, options ...any) error {
	return client.RpcClient.SubmitTx(transaction, options...)
}

func (client *Client) SimulateTx(transaction *lib.Transaction, options ...any) (*SimulateTxResult, error) {
	return client.RpcClient.SimulateTx(transaction, options...)
}

func (client *Client) View(wires []api.PackedInstruction, options ...any) (*ViewResult, error) {
	return client.RpcClient.View(wires, options...)
}

func (client *Client) GetResource(rsHash api.RsHash, options ...any) (*GetResourceResult, error) {
	return client.RpcClient.GetResource(rsHash, options...)
}

func (client *Client) GetBlockByHeight(blockHeight uint64, options ...any) (*GetBlockByHeightResult, error) {
	return client.RpcClient.GetBlockByHeight(blockHeight, options...)
}

func (client *Client) GetTxByHash(txHash any, options ...any) (*GetTxByHashResult, error) {
	return client.RpcClient.GetTxByHash(txHash, options...)
}

func (client *Client) GetAccount(accountRelaxed any, options ...any) (*GetAccountResult, error) {
	return client.RpcClient.GetAccount(accountRelaxed, options...)
}

func (client *Client) EventsByTxHash(txHashRelaxed any, typeTagFilter *uint64, options ...any) (*EventsByTxHashResult, error) {
	return client.RpcClient.EventsByTxHash(txHashRelaxed, typeTagFilter, options...)
}

func (client *Client) ListResourcePath(options ...any) (*ListResourcePathResult, error) {
	return client.RpcClient.ListResourcePath(options...)
}

func (client *Client) GetResourcePathByHash(rsHash api.RsHash, options ...any) (*GetResourcePathByHashResult, error) {
	return client.RpcClient.GetResourcePathByHash(rsHash, options...)
}

func (client *Client) GetAccessValue(blobHashList []api.BlobHash, options ...any) (*GetAccessValueResult, error) {
	return client.RpcClient.GetAccessValue(blobHashList, options...)
}

func (client *Client) BatchGetResourcePathByHash(rsHashList []api.RsHash, options ...any) (*BatchGetResourcePathByHashResult, error) {
	return client.RpcClient.BatchGetResourcePathByHash(rsHashList, options...)
}

func (client *Client) WaitForTransaction(txHash any, options ...any) (*GetTxByHashResult, error) {
	return client.RpcClient.WaitForTransaction(txHash, options...)
}
