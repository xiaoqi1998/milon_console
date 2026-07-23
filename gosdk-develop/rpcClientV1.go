package milon

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/btcsuite/btcutil/base58"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/milon-labs/milon-go-sdk/provider"
	"github.com/milon-labs/milon-go-sdk/tools"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// PollPeriod 定义轮询间隔选项
type PollPeriod time.Duration

// PollTimeout 定义轮询超时选项
type PollTimeout time.Duration

type ChainHeadResult struct {
	HttpStatusCode int
	HttpRspBytes   []byte
	HttpRspBody    []byte
	BodyChainHead  *api.ChainHead
}

type SubmitTransactionResult struct {
	HttpStatusCode int
	HttpRspBytes   []byte
	HttpRspBody    []byte
	BodyTxHash     string
}

type SimulateTransactionResult struct {
	HttpStatusCode      int
	HttpRspBytes        []byte
	HttpRspBody         []byte
	BodySimulateReceipt *api.SimulateReceipt
}

type ViewSingleTransactionResult struct {
	HttpStatusCode int
	HttpRspBytes   []byte
	HttpRspBody    []byte
	BodyValues     any
}

type ViewMultiTransactionResult struct {
	HttpStatusCode int
	HttpRspBytes   []byte
	HttpRspBody    []byte
}

type GetResourceResult struct {
	HttpStatusCode  int
	HttpRspBytes    []byte
	HttpRspBody     []byte
	BodyGetResource *api.GetResource
}

type GetBlockByHeightResult struct {
	HttpStatusCode int
	HttpRspBytes   []byte
	HttpRspBody    []byte
	BodyBlock      *api.Block
}

type GetTxByHashResult struct {
	HttpStatusCode int
	HttpRspBytes   []byte
	HttpRspBody    []byte
	BodyTxHistory  *api.TxHistory
}

type GetAccountResult struct {
	HttpStatusCode  int
	HttpRspBytes    []byte
	HttpRspBody     []byte
	BodyAccountView *api.AccountView
}

type EventsByTxHashResult struct {
	HttpStatusCode     int
	HttpRspBytes       []byte
	HttpRspBody        []byte
	BodyEventsByTxHash *api.EventsByTxHash
}

type ListResourcePathResult struct {
	HttpStatusCode        int
	HttpRspBytes          []byte
	HttpRspBody           []byte
	BodyListResourcePaths []*api.ListResourcePathInfo
}

type GetResourcePathByHashResult struct {
	HttpStatusCode int
	HttpRspBytes   []byte
	HttpRspBody    []byte
}

type GetAccessValueResult struct {
	HttpStatusCode      int
	HttpRspBytes        []byte
	HttpRspBody         []byte
	BodyGetAccessValues []*api.GetAccessValueInfo
}

type rpcClientV1 struct {
	network           NetworkConfig
	providerByIDLName map[string]*provider.Provider
	providerManager   *provider.IDLManager
}

//go:embed provider/IDL
var idlFS embed.FS

// LoadEmbeddedIDLs loads IDL definitions from embedded files.
// This ensures IDL files are always available regardless of the working directory.
func (c *rpcClientV1) LoadEmbeddedIDLs() error {
	data, err := idlFS.ReadFile("provider/IDL/index.json")
	if err != nil {
		return fmt.Errorf("failed to read embedded index file: %w", err)
	}

	var indexConfig struct {
		Apps []struct {
			AppID uint8  `json:"app_id"`
			IDL   string `json:"idl"`
			Name  string `json:"name"`
		} `json:"apps"`
	}

	if err = json.Unmarshal(data, &indexConfig); err != nil {
		return fmt.Errorf("failed to unmarshal embedded index: %w", err)
	}

	for _, app := range indexConfig.Apps {
		// Clean path to handle "./" prefixes from index.json
		idlPath := path.Clean("provider/IDL/" + app.IDL)
		idlData, err := idlFS.ReadFile(idlPath)
		if err != nil {
			return fmt.Errorf("failed to read embedded IDL file %s: %w", idlPath, err)
		}

		var idl provider.IDL
		if err = json.Unmarshal(idlData, &idl); err != nil {
			return fmt.Errorf("failed to unmarshal embedded IDL %s: %w", idlPath, err)
		}

		c.providerByIDLName[app.Name] = provider.NewProvider(idl)
	}

	return nil
}

func (c *rpcClientV1) LoadIDLsFromIndex(indexFilePath string) error {
	data, err := os.ReadFile(indexFilePath)
	if err != nil {
		return fmt.Errorf("failed to read index file: %w", err)
	}

	var indexConfig struct {
		Apps []struct {
			AppID uint8  `json:"app_id"`
			IDL   string `json:"idl"`
			Name  string `json:"name"`
		} `json:"apps"`
	}

	if err = json.Unmarshal(data, &indexConfig); err != nil {
		return fmt.Errorf("failed to unmarshal index file: %w", err)
	}

	baseDir := filepath.Dir(indexFilePath)
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return fmt.Errorf("failed to resolve base directory: %w", err)
	}

	for _, app := range indexConfig.Apps {
		cleanIDL := filepath.Clean(app.IDL)
		if filepath.IsAbs(cleanIDL) || strings.HasPrefix(cleanIDL, ".."+string(filepath.Separator)) || cleanIDL == ".." {
			return fmt.Errorf("IDL path escapes base directory: %s", app.IDL)
		}
		idlPath := filepath.Join(baseDir, cleanIDL)
		// 校验最终路径仍在 baseDir 下
		absIDL, err := filepath.Abs(idlPath)
		if err != nil {
			return fmt.Errorf("failed to resolve IDL path: %w", err)
		}
		if !strings.HasPrefix(absIDL, absBase+string(filepath.Separator)) && absIDL != absBase {
			return fmt.Errorf("IDL path escapes base directory: %s", app.IDL)
		}
		idlData, err := os.ReadFile(idlPath)
		if err != nil {
			return fmt.Errorf("failed to read IDL file %s: %w", idlPath, err)
		}

		var idl provider.IDL
		if err = json.Unmarshal(idlData, &idl); err != nil {
			return fmt.Errorf("failed to unmarshal IDL %s: %w", idlPath, err)
		}

		c.providerByIDLName[app.Name] = provider.NewProvider(idl)
	}

	return nil
}

func (c *rpcClientV1) GetPdByIDLAppName(idlAppName string) (*provider.Provider, error) {
	// 1. 加载 IDL
	pd, ok := c.providerByIDLName[idlAppName]
	if !ok {
		return nil, fmt.Errorf("IDL for app %s not found", idlAppName)
	}

	return pd, nil
}

func (c *rpcClientV1) GetAllPd() map[string]*provider.Provider {
	return c.providerByIDLName
}

func (c *rpcClientV1) GetProviderManager() *provider.IDLManager {
	return c.providerManager
}

func (c *rpcClientV1) ClaimFaucet(claimerSk crypto.SecretKeyer, claimerAddress crypto.Address, mode AccountSignatureMode) error {
	submitTransactionResult, err := c.BuildAndSubmitSingleIxSplit(
		"token",
		"ClaimFaucet",
		provider.Args{
			"claimer": claimerAddress,
		},
		claimerSk,
		claimerAddress,
		mode,
		1,
	)
	if err != nil {
		return fmt.Errorf("failed to claim faucet: %w", err)
	}

	_, err = c.WaitForTransaction(submitTransactionResult.BodyTxHash, 1)
	if err != nil {
		return fmt.Errorf("failed to wait for transaction: %w", err)
	}

	return nil
}

func (c *rpcClientV1) AddressBalance(address crypto.Address) (uint64, error) {
	viewSingleTransactionResult, err := c.BuildAndViewSingleIx(
		"token",
		"BalanceOf",
		provider.Args{
			"token":   api.MIL,
			"account": address,
		},
		1,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to view single transaction: %w", err)
	}

	balance, ok := viewSingleTransactionResult.BodyValues.(uint64)
	if !ok {
		return 0, fmt.Errorf("unexpected balance type: %T", viewSingleTransactionResult.BodyValues)
	}
	return balance, nil
}

func (c *rpcClientV1) CreateTransactionWithParam(instructions []api.PackedInstruction, payer *crypto.Address, options ...any) (*Transaction, error) {
	if len(instructions) == 0 {
		return nil, fmt.Errorf("instructions cannot be empty")
	}

	return NewTransactionWithParam(instructions, payer, options...)
}

func (c *rpcClientV1) SignPayerAndAddSignature(transaction *Transaction, payerSk crypto.SecretKeyer, payerAddress crypto.Address, mode AccountSignatureMode) error {
	sig, err := transaction.SignPayer(payerAddress, payerSk, mode)
	if err != nil {
		return fmt.Errorf("failed to sign payer: %w", err)
	}

	transaction.AddSignature(payerAddress, *sig)
	return nil
}

func (c *rpcClientV1) SimulateSignPayerAndAddSignature(transaction *Transaction, payerAddress crypto.Address, mode AccountSignatureMode) error {
	sig, err := transaction.SimulateSignPayer(payerAddress, mode)
	if err != nil {
		return fmt.Errorf("failed to sign payer: %w", err)
	}

	transaction.AddSignature(payerAddress, *sig)
	return nil
}

func (c *rpcClientV1) SignIxAndAddSignature(transaction *Transaction, ixIndex uint8, ixSk crypto.SecretKeyer, ixAddress crypto.Address, mode AccountSignatureMode) error {
	sig, err := transaction.SignIx(ixAddress, ixSk, ixIndex, mode)
	if err != nil {
		return fmt.Errorf("failed to sign ix at index %d: %w", ixIndex, err)
	}

	transaction.AddSignature(ixAddress, *sig)
	return nil
}

func (c *rpcClientV1) SimulateSignIxAndAddSignature(transaction *Transaction, ixIndex uint8, ixAddress crypto.Address, mode AccountSignatureMode) error {
	sig, err := transaction.SimulateSignIx(ixAddress, ixIndex, mode)
	if err != nil {
		return fmt.Errorf("failed to sign ix at index %d: %w", ixIndex, err)
	}

	transaction.AddSignature(ixAddress, *sig)
	return nil
}

func (c *rpcClientV1) SimulateTx(transactionPostcard []byte, requestId uint64) (*SimulateTransactionResult, error) {
	// 1. 创建 RPC 请求对象（MethodTypeSimulateTx，包含已序列化的交易数据）
	submitTransaction := NewSubmitTransaction(MethodTypeSimulateTx, requestId, transactionPostcard)

	// 2. 将请求序列化为 postcard 格式并发送 HTTP POST
	submitTransactionPostcard, err := postcard.SerializePostcard(submitTransaction)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize submit transaction: %w", err)
	}
	httpStatusCode, httpResponseBytes, err := tools.HttpPostByBytes(
		context.Background(),
		c.network.RpcUrl,
		submitTransactionPostcard,
		map[string]string{
			"Content-Type": ContentTypeMilonPostcard,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
	}

	// 3. 反序列化 postcard 格式的 HTTP 响应为 API RpcResponse 结构
	httpResponse, err := postcard.DeserializePostcard(httpResponseBytes, func(d *postcard.Deserializer) (*api.RpcResponse, error) {
		var rsp api.RpcResponse
		if err = rsp.UnmarshalPostcard(d); err != nil {
			return nil, err
		}
		return &rsp, nil
	}, false)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize API response: %w", err)
	}

	// 4. 验证 API 响应状态码（必须为 RpcResponseStatusOk）
	if httpResponse.Status != api.RpcResponseStatusOk {
		return nil, fmt.Errorf("API returned error status: %+v", httpResponse.Error)
	}

	// 5. 反序列化 postcard 格式的响应体为 SimulateReceipt 结构
	simulateReceipt, err := postcard.DeserializePostcard(httpResponse.Body, func(d *postcard.Deserializer) (*api.SimulateReceipt, error) {
		var rsp api.SimulateReceipt
		if err = rsp.UnmarshalPostcard(d); err != nil {
			return nil, err
		}
		return &rsp, nil
	}, false)
	if err != nil {
		return nil, fmt.Errorf("反序列化 SimulateReceipt 失败: %w", err)
	}

	// 6. 返回提交结果
	return &SimulateTransactionResult{
		HttpStatusCode:      httpStatusCode,
		HttpRspBytes:        httpResponseBytes,
		HttpRspBody:         httpResponse.Body,
		BodySimulateReceipt: simulateReceipt,
	}, nil
}

func (c *rpcClientV1) SubmitTx(transactionPostcard []byte, requestId uint64) (*SubmitTransactionResult, error) {
	// 1. 创建 RPC 请求对象（MethodTypeSubmitTx，包含已序列化的交易数据）
	submitTransaction := NewSubmitTransaction(MethodTypeSubmitTx, requestId, transactionPostcard)

	// 2. 将请求序列化为 postcard 格式并发送 HTTP POST
	submitTransactionPostcard, err := postcard.SerializePostcard(submitTransaction)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize submit transaction: %w", err)
	}
	httpStatusCode, httpResponseBytes, err := tools.HttpPostByBytes(
		context.Background(),
		c.network.RpcUrl,
		submitTransactionPostcard,
		map[string]string{
			"Content-Type": ContentTypeMilonPostcard,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
	}

	// 3. 反序列化 postcard 格式的 HTTP 响应为 API RpcResponse 结构
	httpResponse, err := postcard.DeserializePostcard(httpResponseBytes, func(d *postcard.Deserializer) (*api.RpcResponse, error) {
		var rsp api.RpcResponse
		if err = rsp.UnmarshalPostcard(d); err != nil {
			return nil, err
		}
		return &rsp, nil
	}, false)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize API response: %w", err)
	}

	// 4. 验证 API 响应状态码（必须为 RpcResponseStatusOk）
	if httpResponse.Status != api.RpcResponseStatusOk {
		return nil, fmt.Errorf("API returned error status: %+v", httpResponse.Error)
	}

	// 5. 返回提交结果（HTTP 状态码、响应字节、Base58 编码的交易哈希）
	return &SubmitTransactionResult{
		HttpStatusCode: httpStatusCode,
		HttpRspBytes:   httpResponseBytes,
		HttpRspBody:    httpResponse.Body,
		BodyTxHash:     base58.Encode(httpResponse.Body),
	}, nil
}

// ViewSingle 执行单指令只读查询（单指令）。
func (c *rpcClientV1) ViewSingle(transactionPostcard []byte, requestId uint64) (*ViewSingleTransactionResult, error) {
	// 1. 创建 RPC 请求对象（MethodTypeView，包含 wires 的序列化数据）
	submitTransaction := NewSubmitTransaction(MethodTypeView, requestId, transactionPostcard)

	// 2. 将请求序列化为 JSON 格式并发送 HTTP POST
	bodyData := make([]int, 0)
	for _, value := range submitTransaction.Body {
		bodyData = append(bodyData, int(value))
	}
	httpStatusCode, httpResponseBytes, err := tools.HttpPostByJson(
		context.Background(),
		c.network.RpcUrl,
		map[string]interface{}{
			"method":     submitTransaction.Method,
			"request_id": submitTransaction.RequestId,
			"body":       bodyData,
		},
		map[string]string{
			"Content-Type": ContentTypeMilonJson,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
	}

	// 3. 解析 JSON 格式的 HTTP 响应为 API RpcResponse 结构
	apiResponse := &api.RpcResponse{}
	if err = json.Unmarshal(httpResponseBytes, apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API response: %w", err)
	}

	// 4. 验证 API 响应状态码（必须为 RpcResponseStatusOk）
	if apiResponse.Status != api.RpcResponseStatusOk {
		return nil, fmt.Errorf("API returned error status: %+v", *apiResponse.Error)
	}

	// 5. 返回 ViewSingle 查询结果
	return &ViewSingleTransactionResult{
		HttpStatusCode: httpStatusCode,
		HttpRspBytes:   httpResponseBytes,
		HttpRspBody:    apiResponse.Body,
		BodyValues:     make([]provider.DecodedTaggedValue, 0),
	}, nil
}

// ViewMulti 执行多指令只读查询（多指令）。
func (c *rpcClientV1) ViewMulti(transactionPostcard []byte, requestId uint64) (*ViewMultiTransactionResult, error) {
	// 1. 创建 RPC 请求对象（MethodTypeView，包含 wires 的序列化数据）
	submitTransaction := NewSubmitTransaction(MethodTypeView, requestId, transactionPostcard)

	// 2. 将请求序列化为 JSON 格式并发送 HTTP POST
	bodyData := make([]int, 0)
	for _, value := range submitTransaction.Body {
		bodyData = append(bodyData, int(value))
	}
	httpStatusCode, httpResponseBytes, err := tools.HttpPostByJson(
		context.Background(),
		c.network.RpcUrl,
		map[string]interface{}{
			"method":     submitTransaction.Method,
			"request_id": submitTransaction.RequestId,
			"body":       bodyData,
		},
		map[string]string{
			"Content-Type": ContentTypeMilonJson,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
	}

	// 3. 解析 JSON 格式的 HTTP 响应为 API RpcResponse 结构
	apiResponse := &api.RpcResponse{}
	if err = json.Unmarshal(httpResponseBytes, apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API response: %w", err)
	}

	// 4. 验证 API 响应状态码（必须为 RpcResponseStatusOk）
	if apiResponse.Status != api.RpcResponseStatusOk {
		return nil, fmt.Errorf("API returned error status: %+v", *apiResponse.Error)
	}

	// 5. 返回 ViewMulti 查询结果
	return &ViewMultiTransactionResult{
		HttpStatusCode: httpStatusCode,
		HttpRspBytes:   httpResponseBytes,
		HttpRspBody:    apiResponse.Body,
	}, nil
}

func (c *rpcClientV1) GetResource(rsHash api.RsHash, requestId uint64) (*GetResourceResult, error) {
	// 1. 将  rsHash 解码并序列化为 postcard 格式
	serializer := postcard.NewSerializer()
	serializer.SerializeFixedBytes(rsHash[:])

	// 2. 创建 RPC 请求对象（MethodTypeGetResource）
	submitTransaction := NewSubmitTransaction(MethodTypeGetResource, requestId, serializer.Bytes())

	// 3. 将请求序列化为 JSON 格式并发送 HTTP POST
	bodyData := make([]int, 0)
	for _, value := range submitTransaction.Body {
		bodyData = append(bodyData, int(value))
	}
	httpStatusCode, httpResponseBytes, err := tools.HttpPostByJson(
		context.Background(),
		c.network.RpcUrl,
		map[string]interface{}{
			"method":     submitTransaction.Method,
			"request_id": submitTransaction.RequestId,
			"body":       bodyData,
		},
		map[string]string{
			"Content-Type": ContentTypeMilonJson,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
	}

	// 4. 解析 JSON 格式的 HTTP 响应为 API RpcResponse 结构
	apiResponse := &api.RpcResponse{}
	if err = json.Unmarshal(httpResponseBytes, apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API response: %w", err)
	}

	// 5. 验证 API 响应状态码（必须为 RpcResponseStatusOk）
	if apiResponse.Status != api.RpcResponseStatusOk {
		return nil, fmt.Errorf("API returned error status: %+v", *apiResponse.Error)
	}

	// 6. 反序列化 postcard 格式的响应体为 GetResource 结构
	getResource, err := postcard.DeserializePostcard(apiResponse.Body, func(d *postcard.Deserializer) (*api.GetResource, error) {
		var rsp api.GetResource
		if err = rsp.UnmarshalPostcard(d); err != nil {
			return nil, err
		}
		return &rsp, nil
	}, false)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize GetResource: %w", err)
	}

	// 7. 返回 GetResource 查询结果（BodyGetResource 由调用方后续解码）
	return &GetResourceResult{
		HttpStatusCode:  httpStatusCode,
		HttpRspBytes:    httpResponseBytes,
		HttpRspBody:     apiResponse.Body,
		BodyGetResource: getResource,
	}, nil
}

func (c *rpcClientV1) GetChainHead(requestId uint64) (*ChainHeadResult, error) {
	// 1. 创建 RPC 请求对象（MethodTypeChainHead，空 body 因为不需要参数）
	submitTransaction := NewSubmitTransaction(MethodTypeChainHead, requestId, []byte{})

	// 2. 将请求序列化为 JSON 格式并发送 HTTP POST
	bodyData := make([]int, 0)
	for _, value := range submitTransaction.Body {
		bodyData = append(bodyData, int(value))
	}
	httpStatusCode, httpResponseBytes, err := tools.HttpPostByJson(
		context.Background(),
		c.network.RpcUrl,
		map[string]interface{}{
			"method":     submitTransaction.Method,
			"request_id": submitTransaction.RequestId,
			"body":       bodyData,
		},
		map[string]string{
			"Content-Type": ContentTypeMilonJson,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
	}

	// 3. 解析 JSON 格式的 HTTP 响应为 API RpcResponse 结构
	apiResponse := &api.RpcResponse{}
	if err = json.Unmarshal(httpResponseBytes, apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API response: %w", err)
	}

	// 4. 验证 API 响应状态码（必须为 RpcResponseStatusOk）
	if apiResponse.Status != api.RpcResponseStatusOk {
		return nil, fmt.Errorf("API returned error status: %+v", *apiResponse.Error)
	}

	// 5. 反序列化 postcard 格式的响应体为 ChainHead 结构
	chainHead, err := postcard.DeserializePostcard(apiResponse.Body, func(d *postcard.Deserializer) (*api.ChainHead, error) {
		var rsp api.ChainHead
		if err = rsp.UnmarshalPostcard(d); err != nil {
			return nil, err
		}
		return &rsp, nil
	}, false)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize ChainHead: %w", err)
	}

	// 6. 返回链头查询结果（HTTP 状态码、响应字节、解析后的 ChainHead 结构）
	return &ChainHeadResult{
		HttpStatusCode: httpStatusCode,
		HttpRspBytes:   httpResponseBytes,
		HttpRspBody:    apiResponse.Body,
		BodyChainHead:  chainHead,
	}, nil
}

// ********************************** Submit simulate transaction  **********************************

// BuildAndSimulateSingleIxUnifiedPayerAll 单指令统付模式：构建模拟签名的交易结构（payer签 bit63 和 bit0）
func (c *rpcClientV1) BuildAndSimulateSingleIxUnifiedPayerAll(idlAppName string, methodName string, args provider.Args, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error) {
	// 1. 加载 IDL
	pd, err := c.GetPdByIDLAppName(idlAppName)
	if err != nil {
		return nil, fmt.Errorf("failed to load IDL: %w", err)
	}

	// 2. 编码指令
	wire, err := pd.Encode(methodName, args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode instruction: %w", err)
	}

	// 3. 构建模拟签名的交易结构
	transaction, err := BuildSingleIxUnifiedSimulateSign(payerAddress, acSigMod, wire, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to build unified transaction: %w", err)
	}

	// 4. 序列化
	transactionPostcard, err := transaction.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	// 5. 模拟上链
	return c.SimulateTx(transactionPostcard, requestId)
}

// BuildAndSimulateSingleIxUnifiedDualSign 单指令统付模式的双签名模式：构建模拟签名的交易结构（payer 和 ix 为不同地址分别签 bit63 和 bit0）
func (c *rpcClientV1) BuildAndSimulateSingleIxUnifiedDualSign(idlAppName string, methodName string, args provider.Args, payerAddress crypto.Address, payerAcSigMod AccountSignatureMode, ixAddress crypto.Address, ixAcSigMod AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error) {
	// 1. 加载 IDL
	pd, err := c.GetPdByIDLAppName(idlAppName)
	if err != nil {
		return nil, fmt.Errorf("failed to load IDL: %w", err)
	}

	// 2. 编码指令
	wire, err := pd.Encode(methodName, args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode instruction: %w", err)
	}

	// 3. 构建交易对象
	tx, err := c.CreateTransactionWithParam([]api.PackedInstruction{wire}, &payerAddress, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create tx: %w", err)
	}

	// 4. payer 模拟签名（承担 gas，授权 bit63）
	err = c.SimulateSignPayerAndAddSignature(tx, payerAddress, payerAcSigMod)
	if err != nil {
		return nil, fmt.Errorf("failed to sign payer: %w", err)
	}

	// 5. ix 模拟签名（授权指令执行，授权 bit0）
	err = c.SimulateSignIxAndAddSignature(tx, 0, ixAddress, ixAcSigMod)
	if err != nil {
		return nil, fmt.Errorf("failed to sign ix: %w", err)
	}

	// 6. 序列化
	transactionPostcard, err := tx.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize tx: %w", err)
	}

	// 7. 模拟上链
	return c.SimulateTx(transactionPostcard, requestId)
}

// BuildAndSimulateSingleIxUnifiedPayerOnlyGas 单指令统付模式：构建模拟签名的交易结构（payer 只签 bit63，不授权指令执行）
func (c *rpcClientV1) BuildAndSimulateSingleIxUnifiedPayerOnlyGas(idlAppName string, methodName string, args provider.Args, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error) {
	// 1. 加载 IDL
	pd, err := c.GetPdByIDLAppName(idlAppName)
	if err != nil {
		return nil, fmt.Errorf("failed to load IDL: %w", err)
	}

	// 2. 编码指令
	wire, err := pd.Encode(methodName, args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode instruction: %w", err)
	}

	// 3. 构建模拟签名的交易结构
	transaction, err := BuildSingleIxUnifiedSimulateSignOnlyGas(payerAddress, acSigMod, wire, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to build unified transaction: %w", err)
	}

	// 4. 验证交易结构
	err = transaction.ValidateWire()
	if err != nil {
		return nil, fmt.Errorf("transaction validation failed: %w", err)
	}

	// 5. 序列化
	transactionPostcard, err := transaction.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	// 6. 模拟上链
	return c.SimulateTx(transactionPostcard, requestId)
}

// BuildAndSimulateSingleIxSplit 单指令分账模式：构建模拟签名的交易结构
func (c *rpcClientV1) BuildAndSimulateSingleIxSplit(idlAppName string, methodName string, args provider.Args, ownerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error) {
	// 1. 加载 IDL
	pd, err := c.GetPdByIDLAppName(idlAppName)
	if err != nil {
		return nil, fmt.Errorf("failed to load IDL: %w", err)
	}

	// 2. 编码指令
	wire, err := pd.Encode(methodName, args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode instruction: %w", err)
	}

	// 3. 构建模拟签名的交易结构
	transaction, err := BuildSingleIxSplitSimulateSign(ownerAddress, acSigMod, wire, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to build split transaction: %w", err)
	}

	// 4. 序列化
	transactionPostcard, err := transaction.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	// 5. 模拟上链
	return c.SimulateTx(transactionPostcard, requestId)
}

// Deprecated: Use BuildAndSimulateSingleIxSplit instead. Kept for backward compatibility.
func (c *rpcClientV1) BuildAnSimulateSingleIxSplit(idlAppName string, methodName string, args provider.Args, ownerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error) {
	return c.BuildAndSimulateSingleIxSplit(idlAppName, methodName, args, ownerAddress, acSigMod, requestId, options...)
}

// BuildAndSimulateMultiIxUnified 多指令统付模式：构建模拟签名的交易结构（payer 同时签 bit63 和 ixes）
func (c *rpcClientV1) BuildAndSimulateMultiIxUnified(wires []api.PackedInstruction, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error) {
	// 1. 构建交易对象
	tx, err := c.CreateTransactionWithParam(wires, nil, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create tx: %w", err)
	}

	// 2. 签名：payer 一次性签名所有的指令（ixIndices=[]uint8{0,1,2,...}）+ payer（includePayer=true）
	var ixIndices []uint8
	for i := range wires {
		if i > 63 {
			return nil, fmt.Errorf("too many instructions: index %d exceeds max 63", i)
		}
		ixIndices = append(ixIndices, uint8(i))
	}
	sig, err := tx.SimulateSignIxes(payerAddress, ixIndices, true, acSigMod)
	if err != nil {
		return nil, fmt.Errorf("failed to sign ix and payer: %w", err)
	}
	tx.AddSignature(payerAddress, *sig)

	// 4. 序列化为 postcard 格式
	transactionPostcard, err := tx.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize tx: %w", err)
	}

	// 5. 模拟上链
	return c.SimulateTx(transactionPostcard, requestId)
}

// BuildAndSimulateMultiIxSplit 多指令分账模式：构建模拟签名的交易结构(多个指令由不同账户分别签名)
func (c *rpcClientV1) BuildAndSimulateMultiIxSplit(wires []api.PackedInstruction, ownerAddressList []crypto.Address, acSigModList []AccountSignatureMode, requestId uint64, options ...any) (*SimulateTransactionResult, error) {
	if len(wires) == 0 {
		return nil, fmt.Errorf("wires cannot be empty")
	}

	if len(ownerAddressList) != len(acSigModList) {
		return nil, fmt.Errorf("parameter length mismatch:  ownerAddressList=%d, acSigModList=%d", len(ownerAddressList), len(acSigModList))
	}

	// 1. 构建交易对象
	tx, err := c.CreateTransactionWithParam(wires, nil, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// 2. 循环签名
	for i := range ownerAddressList {
		if i > 63 {
			return nil, fmt.Errorf("too many instructions: index %d exceeds max 63", i)
		}
		sig, err := tx.SimulateSignIxGas(ownerAddressList[i], uint8(i), acSigModList[i])
		if err != nil {
			return nil, fmt.Errorf("failed to sign ix gas at index %d: %w", i, err)
		}
		tx.AddSignature(ownerAddressList[i], *sig)
	}

	// 3. 验证交易结构
	err = tx.ValidateWire()
	if err != nil {
		return nil, fmt.Errorf("transaction validation failed: %w", err)
	}

	// 4. 序列化
	transactionPostcard, err := tx.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	// 5. 模拟上链
	return c.SimulateTx(transactionPostcard, requestId)
}

// ********************************** simulate transaction  **********************************

// ********************************** signature transaction  **********************************

// BuildAndSubmitSingleIxUnifiedPayerSignAll 单指令统付模式：payer 同时签 bit63(gas) 和 bit0(执行)
func (c *rpcClientV1) BuildAndSubmitSingleIxUnifiedPayerSignAll(idlAppName string, methodName string, args provider.Args, payerSk crypto.SecretKeyer, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error) {
	// 1. 加载 IDL
	pd, err := c.GetPdByIDLAppName(idlAppName)
	if err != nil {
		return nil, fmt.Errorf("failed to load IDL: %w", err)
	}

	// 2. 编码指令
	wire, err := pd.Encode(methodName, args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode instruction: %w", err)
	}

	// 3. 单指令统付模式：payer 同时签 bit63(gas) 和 bit0(执行)，设置 tx.Payer
	transaction, err := BuildSingleIxUnifiedPayerSignAll(payerSk, payerAddress, acSigMod, wire, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to build unified transaction: %w", err)
	}

	// 4. 验证交易结构
	err = transaction.ValidateWire()
	if err != nil {
		return nil, fmt.Errorf("transaction validation failed: %w", err)
	}

	// 5. 序列化
	transactionPostcard, err := transaction.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	// 6. 提交上链
	return c.SubmitTx(transactionPostcard, requestId)
}

// BuildAndSubmitSingleIxUnifiedDualSign 单指令统付模式的双签名模式：payer 和 ix 为不同地址（分别签 bit63 和 bit0）
func (c *rpcClientV1) BuildAndSubmitSingleIxUnifiedDualSign(idlAppName string, methodName string, args provider.Args, payerSk crypto.SecretKeyer, payerAddress crypto.Address, payerAcSigMod AccountSignatureMode, ixSk crypto.SecretKeyer, ixAddress crypto.Address, ixAcSigMod AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error) {
	// 1. 加载 IDL
	pd, err := c.GetPdByIDLAppName(idlAppName)
	if err != nil {
		return nil, fmt.Errorf("failed to load IDL: %w", err)
	}

	// 2. 编码指令
	wire, err := pd.Encode(methodName, args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode instruction: %w", err)
	}

	// 3. 构建交易对象
	tx, err := c.CreateTransactionWithParam([]api.PackedInstruction{wire}, &payerAddress, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create tx: %w", err)
	}

	// 4. 双签名模式 payer 签名（承担 gas，授权 bit63）
	err = c.SignPayerAndAddSignature(tx, payerSk, payerAddress, payerAcSigMod)
	if err != nil {
		return nil, fmt.Errorf("failed to sign payer: %w", err)
	}

	// 5. ix 签名（授权指令执行，授权 bit0）
	err = c.SignIxAndAddSignature(tx, 0, ixSk, ixAddress, ixAcSigMod)
	if err != nil {
		return nil, fmt.Errorf("failed to sign ix: %w", err)
	}

	// 6. 验证交易结构
	err = tx.ValidateWire()
	if err != nil {
		return nil, fmt.Errorf("tx validation failed: %w", err)
	}

	// 7. 序列化
	transactionPostcard, err := tx.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize tx: %w", err)
	}

	// 8. 提交上链
	return c.SubmitTx(transactionPostcard, requestId)
}

// BuildAndSubmitSingleIxUnifiedPayerOnlyGas 单指令统付模式：payer 只签 bit63(gas)，不授权指令执行
func (c *rpcClientV1) BuildAndSubmitSingleIxUnifiedPayerOnlyGas(idlAppName string, methodName string, args provider.Args, payerSk crypto.SecretKeyer, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error) {
	// 1. 加载 IDL
	pd, err := c.GetPdByIDLAppName(idlAppName)
	if err != nil {
		return nil, fmt.Errorf("failed to load IDL: %w", err)
	}

	// 2. 编码指令
	wire, err := pd.Encode(methodName, args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode instruction: %w", err)
	}

	// 3. 单指令统付模式：payer 只签 bit63(gas)，不授权指令执行
	transaction, err := BuildSingleIxUnifiedPayerSignOnlyGas(payerSk, payerAddress, acSigMod, wire, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to build unified transaction: %w", err)
	}

	// 4. 验证交易结构
	err = transaction.ValidateWire()
	if err != nil {
		return nil, fmt.Errorf("transaction validation failed: %w", err)
	}

	// 5. 序列化
	transactionPostcard, err := transaction.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	// 6. 提交上链
	return c.SubmitTx(transactionPostcard, requestId)
}

// BuildAndSubmitSingleIxSplit 单指令分账模式：owner 同时承担 gas 和执行
func (c *rpcClientV1) BuildAndSubmitSingleIxSplit(idlAppName string, methodName string, args provider.Args, ownerSk crypto.SecretKeyer, ownerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error) {
	// 1. 加载 IDL
	pd, err := c.GetPdByIDLAppName(idlAppName)
	if err != nil {
		return nil, fmt.Errorf("failed to load IDL: %w", err)
	}

	// 2. 编码指令
	wire, err := pd.Encode(methodName, args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode instruction: %w", err)
	}

	// 3. 分账模式：单指令分账模式：owner 同时承担 gas 和执行（tx.Payer=nil，签 bit63+bit0）
	transaction, err := BuildSingleIxSplitSign(ownerSk, ownerAddress, acSigMod, wire, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to build split transaction: %w", err)
	}

	// 4. 验证交易结构
	err = transaction.ValidateWire()
	if err != nil {
		return nil, fmt.Errorf("transaction validation failed: %w", err)
	}

	// 5. 序列化
	transactionPostcard, err := transaction.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	// 6. 提交上链
	return c.SubmitTx(transactionPostcard, requestId)
}

// BuildAndSubmitMultiIxUnified 多指令统付模式：payer 同时签 bit63(gas) 和 ixes
func (c *rpcClientV1) BuildAndSubmitMultiIxUnified(wires []api.PackedInstruction, payerSk crypto.SecretKeyer, payerAddress crypto.Address, acSigMod AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error) {
	// 1. 构建交易对象
	tx, err := c.CreateTransactionWithParam(wires, nil, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create tx: %w", err)
	}

	// 2. 签名：payer 一次性签名所有的指令（ixIndices=[]uint8{0,1,2,...}）+ payer（includePayer=true）
	var ixIndices []uint8
	for i := range wires {
		if i > 63 {
			return nil, fmt.Errorf("too many instructions: index %d exceeds max 63", i)
		}
		ixIndices = append(ixIndices, uint8(i))
	}
	sig, err := tx.SignIxes(payerAddress, payerSk, ixIndices, true, acSigMod)
	if err != nil {
		return nil, fmt.Errorf("failed to sign ix and payer: %w", err)
	}
	tx.AddSignature(payerAddress, *sig)

	// 3. 验证交易结构（检查指令格式、签名完整性等）
	err = tx.ValidateWire()
	if err != nil {
		return nil, fmt.Errorf("tx validation failed: %w", err)
	}

	// 4. 序列化为 postcard 格式
	transactionPostcard, err := tx.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize tx: %w", err)
	}

	// 5. 提交上链
	return c.SubmitTx(transactionPostcard, requestId)
}

// BuildAndSubmitMultiIxSplit 多指令分账模式：多个指令由不同账户分别签名
func (c *rpcClientV1) BuildAndSubmitMultiIxSplit(wires []api.PackedInstruction, ownerSkList []crypto.SecretKeyer, ownerAddressList []crypto.Address, acSigModList []AccountSignatureMode, requestId uint64, options ...any) (*SubmitTransactionResult, error) {
	if len(wires) == 0 {
		return nil, fmt.Errorf("wires cannot be empty")
	}

	if len(ownerSkList) != len(ownerAddressList) || len(ownerSkList) != len(acSigModList) {
		return nil, fmt.Errorf("parameter length mismatch: ownerSkList=%d, ownerAddressList=%d, acSigModList=%d",
			len(ownerSkList), len(ownerAddressList), len(acSigModList))
	}

	// 1. 构建交易对象
	tx, err := c.CreateTransactionWithParam(wires, nil, options...)
	if err != nil {
		return nil, fmt.Errorf("failed to create transaction: %w", err)
	}

	// 2. 循环签名
	for i := range ownerSkList {
		if i > 63 {
			return nil, fmt.Errorf("too many instructions: index %d exceeds max 63", i)
		}
		sig, err := tx.SignIxGas(ownerAddressList[i], ownerSkList[i], uint8(i), acSigModList[i])
		if err != nil {
			return nil, fmt.Errorf("failed to sign ix gas at index %d: %w", i, err)
		}
		tx.AddSignature(ownerAddressList[i], *sig)
	}

	// 3. 验证交易结构
	err = tx.ValidateWire()
	if err != nil {
		return nil, fmt.Errorf("transaction validation failed: %w", err)
	}

	// 4. 序列化
	transactionPostcard, err := tx.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	// 5. 提交上链
	return c.SubmitTx(transactionPostcard, requestId)
}

//********************************** signature transaction  **********************************

// BuildAndViewSingleIx 构建并执行 View 方法调用（只读查询，不修改链上状态）
func (c *rpcClientV1) BuildAndViewSingleIx(idlAppName string, methodName string, args provider.Args, requestId uint64) (*ViewSingleTransactionResult, error) {
	// 1. 加载 IDL
	pd, err := c.GetPdByIDLAppName(idlAppName)
	if err != nil {
		return nil, fmt.Errorf("failed to load IDL: %w", err)
	}

	// 2. 编码指令
	wire, err := pd.Encode(methodName, args)
	if err != nil {
		return nil, fmt.Errorf("failed to encode instruction: %w", err)
	}

	// 3. 构建 wires 数组并序列化为 postcard 格式
	wires := []api.PackedInstruction{wire}

	// 序列化 wires 为 postcard 格式
	serializer := postcard.NewSerializer()
	if err = serializer.SerializeU32(uint32(len(wires))); err != nil {
		return nil, fmt.Errorf("failed to serialize wires length: %w", err)
	}
	for _, w := range wires {
		if err = serializer.SerializeBytes(w); err != nil {
			return nil, fmt.Errorf("failed to serialize wire: %w", err)
		}
	}
	wiresPostcard := serializer.Bytes()

	// 4. 调用 ViewSingle RPC 方法（直接传入 wires 的序列化数据）
	viewSingleTransactionResult, err := c.ViewSingle(wiresPostcard, requestId)
	if err != nil {
		return nil, fmt.Errorf("failed to view transaction: %w", err)
	}

	// 5. 解码返回值
	viewSingleTransactionResult.BodyValues, err = pd.DecodeViewData(methodName, viewSingleTransactionResult.HttpRspBody)
	if err != nil {
		return nil, fmt.Errorf("failed to decode view values: %w", err)
	}

	return viewSingleTransactionResult, nil
}

func (c *rpcClientV1) BuildAndViewMultiIx(wires []api.PackedInstruction, requestId uint64) (*ViewMultiTransactionResult, error) {
	var err error

	// 1.序列化 wires 为 postcard 格式
	serializer := postcard.NewSerializer()
	if err = serializer.SerializeU32(uint32(len(wires))); err != nil {
		return nil, fmt.Errorf("failed to serialize wires length: %w", err)
	}
	for _, w := range wires {
		if err = serializer.SerializeBytes(w); err != nil {
			return nil, fmt.Errorf("failed to serialize wire: %w", err)
		}
	}
	wiresPostcard := serializer.Bytes()

	// 2. ViewMulti RPC 方法（直接传入 wires 的序列化数据）
	viewTransactionResult, err := c.ViewMulti(wiresPostcard, requestId)
	if err != nil {
		return nil, fmt.Errorf("failed to view transaction: %w", err)
	}

	return viewTransactionResult, nil
}

// GetTxByHash 根据交易哈希查询链上交易的详细信息
func (c *rpcClientV1) GetTxByHash(txHashRelaxed string, requestId uint64) (*GetTxByHashResult, error) {
	// 1. 将 txHashRelaxed 解码并序列化为 postcard 格式
	txHash, err := api.NewTxHashFromStringRelaxed(txHashRelaxed)
	if err != nil {
		return nil, fmt.Errorf("failed to decode txHashRelaxed: %w", err)
	}

	serializer := postcard.NewSerializer()
	if err = serializer.SerializeBytes(txHash[:]); err != nil {
		return nil, fmt.Errorf("failed to serialize txHash: %w", err)
	}

	// 2. 创建 RPC 请求对象（MethodTypeGetTxByHash）
	submitTransaction := NewSubmitTransaction(MethodTypeGetTxByHash, requestId, serializer.Bytes())

	// 3. 将请求序列化为 JSON 格式并发送 HTTP POST
	bodyData := make([]int, 0)
	for _, value := range submitTransaction.Body {
		bodyData = append(bodyData, int(value))
	}
	httpStatusCode, httpResponseBytes, err := tools.HttpPostByJson(
		context.Background(),
		c.network.RpcUrl,
		map[string]interface{}{
			"method":     submitTransaction.Method,
			"request_id": submitTransaction.RequestId,
			"body":       bodyData,
		},
		map[string]string{
			"Content-Type": ContentTypeMilonJson,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
	}

	// 4. 解析 JSON 格式的 HTTP 响应为 API RpcResponse 结构
	apiResponse := &api.RpcResponse{}
	if err = json.Unmarshal(httpResponseBytes, apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API response: %w", err)
	}
	if apiResponse.Status != api.RpcResponseStatusOk {
		return nil, fmt.Errorf("API returned error status: %+v", apiResponse.Error)
	}

	// 5. 反序列化 postcard 格式的响应体为 TxHistory 结构
	txHistory, err := postcard.DeserializePostcard(apiResponse.Body, func(d *postcard.Deserializer) (*api.TxHistory, error) {
		var rsp api.TxHistory
		if err = rsp.UnmarshalPostcard(d); err != nil {
			return nil, err
		}
		return &rsp, nil
	}, false)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize TxHistory: %w", err)
	}

	// 6. 返回交易查询结果（HTTP 状态码、响应字节、解析后的 TxHistory）
	return &GetTxByHashResult{
		HttpStatusCode: httpStatusCode,
		HttpRspBytes:   httpResponseBytes,
		HttpRspBody:    apiResponse.Body,
		BodyTxHistory:  txHistory,
	}, nil
}

func (c *rpcClientV1) GetAccount(addressBase58 string, requestId uint64) (*GetAccountResult, error) {
	// 1. 将 Base58 编码的 address 解码并校验长度，然后序列化为 postcard 格式
	addrBytes := base58.Decode(addressBase58)
	if len(addrBytes) != crypto.AddressRawLen {
		return nil, fmt.Errorf("invalid address length: expected %d, got %d", crypto.AddressRawLen, len(addrBytes))
	}
	serializer := postcard.NewSerializer()
	serializer.SerializeFixedBytes(addrBytes)

	// 2. 创建 RPC 请求对象（MethodTypeGetAccount）
	submitTransaction := NewSubmitTransaction(MethodTypeGetAccount, requestId, serializer.Bytes())

	// 3. 将请求序列化为 JSON 格式并发送 HTTP POST
	bodyData := make([]int, 0)
	for _, value := range submitTransaction.Body {
		bodyData = append(bodyData, int(value))
	}
	httpStatusCode, httpResponseBytes, err := tools.HttpPostByJson(
		context.Background(),
		c.network.RpcUrl,
		map[string]interface{}{
			"method":     submitTransaction.Method,
			"request_id": submitTransaction.RequestId,
			"body":       bodyData,
		},
		map[string]string{
			"Content-Type": ContentTypeMilonJson,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
	}

	// 4. 解析 JSON 格式的 HTTP 响应为 API RpcResponse 结构
	apiResponse := &api.RpcResponse{}
	if err = json.Unmarshal(httpResponseBytes, apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API response: %w", err)
	}
	if apiResponse.Status != api.RpcResponseStatusOk {
		return nil, fmt.Errorf("API returned error status: %+v", *apiResponse.Error)
	}

	// 5.反序列化 postcard 格式的响应体为 AccountView 结构
	accountView, err := postcard.DeserializePostcard(apiResponse.Body, func(d *postcard.Deserializer) (*api.AccountView, error) {
		var rsp api.AccountView
		if err = rsp.UnmarshalPostcard(d); err != nil {
			return nil, err
		}
		return &rsp, nil
	}, false)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize AccountView: %w", err)
	}

	// 6. 返回账户查询结果（HTTP 状态码、响应字节、解析后的 AccountView）
	return &GetAccountResult{
		HttpStatusCode:  httpStatusCode,
		HttpRspBytes:    httpResponseBytes,
		HttpRspBody:     apiResponse.Body,
		BodyAccountView: accountView,
	}, nil
}

func (c *rpcClientV1) GetBlock(blockHeight uint64, requestId uint64) (*GetBlockByHeightResult, error) {
	// 1. 将 blockHeight 序列化为 postcard 格式
	serializer := postcard.NewSerializer()
	err := serializer.SerializeU64(blockHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize blockHeight: %w", err)
	}

	// 2. 创建 RPC 请求对象（MethodTypeGetBlockByHeight）
	submitTransaction := NewSubmitTransaction(MethodTypeGetBlockByHeight, requestId, serializer.Bytes())

	// 3. 将请求序列化为 JSON 格式并发送 HTTP POST
	bodyData := make([]int, 0)
	for _, value := range submitTransaction.Body {
		bodyData = append(bodyData, int(value))
	}
	httpStatusCode, httpResponseBytes, err := tools.HttpPostByJson(
		context.Background(),
		c.network.RpcUrl,
		map[string]interface{}{
			"method":     submitTransaction.Method,
			"request_id": submitTransaction.RequestId,
			"body":       bodyData,
		},
		map[string]string{
			"Content-Type": ContentTypeMilonJson,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
	}

	// 4. 解析 JSON 格式的 HTTP 响应为 API RpcResponse 结构
	apiResponse := &api.RpcResponse{}
	if err = json.Unmarshal(httpResponseBytes, apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API response: %w", err)
	}
	if apiResponse.Status != api.RpcResponseStatusOk {
		return nil, fmt.Errorf("API returned error status: %+v", *apiResponse.Error)
	}

	// 5.反序列化 postcard 格式的响应体为 Block 结构
	block, err := postcard.DeserializePostcard(apiResponse.Body, func(d *postcard.Deserializer) (*api.Block, error) {
		var rsp api.Block
		if err = rsp.UnmarshalPostcard(d); err != nil {
			return nil, err
		}
		return &rsp, nil
	}, false)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize Block: %w", err)
	}

	// 6. 返回区块查询结果（HTTP 状态码、响应字节、解析后的 BodyBlock）
	return &GetBlockByHeightResult{
		HttpStatusCode: httpStatusCode,
		HttpRspBytes:   httpResponseBytes,
		HttpRspBody:    apiResponse.Body,
		BodyBlock:      block,
	}, nil
}

func (c *rpcClientV1) EventsByTxHash(txHashRelaxed string, typeTagFilter *uint64, requestId uint64) (*EventsByTxHashResult, error) {
	txHash, err := api.NewTxHashFromStringRelaxed(txHashRelaxed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse txHash: %w", err)
	}

	serializer := postcard.NewSerializer()
	eventsByTxHashRequest := api.EventsByTxHashReq{
		TxHash:        txHash,
		TypeTagFilter: typeTagFilter,
	}
	err = eventsByTxHashRequest.MarshalPostcard(serializer)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize EventsByTxHashReq: %w", err)
	}

	// 2. 创建 RPC 请求对象（MethodTypeEventsByTxHash）
	submitTransaction := NewSubmitTransaction(MethodTypeEventsByTxHash, requestId, serializer.Bytes())

	// 3. 将请求序列化为 JSON 格式并发送 HTTP POST
	bodyData := make([]int, 0)
	for _, value := range submitTransaction.Body {
		bodyData = append(bodyData, int(value))
	}
	httpStatusCode, httpResponseBytes, err := tools.HttpPostByJson(
		context.Background(),
		c.network.RpcUrl,
		map[string]interface{}{
			"method":     submitTransaction.Method,
			"request_id": submitTransaction.RequestId,
			"body":       bodyData,
		},
		map[string]string{
			"Content-Type": ContentTypeMilonJson,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
	}

	// 4. 解析 JSON 格式的 HTTP 响应为 API RpcResponse 结构
	apiResponse := &api.RpcResponse{}
	if err = json.Unmarshal(httpResponseBytes, apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API response: %w", err)
	}
	if apiResponse.Status != api.RpcResponseStatusOk {
		return nil, fmt.Errorf("API returned error status: %+v", *apiResponse.Error)
	}

	// 5.反序列化 postcard 格式的响应体为 EventsByTxHash 结构
	eventsByTxHashResponse, err := postcard.DeserializePostcard(apiResponse.Body, func(d *postcard.Deserializer) (*api.EventsByTxHash, error) {
		var rsp api.EventsByTxHash
		if err = rsp.UnmarshalPostcard(d); err != nil {
			return nil, err
		}
		return &rsp, nil
	}, false)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize EventsByTxHash: %w", err)
	}

	// 6. 返回事件查询结果（HTTP 状态码、响应字节、解析后的 EventsByTxHash）
	return &EventsByTxHashResult{
		HttpStatusCode:     httpStatusCode,
		HttpRspBytes:       httpResponseBytes,
		HttpRspBody:        apiResponse.Body,
		BodyEventsByTxHash: eventsByTxHashResponse,
	}, nil
}

func (c *rpcClientV1) ListResourcePath(requestId uint64) (*ListResourcePathResult, error) {
	// 1. 创建 RPC 请求对象（MethodTypeListResourcePath）
	submitTransaction := NewSubmitTransaction(MethodTypeListResourcePath, requestId, []byte{})

	// 2. 将请求序列化为 JSON 格式并发送 HTTP POST
	bodyData := make([]int, 0)
	for _, value := range submitTransaction.Body {
		bodyData = append(bodyData, int(value))
	}
	httpStatusCode, httpResponseBytes, err := tools.HttpPostByJson(
		context.Background(),
		c.network.RpcUrl,
		map[string]interface{}{
			"method":     submitTransaction.Method,
			"request_id": submitTransaction.RequestId,
			"body":       bodyData,
		},
		map[string]string{
			"Content-Type": ContentTypeMilonJson,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
	}

	// 3. 解析 JSON 格式的 HTTP 响应为 API RpcResponse 结构
	apiResponse := &api.RpcResponse{}
	if err = json.Unmarshal(httpResponseBytes, apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API response: %w", err)
	}
	if apiResponse.Status != api.RpcResponseStatusOk {
		return nil, fmt.Errorf("API returned error status: %+v", *apiResponse.Error)
	}

	// 4. 反序列化 JSON 格式的响应体为 []ListResourcePathInfo 结构
	var rawList [][]any
	if err = json.Unmarshal(apiResponse.Body, &rawList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ListResourcePathInfo response: %w", err)
	}

	listResourcePaths, err := api.UnmarshalListResourcePathListFromRawList(rawList)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ListResourcePathInfo: %w", err)
	}

	// 5. 返回资源路径查询结果（HTTP 状态码、响应字节、解析后的 []api.ListResourcePathInfo）
	return &ListResourcePathResult{
		HttpStatusCode:        httpStatusCode,
		HttpRspBytes:          httpResponseBytes,
		HttpRspBody:           apiResponse.Body,
		BodyListResourcePaths: listResourcePaths,
	}, nil
}

func (c *rpcClientV1) GetResourcePathByHash(rsHash api.RsHash, requestId uint64) (*GetResourcePathByHashResult, error) {
	// 1. 创建 RPC 请求对象（MethodTypeGetResourcePathByHash）
	submitTransaction := NewSubmitTransaction(MethodTypeGetResourcePathByHash, requestId, rsHash[:])

	// 2. 将请求序列化为 JSON 格式并发送 HTTP POST
	bodyData := make([]int, 0)
	for _, value := range submitTransaction.Body {
		bodyData = append(bodyData, int(value))
	}
	httpStatusCode, httpResponseBytes, err := tools.HttpPostByJson(
		context.Background(),
		c.network.RpcUrl,
		map[string]interface{}{
			"method":     submitTransaction.Method,
			"request_id": submitTransaction.RequestId,
			"body":       bodyData,
		},
		map[string]string{
			"Content-Type": ContentTypeMilonJson,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
	}

	// 3. 解析 JSON 格式的 HTTP 响应为 API RpcResponse 结构
	apiResponse := &api.RpcResponse{}
	if err = json.Unmarshal(httpResponseBytes, apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API response: %w", err)
	}
	if apiResponse.Status != api.RpcResponseStatusOk {
		return nil, fmt.Errorf("API returned error status: %+v", *apiResponse.Error)
	}

	// 4. 返回资源路径查询结果（HTTP 状态码、响应字节）
	return &GetResourcePathByHashResult{
		HttpStatusCode: httpStatusCode,
		HttpRspBytes:   httpResponseBytes,
		HttpRspBody:    apiResponse.Body,
	}, nil
}

func (c *rpcClientV1) GetAccessValue(blobHashList []api.BlobHash, requestId uint64) (*GetAccessValueResult, error) {
	// 1. 序列化 blobHashList 为 postcard 格式
	serializer := postcard.NewSerializer()
	if err := postcard.SerializeSeq(serializer, blobHashList, func(s *postcard.Serializer, bh api.BlobHash) error {
		s.SerializeFixedBytes(bh[:])
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to serialize blobHashList: %w", err)
	}

	// 2. 创建 RPC 请求对象（MethodTypeGetAccessValue）
	submitTransaction := NewSubmitTransaction(MethodTypeGetAccessValue, requestId, serializer.Bytes())

	// 3. 将请求序列化为 JSON 格式并发送 HTTP POST
	bodyData := make([]int, 0)
	for _, value := range submitTransaction.Body {
		bodyData = append(bodyData, int(value))
	}
	httpStatusCode, httpResponseBytes, err := tools.HttpPostByJson(
		context.Background(),
		c.network.RpcUrl,
		map[string]interface{}{
			"method":     submitTransaction.Method,
			"request_id": submitTransaction.RequestId,
			"body":       bodyData,
		},
		map[string]string{
			"Content-Type": ContentTypeMilonJson,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
	}

	// 4. 解析 JSON 格式的 HTTP 响应为 API RpcResponse 结构
	apiResponse := &api.RpcResponse{}
	if err = json.Unmarshal(httpResponseBytes, apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API response: %w", err)
	}
	if apiResponse.Status != api.RpcResponseStatusOk {
		return nil, fmt.Errorf("API returned error status: %+v", *apiResponse.Error)
	}

	// 5. 反序列化 postcard 格式的响应体为 []GetAccessValueInfo 结构
	deserializer := postcard.NewDeserializer(apiResponse.Body)
	accessValues, err := postcard.DeserializeSeq(deserializer, func(d *postcard.Deserializer) (*api.GetAccessValueInfo, error) {
		var info api.GetAccessValueInfo
		if err = info.UnmarshalPostcard(d); err != nil {
			return nil, fmt.Errorf("failed to deserialize GetAccessValueInfo: %w", err)
		}
		return &info, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize GetAccessValue sequence: %w", err)
	}

	// 6. 返回访问值查询结果（HTTP 状态码、响应字节、解析后的 BodyGetAccessValues）
	return &GetAccessValueResult{
		HttpStatusCode:      httpStatusCode,
		HttpRspBytes:        httpResponseBytes,
		HttpRspBody:         apiResponse.Body,
		BodyGetAccessValues: accessValues,
	}, nil
}

func (c *rpcClientV1) WaitForTransaction(txHashRelaxed string, requestId uint64, options ...any) (*GetTxByHashResult, error) {
	// 设置默认轮询参数
	pollPeriod := 200 * time.Millisecond //轮询间隔选项
	pollTimeout := 20 * time.Second      //轮询超时选项

	// 解析可选参数
	for _, opt := range options {
		switch v := opt.(type) {
		case PollPeriod:
			pollPeriod = time.Duration(v)
		case PollTimeout:
			pollTimeout = time.Duration(v)
		default:
			return nil, fmt.Errorf("unknown option type: %T", opt)
		}
	}

	start := time.Now()
	deadline := start.Add(pollTimeout)

	// 轮询直到交易完成或超时
	consecutiveErrors := 0
	const maxConsecutiveErrors = 10
	for {
		// 检查是否超时
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("WaitForTransaction timeout after %v", pollTimeout)
		}

		// 等待轮询间隔
		time.Sleep(pollPeriod)

		// 查询交易状态
		result, err := c.GetTxByHash(txHashRelaxed, requestId)
		if err != nil {
			// 查询失败，连续错误计数，超过阈值后返回错误（避免无限轮询）
			consecutiveErrors++
			if consecutiveErrors >= maxConsecutiveErrors {
				return nil, fmt.Errorf("WaitForTransaction failed after %d consecutive errors: %w", maxConsecutiveErrors, err)
			}
			continue
		}
		consecutiveErrors = 0

		// 交易仍在 Pending 状态，继续轮询
		if result.BodyTxHistory.Receipt.State == api.TxStatePending {
			continue
		}

		return result, nil
	}
}
