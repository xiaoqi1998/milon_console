package milon

import (
	"fmt"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/provider"
)

type MolinClient struct {
	RpcClient RpcClientImpl
}

type RpcClientImpl interface {
	GetPdByIDLAppName(idlAppName string) (*provider.Provider, error)
	GetAllPd() map[string]*provider.Provider
	GetProviderManager() *provider.IDLManager
	ClaimFaucet(claimerSk crypto.SecretKeyer, claimerAddress crypto.Address, mode AccountSignatureMode) error
	AddressBalance(address crypto.Address) (uint64, error)

	CreateTransactionWithParam(instructions []api.PackedInstruction, payer *crypto.Address, options ...any) (*Transaction, error)
	SignPayerAndAddSignature(transaction *Transaction, payerSk crypto.SecretKeyer, payerAddress crypto.Address, mode AccountSignatureMode) error
	SimulateSignPayerAndAddSignature(transaction *Transaction, payerAddress crypto.Address, mode AccountSignatureMode) error
	SignIxAndAddSignature(transaction *Transaction, ixIndex uint8, ixSk crypto.SecretKeyer, ixAddress crypto.Address, mode AccountSignatureMode) error
	SimulateSignIxAndAddSignature(transaction *Transaction, ixIndex uint8, ixAddress crypto.Address, mode AccountSignatureMode) error

	GetChainHead(requestId uint64) (*ChainHeadResult, error)

	SimulateTx(transactionPostcard []byte, requestId uint64) (*SimulateTransactionResult, error)
	SubmitTx(transactionPostcard []byte, requestId uint64) (*SubmitTransactionResult, error)

	ViewSingle(transactionPostcard []byte, requestId uint64) (*ViewSingleTransactionResult, error)
	ViewMulti(transactionPostcard []byte, requestId uint64) (*ViewMultiTransactionResult, error)

	GetResource(rsHash api.RsHash, requestId uint64) (*GetResourceResult, error)

	//* ********************************* simulate transaction  **********************************

	BuildAndSimulateSingleIxUnifiedPayerAll(idlAppName string, methodName string, args provider.Args, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error)
	BuildAndSimulateSingleIxUnifiedDualSign(idlAppName string, methodName string, args provider.Args, payerAddress crypto.Address, payerAcSigMod AccountSignatureMode, ixAddress crypto.Address, ixAcSigMod AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error)
	BuildAndSimulateSingleIxUnifiedPayerOnlyGas(idlAppName string, methodName string, args provider.Args, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error)

	BuildAndSimulateSingleIxSplit(idlAppName string, methodName string, args provider.Args, ownerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error)

	BuildAndSimulateMultiIxUnified(wires []api.PackedInstruction, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error)
	BuildAndSimulateMultiIxSplit(wires []api.PackedInstruction, ownerAddressList []crypto.Address, acSigModList []AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error)

	// ********************************** simulate transaction  **********************************

	// ********************************** signature transaction  **********************************

	BuildAndSubmitSingleIxUnifiedPayerSignAll(idlAppName string, methodName string, args provider.Args, payerSk crypto.SecretKeyer, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error)
	BuildAndSubmitSingleIxUnifiedDualSign(idlAppName string, methodName string, args provider.Args, payerSk crypto.SecretKeyer, payerAddress crypto.Address, payerAcSigMod AccountSignatureMode, ixSk crypto.SecretKeyer, ixAddress crypto.Address, ixAcSigMod AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error)
	BuildAndSubmitSingleIxUnifiedPayerOnlyGas(idlAppName string, methodName string, args provider.Args, payerSk crypto.SecretKeyer, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error)

	BuildAndSubmitSingleIxSplit(idlAppName string, methodName string, args provider.Args, ownerSk crypto.SecretKeyer, ownerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error)

	BuildAndSubmitMultiIxUnified(wires []api.PackedInstruction, payerSk crypto.SecretKeyer, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error)
	BuildAndSubmitMultiIxSplit(wires []api.PackedInstruction, ownerSkList []crypto.SecretKeyer, ownerAddressList []crypto.Address, acSigModList []AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error)

	// ********************************** signature transaction  **********************************

	BuildAndViewSingleIx(idlAppName string, methodName string, args provider.Args, requestId uint64) (*ViewSingleTransactionResult, error)
	BuildAndViewMultiIx(wires []api.PackedInstruction, requestId uint64) (*ViewMultiTransactionResult, error)

	GetTxByHash(txHash string, requestId uint64) (*GetTxByHashResult, error)
	GetAccount(address string, requestId uint64) (*GetAccountResult, error)
	GetBlock(blockHeight uint64, requestId uint64) (*GetBlockByHeightResult, error)
	EventsByTxHash(txHashRelaxed string, typeTagFilter *uint64, requestId uint64) (*EventsByTxHashResult, error)
	ListResourcePath(requestId uint64) (*ListResourcePathResult, error)
	GetResourcePathByHash(rsHash api.RsHash, requestId uint64) (*GetResourcePathByHashResult, error)
	GetAccessValue(blobHashList []api.BlobHash, requestId uint64) (*GetAccessValueResult, error)
	WaitForTransaction(txHashRelaxed string, requestId uint64, options ...any) (*GetTxByHashResult, error)
}

// ClientOption configures MolinClient with optional settings.
type ClientOption func(*clientOptions)

type clientOptions struct {
	idlIndexPath string
}

// WithIDLPath sets a custom IDL index file path.
// By default, IDL files are embedded in the binary. Use this only for custom/override IDL files.
func WithIDLPath(path string) ClientOption {
	return func(o *clientOptions) {
		o.idlIndexPath = path
	}
}

// NewMilonClient 创建 Milon 客户端。
// 注意：当 IDL 加载失败时会 panic，调用方应确保配置正确。
// 如需错误处理，请使用 NewMolinClientWithErr。
func NewMilonClient(config NetworkConfig, options ...ClientOption) *MolinClient {
	client, err := NewMolinClientWithErr(config, options...)
	if err != nil {
		panic(err)
	}
	return client
}

// NewMolinClientWithErr 创建 Milon 客户端，并在 IDL 加载失败时返回 error。
func NewMolinClientWithErr(config NetworkConfig, options ...ClientOption) (*MolinClient, error) {
	SetChainId(config.ChainId)

	opts := &clientOptions{}
	for _, opt := range options {
		opt(opts)
	}

	rpc := &rpcClientV1{
		network:           config,
		providerByIDLName: make(map[string]*provider.Provider),
		//providerManager:        provider.NewIDLManager(rpc.providerByIDLName),
	}

	if opts.idlIndexPath != "" {
		// Custom path via WithIDLPath - load from file system
		if err := rpc.LoadIDLsFromIndex(opts.idlIndexPath); err != nil {
			return nil, fmt.Errorf("failed to load IDLs from %s: %w", opts.idlIndexPath, err)
		}
	} else {
		// Use embedded IDL by default (always available regardless of working directory)
		if err := rpc.LoadEmbeddedIDLs(); err != nil {
			return nil, fmt.Errorf("failed to load embedded IDLs: %w", err)
		}
	}

	// Set global TypeTagWithDataResolver
	api.SetGlobalTypeResolver(&provider.IDLTypeResolver{
		Providers: rpc.GetAllPd(),
	})

	idlManager, err := provider.NewIDLManager(rpc.GetAllPd())
	if err != nil {
		return nil, err
	}
	rpc.providerManager = idlManager

	return &MolinClient{
		RpcClient: rpc,
	}, nil
}

func (client *MolinClient) GetPdByIDLAppName(idlAppName string) (*provider.Provider, error) {
	return client.RpcClient.GetPdByIDLAppName(idlAppName)
}

func (client *MolinClient) GetAllPd() map[string]*provider.Provider {
	return client.RpcClient.GetAllPd()
}

func (client *MolinClient) GetProviderManager() *provider.IDLManager {
	return client.RpcClient.GetProviderManager()
}

func (client *MolinClient) ClaimFaucet(claimerSk crypto.SecretKeyer, claimerAddress crypto.Address, mode AccountSignatureMode) error {
	return client.RpcClient.ClaimFaucet(claimerSk, claimerAddress, mode)
}

func (client *MolinClient) AddressBalance(address crypto.Address) (uint64, error) {
	return client.RpcClient.AddressBalance(address)
}

func (client *MolinClient) CreateTransactionWithParam(instructions []api.PackedInstruction, payer *crypto.Address, options ...any) (*Transaction, error) {
	return client.RpcClient.CreateTransactionWithParam(instructions, payer, options...)
}

func (client *MolinClient) SignPayerAndAddSignature(transaction *Transaction, payerSk crypto.SecretKeyer, payerAddress crypto.Address, mode AccountSignatureMode) error {
	return client.RpcClient.SignPayerAndAddSignature(transaction, payerSk, payerAddress, mode)
}
func (client *MolinClient) SimulateSignPayerAndAddSignature(transaction *Transaction, payerAddress crypto.Address, mode AccountSignatureMode) error {
	return client.RpcClient.SimulateSignPayerAndAddSignature(transaction, payerAddress, mode)
}
func (client *MolinClient) SignIxAndAddSignature(transaction *Transaction, ixIndex uint8, ixSk crypto.SecretKeyer, ixAddress crypto.Address, mode AccountSignatureMode) error {
	return client.RpcClient.SignIxAndAddSignature(transaction, ixIndex, ixSk, ixAddress, mode)
}
func (client *MolinClient) SimulateSignIxAndAddSignature(transaction *Transaction, ixIndex uint8, ixAddress crypto.Address, mode AccountSignatureMode) error {
	return client.RpcClient.SimulateSignIxAndAddSignature(transaction, ixIndex, ixAddress, mode)
}

func (client *MolinClient) GetChainHead(requestId uint64) (*ChainHeadResult, error) {
	return client.RpcClient.GetChainHead(requestId)
}

func (client *MolinClient) SubmitTx(transactionPostcard []byte, requestId uint64) (*SubmitTransactionResult, error) {
	return client.RpcClient.SubmitTx(transactionPostcard, requestId)
}

func (client *MolinClient) SimulateTx(transactionPostcard []byte, requestId uint64) (*SimulateTransactionResult, error) {
	return client.RpcClient.SimulateTx(transactionPostcard, requestId)
}

func (client *MolinClient) ViewSingle(transactionPostcard []byte, requestId uint64) (*ViewSingleTransactionResult, error) {
	return client.RpcClient.ViewSingle(transactionPostcard, requestId)
}

func (client *MolinClient) ViewMulti(transactionPostcard []byte, requestId uint64) (*ViewMultiTransactionResult, error) {
	return client.RpcClient.ViewMulti(transactionPostcard, requestId)
}

func (client *MolinClient) GetResource(rsHash api.RsHash, requestId uint64) (*GetResourceResult, error) {
	return client.RpcClient.GetResource(rsHash, requestId)
}

func (client *MolinClient) BuildAndSimulateSingleIxUnifiedPayerAll(idlAppName string, methodName string, args provider.Args, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error) {
	return client.RpcClient.BuildAndSimulateSingleIxUnifiedPayerAll(idlAppName, methodName, args, payerAddress, acSigMod, requestId, options...)
}
func (client *MolinClient) BuildAndSimulateSingleIxUnifiedDualSign(idlAppName string, methodName string, args provider.Args, payerAddress crypto.Address, payerAcSigMod AccountSignatureMode, ixAddress crypto.Address, ixAcSigMod AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error) {
	return client.RpcClient.BuildAndSimulateSingleIxUnifiedDualSign(idlAppName, methodName, args, payerAddress, payerAcSigMod, ixAddress, ixAcSigMod, requestId, options...)
}
func (client *MolinClient) BuildAndSimulateSingleIxUnifiedPayerOnlyGas(idlAppName string, methodName string, args provider.Args, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error) {
	return client.RpcClient.BuildAndSimulateSingleIxUnifiedPayerOnlyGas(idlAppName, methodName, args, payerAddress, acSigMod, requestId, options...)
}

func (client *MolinClient) BuildAndSimulateSingleIxSplit(idlAppName string, methodName string, args provider.Args, ownerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error) {
	return client.RpcClient.BuildAndSimulateSingleIxSplit(idlAppName, methodName, args, ownerAddress, acSigMod, requestId, options...)
}

// Deprecated: Use BuildAndSimulateSingleIxSplit instead. Kept for backward compatibility.
func (client *MolinClient) BuildAnSimulateSingleIxSplit(idlAppName string, methodName string, args provider.Args, ownerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error) {
	return client.BuildAndSimulateSingleIxSplit(idlAppName, methodName, args, ownerAddress, acSigMod, requestId, options...)
}

func (client *MolinClient) BuildAndSimulateMultiIxUnified(wires []api.PackedInstruction, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error) {
	return client.RpcClient.BuildAndSimulateMultiIxUnified(wires, payerAddress, acSigMod, requestId, options...)
}
func (client *MolinClient) BuildAndSimulateMultiIxSplit(wires []api.PackedInstruction, ownerAddressList []crypto.Address, acSigModList []AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error) {
	return client.RpcClient.BuildAndSimulateMultiIxSplit(wires, ownerAddressList, acSigModList, requestId, options...)
}

func (client *MolinClient) BuildAndSubmitSingleIxUnifiedPayerSignAll(idlAppName string, methodName string, args provider.Args, payerSk crypto.SecretKeyer, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error) {
	return client.RpcClient.BuildAndSubmitSingleIxUnifiedPayerSignAll(idlAppName, methodName, args, payerSk, payerAddress, acSigMod, requestId, options...)
}
func (client *MolinClient) BuildAndSubmitSingleIxUnifiedDualSign(idlAppName string, methodName string, args provider.Args, payerSk crypto.SecretKeyer, payerAddress crypto.Address, payerAcSigMod AccountSignatureMode, ixSk crypto.SecretKeyer, ixAddress crypto.Address, ixAcSigMod AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error) {
	return client.RpcClient.BuildAndSubmitSingleIxUnifiedDualSign(idlAppName, methodName, args, payerSk, payerAddress, payerAcSigMod, ixSk, ixAddress, ixAcSigMod, requestId, options...)
}
func (client *MolinClient) BuildAndSubmitSingleIxUnifiedPayerOnlyGas(idlAppName string, methodName string, args provider.Args, payerSk crypto.SecretKeyer, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error) {
	return client.RpcClient.BuildAndSubmitSingleIxUnifiedPayerOnlyGas(idlAppName, methodName, args, payerSk, payerAddress, acSigMod, requestId, options...)
}

func (client *MolinClient) BuildAndSubmitSingleIxSplit(idlAppName string, methodName string, args provider.Args, ownerSk crypto.SecretKeyer, ownerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error) {
	return client.RpcClient.BuildAndSubmitSingleIxSplit(idlAppName, methodName, args, ownerSk, ownerAddress, acSigMod, requestId, options...)
}

func (client *MolinClient) BuildAndSubmitMultiIxUnified(wires []api.PackedInstruction, payerSk crypto.SecretKeyer, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error) {
	return client.RpcClient.BuildAndSubmitMultiIxUnified(wires, payerSk, payerAddress, acSigMod, requestId, options...)
}
func (client *MolinClient) BuildAndSubmitMultiIxSplit(wires []api.PackedInstruction, ownerSkList []crypto.SecretKeyer, ownerAddressList []crypto.Address, acSigModList []AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error) {
	return client.RpcClient.BuildAndSubmitMultiIxSplit(wires, ownerSkList, ownerAddressList, acSigModList, requestId, options...)
}

func (client *MolinClient) BuildAndViewSingleIx(idlAppName string, methodName string, args provider.Args, requestId uint64) (*ViewSingleTransactionResult, error) {
	return client.RpcClient.BuildAndViewSingleIx(idlAppName, methodName, args, requestId)
}
func (client *MolinClient) BuildAndViewMultiIx(wires []api.PackedInstruction, requestId uint64) (*ViewMultiTransactionResult, error) {
	return client.RpcClient.BuildAndViewMultiIx(wires, requestId)
}

func (client *MolinClient) GetTxByHash(txHash string, requestId uint64) (*GetTxByHashResult, error) {
	return client.RpcClient.GetTxByHash(txHash, requestId)
}
func (client *MolinClient) GetAccount(address string, requestId uint64) (*GetAccountResult, error) {
	return client.RpcClient.GetAccount(address, requestId)
}
func (client *MolinClient) GetBlock(blockHeight uint64, requestId uint64) (*GetBlockByHeightResult, error) {
	return client.RpcClient.GetBlock(blockHeight, requestId)
}
func (client *MolinClient) EventsByTxHash(txHashRelaxed string, typeTagFilter *uint64, requestId uint64) (*EventsByTxHashResult, error) {
	return client.RpcClient.EventsByTxHash(txHashRelaxed, typeTagFilter, requestId)
}

func (client *MolinClient) ListResourcePath(requestId uint64) (*ListResourcePathResult, error) {
	return client.RpcClient.ListResourcePath(requestId)
}

func (client *MolinClient) GetResourcePathByHash(rsHash api.RsHash, requestId uint64) (*GetResourcePathByHashResult, error) {
	return client.RpcClient.GetResourcePathByHash(rsHash, requestId)
}
func (client *MolinClient) WaitForTransaction(txHashRelaxed string, requestId uint64, options ...any) (*GetTxByHashResult, error) {
	return client.RpcClient.WaitForTransaction(txHashRelaxed, requestId, options...)
}
func (client *MolinClient) GetAccessValue(blobHashList []api.BlobHash, requestId uint64) (*GetAccessValueResult, error) {
	return client.RpcClient.GetAccessValue(blobHashList, requestId)
}
