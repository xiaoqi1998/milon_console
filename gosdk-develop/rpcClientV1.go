package milon

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"github.com/btcsuite/btcutil/base58"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/milon-labs/milon-go-sdk/provider"
	"github.com/milon-labs/milon-go-sdk/tools"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
	"time"
)

type ChainHeadResult struct {
	HttpRspBytes  []byte
	HttpRspBody   []byte
	BodyChainHead *api.ChainHead
}

type SimulateTransactionResult struct {
	HttpRspBody         []byte
	BodySimulateReceipt *api.SimulateReceipt
}

type ViewSingleTransactionResult struct {
	HttpRspBytes []byte
	HttpRspBody  []byte
	BodyValues   any
}

type ViewMultiTransactionResult struct {
	HttpRspBytes []byte
	HttpRspBody  []byte
}

type GetResourceResult struct {
	HttpRspBytes    []byte
	HttpRspBody     []byte
	BodyGetResource *api.GetResource
}

type GetBlockByHeightResult struct {
	HttpRspBytes []byte
	HttpRspBody  []byte
	BodyBlock    *api.Block
}

type GetTxByHashResult struct {
	HttpRspBytes  []byte
	HttpRspBody   []byte
	BodyTxHistory *api.TxHistory
}

type GetAccountResult struct {
	HttpRspBytes    []byte
	HttpRspBody     []byte
	BodyAccountView *api.AccountView
}

type EventsByTxHashResult struct {
	HttpRspBytes       []byte
	HttpRspBody        []byte
	BodyEventsByTxHash *api.EventsByTxHash
}

type ListResourcePathResult struct {
	HttpRspBytes          []byte
	HttpRspBody           []byte
	BodyListResourcePaths []*api.ListResourcePathInfo
}

type GetResourcePathByHashResult struct {
	HttpRspBytes []byte
	HttpRspBody  []byte
}

type GetAccessValueResult struct {
	HttpRspBytes        []byte
	HttpRspBody         []byte
	BodyGetAccessValues []*api.GetAccessValueInfo
}

type rpcClientV1 struct {
	network           Network
	providerByIDLName map[string]*provider.Provider
	providerManager   *provider.IDLManager
	pollPeriod        time.Duration
	pollTimeout       time.Duration
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

	dir := indexFilePath[:strings.LastIndex(indexFilePath, "/")]

	for _, app := range indexConfig.Apps {
		idlPath := dir + "/" + app.IDL
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

func (c *rpcClientV1) ClaimFaucet(claimerSk crypto.SecretKeyer, claimerAddress crypto.Address, mode lib.AccountSignatureMode) error {
	// 1. load IDL
	pd, err := c.GetPdByIDLAppName("token")
	if err != nil {
		return fmt.Errorf("failed to load IDL: %w", err)
	}

	// 2. Encode instruction
	wire, err := pd.Encode(
		"ClaimFaucet",
		provider.Args{
			"claimer": claimerAddress,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to encode instruction: %w", err)
	}

	// 3. Split-payer mode: the claimer both pays gas and executes the instruction (tx.Payer=nil, signs bit63 + bit0)
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).AddIxAndPayerSig(claimerAddress, claimerSk, 0, mode).Build()
	if err != nil {
		return fmt.Errorf("failed to build split transaction: %w", err)
	}

	// 4. Submit transaction on chain
	err = c.SubmitTx(tx)
	if err != nil {
		return fmt.Errorf("failed to ClaimFaucet: %w", err)
	}

	// 5. Wait for the transaction to complete
	_, err = c.WaitForTransaction(tx.TxHash(), 1)
	if err != nil {
		return fmt.Errorf("failed to wait for transaction: %w", err)
	}

	return nil
}

func (c *rpcClientV1) BalanceOf(address crypto.Address) (uint64, error) {
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
		return 0, fmt.Errorf("failed to BalanceOf: %w", err)
	}

	return viewSingleTransactionResult.BodyValues.(uint64), nil
}

func (c *rpcClientV1) SimulateTx(transaction *lib.Transaction, options ...any) (*SimulateTransactionResult, error) {
	// 1. 序列化交易
	txPostcard, err := transaction.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	// 2. 解析可选参数
	requestId := lib.RequestID(time.Now().UnixMilli())
	for _, opt := range options {
		switch v := opt.(type) {
		case lib.RequestID:
			requestId = v
		default:
			return nil, fmt.Errorf("invalid option type: %T", v)
		}
	}

	// 3. 创建 RPC 请求对象（MethodTypeSimulateTx，包含已序列化的交易数据）
	rpcReq := lib.NewRpcRequest(lib.MethodTypeSimulateTx, requestId, txPostcard)

	// 4. 将请求序列化为 postcard 格式并发送 HTTP POST
	rpcReqPostcard, err := postcard.SerializePostcard(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize submit transaction: %w", err)
	}
	httpStatusCode, httpResponseBytes, err := tools.HttpPostByBytes(
		context.Background(),
		c.network.RpcUrl,
		rpcReqPostcard,
		map[string]string{
			"Content-Type": lib.ContentTypeMilonPostcard,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
	}

	// 5. 反序列化 postcard 格式的 HTTP 响应为 API RpcResponse 结构
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

	// 6. 验证 API 响应状态码（必须为 RpcResponseStatusOk）
	if httpResponse.Status != api.RpcResponseStatusOk {
		return nil, fmt.Errorf("API returned error status xxx: %+v", httpResponse.Error)
	}

	// 7. 反序列化 postcard 格式的响应体为 SimulateReceipt 结构
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

	// 8. 返回模拟结果
	return &SimulateTransactionResult{
		HttpRspBody:         httpResponse.Body,
		BodySimulateReceipt: simulateReceipt,
	}, nil
}

func (c *rpcClientV1) SubmitTx(tx *lib.Transaction, options ...any) error {
	// 1. 验证交易结构
	err := tx.ValidateWire()
	if err != nil {
		return fmt.Errorf("transaction validation failed: %w", err)
	}

	// 2. 序列化交易
	txPostcard, err := tx.ToBytes()
	if err != nil {
		return fmt.Errorf("failed to serialize transaction: %w", err)
	}

	// 3. 解析可选参数
	requestId := lib.RequestID(time.Now().UnixMilli())
	for _, opt := range options {
		switch v := opt.(type) {
		case lib.RequestID:
			requestId = v
		default:
			return fmt.Errorf("invalid option type: %T", v)
		}
	}

	// 4. 创建 RPC 请求对象（MethodTypeSubmitTx，包含已序列化的交易数据）
	rpcReq := lib.NewRpcRequest(lib.MethodTypeSubmitTx, requestId, txPostcard)

	// 5. 将请求序列化为 postcard 格式并发送 HTTP POST
	rpcReqPostcard, err := postcard.SerializePostcard(rpcReq)
	if err != nil {
		return fmt.Errorf("failed to serialize submit transaction: %w", err)
	}
	httpStatusCode, httpResponseBytes, err := tools.HttpPostByBytes(
		context.Background(),
		c.network.RpcUrl,
		rpcReqPostcard,
		map[string]string{
			"Content-Type": lib.ContentTypeMilonPostcard,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}
	if httpStatusCode != http.StatusOK {
		return fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
	}

	// 6. 反序列化 postcard 格式的 HTTP 响应为 API RpcResponse 结构
	httpResponse, err := postcard.DeserializePostcard(httpResponseBytes, func(d *postcard.Deserializer) (*api.RpcResponse, error) {
		var rsp api.RpcResponse
		if err = rsp.UnmarshalPostcard(d); err != nil {
			return nil, err
		}
		return &rsp, nil
	}, false)
	if err != nil {
		return fmt.Errorf("failed to deserialize API response: %w", err)
	}

	// 7. 验证 API 响应状态码（必须为 RpcResponseStatusOk）
	if httpResponse.Status != api.RpcResponseStatusOk {
		return fmt.Errorf("API returned error status: %+v", httpResponse.Error)
	}

	return nil
}

func (c *rpcClientV1) ViewSingle(transactionPostcard []byte, requestId lib.RequestID) (*ViewSingleTransactionResult, error) {
	// 1. 创建 RPC 请求对象（MethodTypeView，包含 wires 的序列化数据）
	submitTransaction := lib.NewRpcRequest(lib.MethodTypeView, requestId, transactionPostcard)

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
			"Content-Type": lib.ContentTypeMilonJson,
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
	return &ViewSingleTransactionResult{
		HttpRspBytes: httpResponseBytes,
		HttpRspBody:  apiResponse.Body,
		BodyValues:   make([]provider.DecodedTaggedValue, 0),
	}, nil
}

func (c *rpcClientV1) ViewMulti(transactionPostcard []byte, requestId lib.RequestID) (*ViewMultiTransactionResult, error) {
	// 1. 创建 RPC 请求对象（MethodTypeView，包含 wires 的序列化数据）
	submitTransaction := lib.NewRpcRequest(lib.MethodTypeView, requestId, transactionPostcard)

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
			"Content-Type": lib.ContentTypeMilonJson,
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
		HttpRspBytes: httpResponseBytes,
		HttpRspBody:  apiResponse.Body,
	}, nil
}

func (c *rpcClientV1) GetResource(rsHash api.RsHash, requestId lib.RequestID) (*GetResourceResult, error) {
	// 1. 将  rsHash 解码并序列化为 postcard 格式
	serializer := postcard.NewSerializer()
	serializer.SerializeFixedBytes(rsHash[:])

	// 2. 创建 RPC 请求对象（MethodTypeGetResource）
	submitTransaction := lib.NewRpcRequest(lib.MethodTypeGetResource, requestId, serializer.Bytes())

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
			"Content-Type": lib.ContentTypeMilonJson,
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

	// 6. 反序列化 postcard 格式的响应体为 ChainHead 结构
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

	// 7. 返回 ViewMulti 查询结果（BodySimulateReceipt 由调用方后续解码）
	return &GetResourceResult{
		HttpRspBytes:    httpResponseBytes,
		HttpRspBody:     apiResponse.Body,
		BodyGetResource: getResource,
	}, nil
}

func (c *rpcClientV1) GetChainHead(requestId lib.RequestID) (*ChainHeadResult, error) {
	// 1. 创建 RPC 请求对象（MethodTypeChainHead，空 body 因为不需要参数）
	submitTransaction := lib.NewRpcRequest(lib.MethodTypeChainHead, requestId, []byte{})

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
			"Content-Type": lib.ContentTypeMilonJson,
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
		HttpRspBytes:  httpResponseBytes,
		HttpRspBody:   apiResponse.Body,
		BodyChainHead: chainHead,
	}, nil
}

// BuildAndViewSingleIx 构建并执行 View 方法调用（只读查询，不修改链上状态）
func (c *rpcClientV1) BuildAndViewSingleIx(idlAppName string, methodName string, args provider.Args, requestId lib.RequestID) (*ViewSingleTransactionResult, error) {
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

	// 4. 调用 ViewMulti RPC 方法（直接传入 wires 的序列化数据）
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

func (c *rpcClientV1) BuildAndViewMultiIx(wires []api.PackedInstruction, requestId lib.RequestID) (*ViewMultiTransactionResult, error) {
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

func (c *rpcClientV1) GetTxByHash(txHash any, requestId lib.RequestID) (*GetTxByHashResult, error) {
	// 1. 将 txHashRelaxed 解码并序列化为 postcard 格式
	hash, err := api.NewTxHashFromRelaxed(txHash)
	if err != nil {
		return nil, fmt.Errorf("failed to decode txHashRelaxed: %w", err)
	}

	serializer := postcard.NewSerializer()
	if err = serializer.SerializeBytes(hash[:]); err != nil {
		return nil, fmt.Errorf("failed to serialize txHash: %w", err)
	}

	// 2. 创建 RPC 请求对象（MethodTypeGetTxByHash）
	submitTransaction := lib.NewRpcRequest(lib.MethodTypeGetTxByHash, requestId, serializer.Bytes())

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
			"Content-Type": lib.ContentTypeMilonJson,
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
		return nil, fmt.Errorf("API returned error status : %+v", *apiResponse.Error)
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
		HttpRspBytes:  httpResponseBytes,
		HttpRspBody:   apiResponse.Body,
		BodyTxHistory: txHistory,
	}, nil
}

func (c *rpcClientV1) GetAccount(addressBase58 string, requestId lib.RequestID) (*GetAccountResult, error) {
	// 1. 将 Base58 编码的 txHash 解码并序列化为 postcard 格式
	serializer := postcard.NewSerializer()
	serializer.SerializeFixedBytes(base58.Decode(addressBase58))

	// 2. 创建 RPC 请求对象（MethodTypeGetTxByHash）
	submitTransaction := lib.NewRpcRequest(lib.MethodTypeGetAccount, requestId, serializer.Bytes())

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
			"Content-Type": lib.ContentTypeMilonJson,
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
		return nil, fmt.Errorf("failed to deserialize TxHistory: %w", err)
	}

	// 6. 返回交易查询结果（HTTP 状态码、响应字节、解析后的 AccountView）
	return &GetAccountResult{
		HttpRspBytes:    httpResponseBytes,
		HttpRspBody:     apiResponse.Body,
		BodyAccountView: accountView,
	}, nil
}

func (c *rpcClientV1) GetBlockByHeight(blockHeight uint64, requestId lib.RequestID) (*GetBlockByHeightResult, error) {
	// 1. 将 blockHeight 并序列化为 postcard 格式
	serializer := postcard.NewSerializer()
	err := serializer.SerializeU64(blockHeight)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize blockHeight: %w", err)
	}

	// 2. 创建 RPC 请求对象（MethodTypeGetTxByHash）
	submitTransaction := lib.NewRpcRequest(lib.MethodTypeGetBlockByHeight, requestId, serializer.Bytes())

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
			"Content-Type": lib.ContentTypeMilonJson,
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
		return nil, fmt.Errorf("failed to deserialize TxHistory: %w", err)
	}

	// 6. 返回交易查询结果（HTTP 状态码、响应字节、解析后的 BodyBlock）
	return &GetBlockByHeightResult{
		HttpRspBytes: httpResponseBytes,
		HttpRspBody:  apiResponse.Body,
		BodyBlock:    block,
	}, nil
}

func (c *rpcClientV1) EventsByTxHash(txHashRelaxed any, typeTagFilter *uint64, requestId lib.RequestID) (*EventsByTxHashResult, error) {
	txHash, err := api.NewTxHashFromRelaxed(txHashRelaxed)
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
	submitTransaction := lib.NewRpcRequest(lib.MethodTypeEventsByTxHash, requestId, serializer.Bytes())

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
			"Content-Type": lib.ContentTypeMilonJson,
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

	// 6. 返回交易查询结果（HTTP 状态码、响应字节、解析后的 EventsByTxHash）
	return &EventsByTxHashResult{
		HttpRspBytes:       httpResponseBytes,
		HttpRspBody:        apiResponse.Body,
		BodyEventsByTxHash: eventsByTxHashResponse,
	}, nil
}

func (c *rpcClientV1) ListResourcePath(requestId lib.RequestID) (*ListResourcePathResult, error) {
	// 1. 创建 RPC 请求对象（MethodTypeListResourcePath）
	submitTransaction := lib.NewRpcRequest(lib.MethodTypeListResourcePath, requestId, []byte{})

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
			"Content-Type": lib.ContentTypeMilonJson,
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

	// 5. 返回交易查询结果（HTTP 状态码、响应字节、解析后的 []api.ListResourcePathInfo）
	return &ListResourcePathResult{
		HttpRspBytes:          httpResponseBytes,
		HttpRspBody:           apiResponse.Body,
		BodyListResourcePaths: listResourcePaths,
	}, nil
}

func (c *rpcClientV1) GetResourcePathByHash(rsHash api.RsHash, requestId lib.RequestID) (*GetResourcePathByHashResult, error) {
	// 1. 创建 RPC 请求对象（MethodTypeListResourcePath）
	submitTransaction := lib.NewRpcRequest(lib.MethodTypeGetResourcePathByHash, requestId, rsHash[:])

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
			"Content-Type": lib.ContentTypeMilonJson,
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

	// 6. 返回交易查询结果（HTTP 状态码、响应字节）
	return &GetResourcePathByHashResult{
		HttpRspBytes: httpResponseBytes,
		HttpRspBody:  apiResponse.Body,
	}, nil
}

func (c *rpcClientV1) GetAccessValue(blobHashList []api.BlobHash, requestId lib.RequestID) (*GetAccessValueResult, error) {
	// 1. 序列化 blobHashList 为 postcard 格式
	serializer := postcard.NewSerializer()
	if err := postcard.SerializeSeq(serializer, blobHashList, func(s *postcard.Serializer, bh api.BlobHash) error {
		s.SerializeFixedBytes(bh[:])
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to serialize blobHashList: %w", err)
	}

	// 2. 创建 RPC 请求对象（MethodTypeGetAccessValue）
	submitTransaction := lib.NewRpcRequest(lib.MethodTypeGetAccessValue, requestId, serializer.Bytes())

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
			"Content-Type": lib.ContentTypeMilonJson,
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

	// 4.反序列化 postcard 格式的响应体为 []GetAccessValueInfo 结构
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

	// 5. 返回交易查询结果（HTTP 状态码、响应字节）
	return &GetAccessValueResult{
		HttpRspBytes:        httpResponseBytes,
		HttpRspBody:         apiResponse.Body,
		BodyGetAccessValues: accessValues,
	}, nil
}

func (c *rpcClientV1) WaitForTransaction(txHash any, requestId lib.RequestID, options ...any) (*GetTxByHashResult, error) {
	// 设置默认轮询参数
	pollPeriod := c.pollPeriod
	pollTimeout := c.pollTimeout

	// 解析可选参数（单次调用可覆盖）
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
	for {
		// 检查是否超时
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("WaitForTransaction timeout after %v", pollTimeout)
		}

		// 等待轮询间隔
		time.Sleep(pollPeriod)

		// 查询交易状态
		result, err := c.GetTxByHash(txHash, requestId)
		if err != nil {
			//log.Printf("WaitForTransaction GetTxByHash err = %+v \n\n", err)
			// 如果查询失败，继续轮询（可能是交易还未传播到节点）
			continue
		}

		// 交易仍在 Pending 状态，继续轮询
		if result.BodyTxHistory.Receipt.State == api.TxStatePending {
			log.Printf("WaitForTransaction TxHistory.Receipt.State = %v \n\n", result.BodyTxHistory.Receipt.State)
			continue
		}

		return result, nil
	}
}

//func (c *rpcClientV1) SimulateTx(transactionPostcard []byte, requestId lib.RequestID) (*SimulateTransactionResult, error) {
//	// 1. 创建 RPC 请求对象（MethodTypeSimulateTx，包含已序列化的交易数据）
//	submitTransaction := lib.NewRpcRequest(lib.MethodTypeSimulateTx, requestId, transactionPostcard)
//
//	// 2. 将请求序列化为 postcard 格式并发送 HTTP POST
//	submitTransactionPostcard, err := postcard.SerializePostcard(submitTransaction)
//	if err != nil {
//		return nil, fmt.Errorf("failed to serialize submit transaction: %w", err)
//	}
//	httpStatusCode, httpResponseBytes, err := tools.HttpPostByBytes(
//		context.Background(),
//		c.network.RpcUrl,
//		submitTransactionPostcard,
//		map[string]string{
//			"Content-Type": lib.ContentTypeMilonPostcard,
//		},
//	)
//	if err != nil {
//		return nil, fmt.Errorf("failed to submit transaction: %w", err)
//	}
//	if httpStatusCode != http.StatusOK {
//		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
//	}
//
//	// 3. 反序列化 postcard 格式的 HTTP 响应为 API RpcResponse 结构
//	httpResponse, err := postcard.DeserializePostcard(httpResponseBytes, func(d *postcard.Deserializer) (*api.RpcResponse, error) {
//		var rsp api.RpcResponse
//		if err = rsp.UnmarshalPostcard(d); err != nil {
//			return nil, err
//		}
//		return &rsp, nil
//	}, false)
//	if err != nil {
//		return nil, fmt.Errorf("failed to deserialize API response: %w", err)
//	}
//
//	// 4. 验证 API 响应状态码（必须为 RpcResponseStatusOk）
//	if httpResponse.Status != api.RpcResponseStatusOk {
//		return nil, fmt.Errorf("API returned error status: %+v", httpResponse.Error)
//	}
//
//	// 5. 反序列化 postcard 格式的响应体为 SimulateReceipt 结构
//	simulateReceipt, err := postcard.DeserializePostcard(httpResponse.Body, func(d *postcard.Deserializer) (*api.SimulateReceipt, error) {
//		var rsp api.SimulateReceipt
//		if err = rsp.UnmarshalPostcard(d); err != nil {
//			return nil, err
//		}
//		return &rsp, nil
//	}, false)
//	if err != nil {
//		return nil, fmt.Errorf("反序列化 SimulateReceipt 失败: %w", err)
//	}
//
//	// 6. 返回提交结果
//	return &SimulateTransactionResult{
//		HttpRspBytes:        httpResponseBytes,
//		HttpRspBody:         httpResponse.Body,
//		BodySimulateReceipt: simulateReceipt,
//	}, nil
//}
//func (c *rpcClientV1) SubmitTx(transactionPostcard []byte, requestId lib.RequestID) (*SubmitTransactionResult, error) {
//	// 1. 创建 RPC 请求对象（MethodTypeSubmitTx，包含已序列化的交易数据）
//	submitTransaction := lib.NewRpcRequest(lib.MethodTypeSubmitTx, requestId, transactionPostcard)
//
//	// 2. 将请求序列化为 postcard 格式并发送 HTTP POST
//	submitTransactionPostcard, err := postcard.SerializePostcard(submitTransaction)
//	if err != nil {
//		return nil, fmt.Errorf("failed to serialize submit transaction: %w", err)
//	}
//	httpStatusCode, httpResponseBytes, err := tools.HttpPostByBytes(
//		context.Background(),
//		c.network.RpcUrl,
//		submitTransactionPostcard,
//		map[string]string{
//			"Content-Type": lib.ContentTypeMilonPostcard,
//		},
//	)
//	if err != nil {
//		return nil, fmt.Errorf("failed to submit transaction: %w", err)
//	}
//	if httpStatusCode != http.StatusOK {
//		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
//	}
//
//	// 3. 反序列化 postcard 格式的 HTTP 响应为 API RpcResponse 结构
//	httpResponse, err := postcard.DeserializePostcard(httpResponseBytes, func(d *postcard.Deserializer) (*api.RpcResponse, error) {
//		var rsp api.RpcResponse
//		if err = rsp.UnmarshalPostcard(d); err != nil {
//			return nil, err
//		}
//		return &rsp, nil
//	}, false)
//	if err != nil {
//		return nil, fmt.Errorf("failed to deserialize API response: %w", err)
//	}
//
//	// 4. 验证 API 响应状态码（必须为 RpcResponseStatusOk）
//	if httpResponse.Status != api.RpcResponseStatusOk {
//		return nil, fmt.Errorf("API returned error status: %+v", httpResponse.Error)
//	}
//
//	// 5. 返回提交结果
//	return &SubmitTransactionResult{
//		HttpRspBytes: httpResponseBytes,
//		HttpRspBody:  httpResponse.Body,
//	}, nil
//}
