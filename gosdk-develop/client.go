package milon

import (
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/provider"
	"time"
)

type RpcClientImpl interface {
	GetPdByIDLAppName(idlAppName string) (*provider.Provider, error)
	GetAllPd() map[string]*provider.Provider
	GetProviderManager() *provider.IDLManager

	ClaimFaucet(claimerSk crypto.SecretKeyer, claimerAddress crypto.Address, mode lib.AccountSignatureMode) error
	BalanceOf(address crypto.Address) (uint64, error)

	GetChainHead(requestId lib.RequestID) (*ChainHeadResult, error)
	SubmitTx(transaction *lib.Transaction, options ...any) error
	SimulateTx(transaction *lib.Transaction, options ...any) (*SimulateTransactionResult, error)
	ViewSingle(transactionPostcard []byte, requestId lib.RequestID) (*ViewSingleTransactionResult, error)
	ViewMulti(transactionPostcard []byte, requestId lib.RequestID) (*ViewMultiTransactionResult, error)
	GetResource(rsHash api.RsHash, requestId lib.RequestID) (*GetResourceResult, error)
	GetBlockByHeight(blockHeight uint64, requestId lib.RequestID) (*GetBlockByHeightResult, error)
	GetTxByHash(txHash any, requestId lib.RequestID) (*GetTxByHashResult, error)
	GetAccount(address string, requestId lib.RequestID) (*GetAccountResult, error)
	EventsByTxHash(txHash any, typeTagFilter *uint64, requestId lib.RequestID) (*EventsByTxHashResult, error)
	ListResourcePath(requestId lib.RequestID) (*ListResourcePathResult, error)
	GetResourcePathByHash(rsHash api.RsHash, requestId lib.RequestID) (*GetResourcePathByHashResult, error)
	GetAccessValue(blobHashList []api.BlobHash, requestId lib.RequestID) (*GetAccessValueResult, error)

	WaitForTransaction(txHash any, requestId lib.RequestID, options ...any) (*GetTxByHashResult, error)

	BuildAndViewSingleIx(idlPath string, methodName string, args provider.Args, requestId lib.RequestID) (*ViewSingleTransactionResult, error)
	BuildAndViewMultiIx(wires []api.PackedInstruction, requestId lib.RequestID) (*ViewMultiTransactionResult, error)
}

type Client struct {
	RpcClient RpcClientImpl
}

// ClientOption configures Client with optional settings.
type ClientOption func(*clientOptions)

type clientOptions struct {
	idlIndexPath string
	pollPeriod   PollPeriod
	pollTimeout  PollTimeout
}

// PollPeriod 定义轮询间隔选项
type PollPeriod time.Duration

// PollTimeout 定义轮询超时选项
type PollTimeout time.Duration

var (
	DefaultPollPeriod  = PollPeriod(1 * time.Second)
	DefaultPollTimeout = PollTimeout(10 * time.Second)
)

// WithIDLPath sets a custom IDL index file path.
// By default, IDL files are embedded in the binary. Use this only for custom/override IDL files.
func WithIDLPath(path string) ClientOption {
	return func(o *clientOptions) {
		o.idlIndexPath = path
	}
}

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

	if opts.idlIndexPath != "" {
		// Custom path via WithIDLPath - load from file system
		if err := rpc.LoadIDLsFromIndex(opts.idlIndexPath); err != nil {
			panic("Failed to load IDLs from" + opts.idlIndexPath + ":" + err.Error())
		}
	} else {
		// Use embedded IDL by default (always available regardless of working directory)
		if err := rpc.LoadEmbeddedIDLs(); err != nil {
			panic("Failed to load embedded IDLs:" + err.Error())
		}
	}

	// Set global TypeTagWithDataResolver
	api.SetGlobalTypeResolver(&provider.IDLTypeResolver{
		Providers: rpc.GetAllPd(),
	})

	idlManager, err := provider.NewIDLManager(rpc.GetAllPd())
	if err != nil {
		panic(err)
	}
	rpc.providerManager = idlManager

	return &Client{
		RpcClient: rpc,
	}
}

func (client *Client) GetPdByIDLAppName(idlAppName string) (*provider.Provider, error) {
	return client.RpcClient.GetPdByIDLAppName(idlAppName)
}

func (client *Client) GetAllPd() map[string]*provider.Provider {
	return client.RpcClient.GetAllPd()
}

func (client *Client) GetProviderManager() *provider.IDLManager {
	return client.RpcClient.GetProviderManager()
}

func (client *Client) ClaimFaucet(claimerSk crypto.SecretKeyer, claimerAddress crypto.Address, mode lib.AccountSignatureMode) error {
	return client.RpcClient.ClaimFaucet(claimerSk, claimerAddress, mode)
}

func (client *Client) BalanceOf(address crypto.Address) (uint64, error) {
	return client.RpcClient.BalanceOf(address)
}

func (client *Client) GetChainHead(requestId lib.RequestID) (*ChainHeadResult, error) {
	return client.RpcClient.GetChainHead(requestId)
}

func (client *Client) SubmitTx(transaction *lib.Transaction, options ...any) error {
	return client.RpcClient.SubmitTx(transaction, options...)
}

func (client *Client) SimulateTx(transaction *lib.Transaction, options ...any) (*SimulateTransactionResult, error) {
	return client.RpcClient.SimulateTx(transaction, options...)
}

func (client *Client) ViewSingle(transactionPostcard []byte, requestId lib.RequestID) (*ViewSingleTransactionResult, error) {
	return client.RpcClient.ViewSingle(transactionPostcard, requestId)
}
func (client *Client) ViewMulti(transactionPostcard []byte, requestId lib.RequestID) (*ViewMultiTransactionResult, error) {
	return client.RpcClient.ViewMulti(transactionPostcard, requestId)
}

func (client *Client) GetResource(rsHash api.RsHash, requestId lib.RequestID) (*GetResourceResult, error) {
	return client.RpcClient.GetResource(rsHash, requestId)
}

func (client *Client) BuildAndViewSingleIx(idlAppName string, methodName string, args provider.Args, requestId lib.RequestID) (*ViewSingleTransactionResult, error) {
	return client.RpcClient.BuildAndViewSingleIx(idlAppName, methodName, args, requestId)
}
func (client *Client) BuildAndViewMultiIx(wires []api.PackedInstruction, requestId lib.RequestID) (*ViewMultiTransactionResult, error) {
	return client.RpcClient.BuildAndViewMultiIx(wires, requestId)
}

func (client *Client) GetTxByHash(txHash any, requestId lib.RequestID) (*GetTxByHashResult, error) {
	return client.RpcClient.GetTxByHash(txHash, requestId)
}
func (client *Client) GetAccount(address string, requestId lib.RequestID) (*GetAccountResult, error) {
	return client.RpcClient.GetAccount(address, requestId)
}
func (client *Client) GetBlockByHeight(blockHeight uint64, requestId lib.RequestID) (*GetBlockByHeightResult, error) {
	return client.RpcClient.GetBlockByHeight(blockHeight, requestId)
}
func (client *Client) EventsByTxHash(txHash any, typeTagFilter *uint64, requestId lib.RequestID) (*EventsByTxHashResult, error) {
	return client.RpcClient.EventsByTxHash(txHash, typeTagFilter, requestId)
}

func (client *Client) ListResourcePath(requestId lib.RequestID) (*ListResourcePathResult, error) {
	return client.RpcClient.ListResourcePath(requestId)
}

func (client *Client) GetResourcePathByHash(rsHash api.RsHash, requestId lib.RequestID) (*GetResourcePathByHashResult, error) {
	return client.RpcClient.GetResourcePathByHash(rsHash, requestId)
}
func (client *Client) WaitForTransaction(txHash any, requestId lib.RequestID, options ...any) (*GetTxByHashResult, error) {
	return client.RpcClient.WaitForTransaction(txHash, requestId, options...)
}
func (client *Client) GetAccessValue(blobHashList []api.BlobHash, requestId lib.RequestID) (*GetAccessValueResult, error) {
	return client.RpcClient.GetAccessValue(blobHashList, requestId)
}
