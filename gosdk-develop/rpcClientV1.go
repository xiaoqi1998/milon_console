package milon

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/gen"
	"github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/postcard"
	"github.com/milon-labs/milon-go-sdk/provider"
	"github.com/milon-labs/milon-go-sdk/tools"
	"github.com/milon-labs/milon-go-sdk/types"
	"net/http"
	"time"
)

type ChainHeadResult struct {
	HTTPResponseBody []byte
	BodyChainHead    *api.ChainHead
}

type SimulateTxResult struct {
	HTTPResponseBody    []byte
	BodySimulateReceipt *api.SimulateReceipt
}

type ViewResult struct {
	HTTPResponseBody []byte
}

type GetResourceResult struct {
	HTTPResponseBody []byte
	BodyGetResource  *api.GetResource
}

type GetBlockByHeightResult struct {
	HTTPResponseBody []byte
	BodyBlock        *api.Block
}

type GetTxByHashResult struct {
	HTTPResponseBody []byte
	BodyTxHistory    *api.TxHistory
}

type GetAccountResult struct {
	HTTPResponseBody []byte
	BodyAccountView  *api.AccountView
}

type EventsByTxHashResult struct {
	HTTPResponseBody   []byte
	BodyEventsByTxHash *api.EventsByTxHash
}

type ListResourcePathResult struct {
	HTTPResponseBody      []byte
	BodyListResourcePaths []*api.ListResourcePathInfo
}

type GetResourcePathByHashResult struct {
	HTTPResponseBody []byte
	Path             string
}

type GetAccessValueResult struct {
	HTTPResponseBody    []byte
	BodyGetAccessValues []*api.GetAccessValueInfo
}

type BatchGetResourcePathByHashResult struct {
	HTTPResponseBody          []byte
	BodyBatchResourcePathList []*api.BatchGetResourcePathInfo
}

type rpcClientV1 struct {
	network           Network
	providerByIDLName map[string]*provider.Provider
	providerManager   *provider.IDLRegistry
	pollPeriod        time.Duration
	pollTimeout       time.Duration
}

// parseRequestID parses the RequestID option from options; defaults to the current timestamp.
func parseRequestID(options []any) (lib.RequestID, error) {
	requestID := lib.RequestID(time.Now().UnixMilli())
	for _, opt := range options {
		switch v := opt.(type) {
		case lib.RequestID:
			requestID = v
		default:
			return 0, fmt.Errorf("unknown option type: %T", opt)
		}
	}
	return requestID, nil
}

// callJsonRPC sends an RPC request in JSON format and returns the parsed response.
func (c *rpcClientV1) callJsonRPC(method lib.MethodType, body []byte, requestID lib.RequestID) (*lib.RpcResponse, error) {
	rpcReq := lib.NewRpcRequest(method, requestID, body)

	// The Body is transmitted as a JSON integer array.
	bodyData := make([]int, len(rpcReq.Body))
	for i, value := range rpcReq.Body {
		bodyData[i] = int(value)
	}

	httpStatusCode, httpResponseBytes, err := tools.HttpPostByJson(
		context.Background(),
		c.network.RpcUrl,
		map[string]any{
			"method":     rpcReq.Method,
			"request_id": rpcReq.RequestId,
			"body":       bodyData,
		},
		map[string]string{
			"Content-Type": lib.ContentTypeMilonJson,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("RPC call failed: %w", err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
	}

	apiResponse := &lib.RpcResponse{}
	if err = json.Unmarshal(httpResponseBytes, apiResponse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API response: %w", err)
	}
	if apiResponse.Status != lib.RpcResponseStatusOk {
		if apiResponse.Error != nil {
			return nil, fmt.Errorf("API returned error status %d: %+v", apiResponse.Status, *apiResponse.Error)
		}
		return nil, fmt.Errorf("API returned error status: %d", apiResponse.Status)
	}
	return apiResponse, nil
}

// callPostcardRPC sends an RPC request in postcard format and returns the parsed response.
func (c *rpcClientV1) callPostcardRPC(method lib.MethodType, body []byte, requestID lib.RequestID) (*lib.RpcResponse, error) {
	rpcReq := lib.NewRpcRequest(method, requestID, body)
	rpcReqPostcard, err := postcard.SerializePostcard(rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize RPC request: %w", err)
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
		return nil, fmt.Errorf("RPC call failed: %w", err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned error statusCode: %d", httpStatusCode)
	}

	httpResponse, err := decodePostcardBody[lib.RpcResponse](httpResponseBytes, "API response")
	if err != nil {
		return nil, err
	}
	if httpResponse.Status != lib.RpcResponseStatusOk {
		if httpResponse.Error != nil {
			return nil, fmt.Errorf("API returned error status %d: %+v", httpResponse.Status, *httpResponse.Error)
		}
		return nil, fmt.Errorf("API returned error status: %d", httpResponse.Status)
	}
	return httpResponse, nil
}

// decodePostcardBody decodes a postcard-encoded response body into T.
// T must implement postcard.Unmarshaler.
func decodePostcardBody[T any](body []byte, name string) (*T, error) {
	var value T
	decoded, err := postcard.DeserializePostcard(body, func(d *postcard.Deserializer) (*T, error) {
		if u, ok := any(&value).(postcard.Unmarshaler); ok {
			if err := u.UnmarshalPostcard(d); err != nil {
				return nil, err
			}
			return &value, nil
		}
		return nil, fmt.Errorf("%T does not implement postcard.Unmarshaler", &value)
	}, false)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize %s: %w", name, err)
	}
	return decoded, nil
}

// LoadIDLsFromData loads a list of IDLs from data and registers a Provider for each IDL name.
func (c *rpcClientV1) LoadIDLsFromData(idls []provider.IDL) error {
	if len(idls) == 0 {
		return fmt.Errorf("empty IDL data")
	}
	for _, idl := range idls {
		if idl.Metadata.Name == "" {
			return fmt.Errorf("IDL metadata name is empty")
		}
		c.providerByIDLName[idl.Metadata.Name] = provider.NewProvider(idl)
	}
	return nil
}

// GetAllPd returns all loaded Providers, indexed by IDL name.
func (c *rpcClientV1) GetAllPd() map[string]*provider.Provider {
	return c.providerByIDLName
}

// GetProviderManager returns the IDL registry.
func (c *rpcClientV1) GetProviderManager() *provider.IDLRegistry {
	return c.providerManager
}

func (c *rpcClientV1) ClaimFaucet(accountSk crypto.SecretKeyer, account *crypto.Address, mode lib.AccountSignatureMode) error {
	// 1. Encode instruction
	wire, err := gen.Token.ClaimFaucet.Args(account).Encode()
	if err != nil {
		return fmt.Errorf("failed to encode instruction: %w", err)
	}

	// 2. SplitPayerSelfPay mode: no payer; each executor signs its own ix bit(s) and gas bit (bit63).
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		AddIxAndPayerSig(*account, accountSk, 0, mode).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build split transaction: %w", err)
	}

	// 3. Submit the transaction to the chain
	err = c.SubmitTx(tx)
	if err != nil {
		return fmt.Errorf("failed to Submit transaction on chain: %w", err)
	}

	// 4. Wait for the transaction to complete
	_, err = c.WaitForTransaction(tx.TxHash())
	if err != nil {
		return fmt.Errorf("failed to wait for transaction: %w", err)
	}

	return nil
}

func (c *rpcClientV1) CreateAccount(accountSk crypto.SecretKeyer, pk *crypto.PublicKey) error {
	// 1. Encode instruction
	wire, err := gen.Account.Create.Args(pk).Encode()
	if err != nil {
		return fmt.Errorf("failed to encode instruction: %w", err)
	}

	// 2. Get account from public key
	account, err := crypto.NewAddressFromPublicKey(pk)
	if err != nil {
		return fmt.Errorf("failed to get account from public key: %w", err)
	}

	// 3. UnifiedPayerGasOnly mode: payer signs gas (bit63) only, ix needs no signature (pure sponsorship).
	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).WithPayer(account).
		AddPayerSig(*account, accountSk, lib.PubKeySignatureMode{PublicKey: *pk}).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build transaction: %w", err)
	}

	// 4. Submit the transaction to the chain
	err = c.SubmitTx(tx)
	if err != nil {
		return fmt.Errorf("failed to Submit transaction on chain: %w", err)
	}

	// 5. Wait for the transaction to complete
	_, err = c.WaitForTransaction(tx.TxHash())
	if err != nil {
		return fmt.Errorf("failed to wait for transaction: %w", err)
	}

	return nil
}

func (c *rpcClientV1) BalanceOf(account *crypto.Address) (uint64, error) {
	wire, err := gen.Token.BalanceOf.Args(api.MILToken, account).Encode()
	if err != nil {
		return 0, fmt.Errorf("failed to encode BalanceOf instruction: %w", err)
	}

	viewTxResult, err := c.View([]api.PackedInstruction{wire})
	if err != nil {
		return 0, fmt.Errorf("failed to view BalanceOf: %w", err)
	}
	return gen.Token.BalanceOf.DecodeView(viewTxResult.HTTPResponseBody)
}

func (c *rpcClientV1) ListAccountSigners(account *crypto.Address) ([]any, error) {
	wire, err := gen.Account.ListSigners.Args(account).Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode ListSigners view: %w", err)
	}
	viewResult, err := c.View([]api.PackedInstruction{wire})
	if err != nil {
		return nil, fmt.Errorf("failed to call ListSigners view: %w", err)
	}
	out, err := gen.Account.ListSigners.DecodeView(viewResult.HTTPResponseBody)
	if err != nil {
		return nil, fmt.Errorf("failed to decode ListSigners view: %w", err)
	}
	if len(out) < 2 {
		return nil, fmt.Errorf("unexpected ListSigners result: %v", out)
	}

	return out, nil
}
func (c *rpcClientV1) AccountSignerBit(account *crypto.Address) (types.Bitmap64, error) {
	listSigners, err := c.ListAccountSigners(account)
	if err != nil {
		return types.NewBitmap64(0), fmt.Errorf("failed to get account list signers: %w", err)
	}

	accountMap, ok := listSigners[0].(map[string]any)
	if !ok {
		return types.NewBitmap64(0), fmt.Errorf("unexpected account data: %v", listSigners[0])
	}
	signers, ok := listSigners[1].([]any)
	if !ok {
		return types.NewBitmap64(0), fmt.Errorf("unexpected signers list: %v", listSigners[1])
	}
	if len(signers) != 1 {
		return types.NewBitmap64(0), fmt.Errorf("account %v has %d signers; AccountSignerBit supports single-signer accounts only", account, len(signers))
	}

	bm, ok := accountMap["bitmap"].(uint64)
	if !ok || bm == 0 {
		return types.NewBitmap64(0), fmt.Errorf("unexpected account bitmap: %v", accountMap["bitmap"])
	}
	// 单签名账户的 bitmap 中恰好只有一个置位。
	lowest := bm & -bm
	if bm != lowest {
		return types.NewBitmap64(0), fmt.Errorf("account %v bitmap %#x has multiple signer slots; use multisig signing instead", account, bm)
	}
	return types.NewBitmap64(lowest), nil
}

func (c *rpcClientV1) GetChainHead(options ...any) (*ChainHeadResult, error) {
	// 1. Parse the optional arguments
	requestID, err := parseRequestID(options)
	if err != nil {
		return nil, err
	}

	// 2. Create and send the RPC request (MethodTypeChainHead; empty body since no arguments are needed)
	apiResponse, err := c.callJsonRPC(lib.MethodTypeChainHead, []byte{}, requestID)
	if err != nil {
		return nil, err
	}

	// 3. Deserialize the postcard-encoded response body into a ChainHead struct
	chainHead, err := decodePostcardBody[api.ChainHead](apiResponse.Body, "ChainHead")
	if err != nil {
		return nil, err
	}

	return &ChainHeadResult{
		HTTPResponseBody: apiResponse.Body,
		BodyChainHead:    chainHead,
	}, nil
}

func (c *rpcClientV1) SubmitTx(tx *lib.Transaction, options ...any) error {
	// 1. Validate the transaction structure
	err := tx.ValidateWire()
	if err != nil {
		return fmt.Errorf("transaction validation failed: %w", err)
	}

	// 2. Serialize the transaction
	txPostcard, err := tx.ToBytes()
	if err != nil {
		return fmt.Errorf("failed to serialize transaction: %w", err)
	}

	// 3. Parse the optional arguments
	requestID, err := parseRequestID(options)
	if err != nil {
		return err
	}

	// 4. Send the RPC request (MethodTypeSubmitTx, containing the serialized transaction data)
	_, err = c.callPostcardRPC(lib.MethodTypeSubmitTx, txPostcard, requestID)
	return err
}

func (c *rpcClientV1) SimulateTx(transaction *lib.Transaction, options ...any) (*SimulateTxResult, error) {
	// 1. Serialize the transaction
	txPostcard, err := transaction.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	// 2. Parse the optional arguments
	requestID, err := parseRequestID(options)
	if err != nil {
		return nil, err
	}

	// 3. Send the RPC request (MethodTypeSimulateTx, containing the serialized transaction data)
	httpResponse, err := c.callPostcardRPC(lib.MethodTypeSimulateTx, txPostcard, requestID)
	if err != nil {
		return nil, err
	}

	// 4. Deserialize the postcard-encoded response body into a SimulateReceipt struct
	simulateReceipt, err := decodePostcardBody[api.SimulateReceipt](httpResponse.Body, "SimulateReceipt")
	if err != nil {
		return nil, err
	}

	return &SimulateTxResult{
		HTTPResponseBody:    httpResponse.Body,
		BodySimulateReceipt: simulateReceipt,
	}, nil
}

func (c *rpcClientV1) View(wires []api.PackedInstruction, options ...any) (*ViewResult, error) {
	// 1. Serialize the wires in postcard format
	serializer := postcard.NewSerializer()
	if err := serializer.SerializeU32(uint32(len(wires))); err != nil {
		return nil, fmt.Errorf("failed to serialize wires length: %w", err)
	}
	for _, w := range wires {
		if err := serializer.SerializeBytes(w); err != nil {
			return nil, fmt.Errorf("failed to serialize wire: %w", err)
		}
	}

	// 2. Parse the optional arguments
	requestID, err := parseRequestID(options)
	if err != nil {
		return nil, err
	}

	// 3. Send the RPC request (MethodTypeView, containing the serialized wires data)
	apiResponse, err := c.callJsonRPC(lib.MethodTypeView, serializer.Bytes(), requestID)
	if err != nil {
		return nil, err
	}

	return &ViewResult{
		HTTPResponseBody: apiResponse.Body,
	}, nil
}

func (c *rpcClientV1) GetResource(rsHash api.RsHash, options ...any) (*GetResourceResult, error) {
	// 1. Parse the optional arguments
	requestID, err := parseRequestID(options)
	if err != nil {
		return nil, err
	}

	// 2. Encode rsHash and serialize it in postcard format
	serializer := postcard.NewSerializer()
	serializer.SerializeFixedBytes(rsHash[:])

	// 3. Send the RPC request (MethodTypeGetResource)
	apiResponse, err := c.callJsonRPC(lib.MethodTypeGetResource, serializer.Bytes(), requestID)
	if err != nil {
		return nil, err
	}

	// 4. Deserialize the postcard-encoded response body into a GetResource struct
	getResource, err := decodePostcardBody[api.GetResource](apiResponse.Body, "GetResource")
	if err != nil {
		return nil, err
	}

	return &GetResourceResult{
		HTTPResponseBody: apiResponse.Body,
		BodyGetResource:  getResource,
	}, nil
}

func (c *rpcClientV1) GetBlockByHeight(blockHeight uint64, options ...any) (*GetBlockByHeightResult, error) {
	// 1. Parse the optional arguments
	requestID, err := parseRequestID(options)
	if err != nil {
		return nil, err
	}

	// 2. Serialize blockHeight as postcard format
	serializer := postcard.NewSerializer()
	if err := serializer.SerializeU64(blockHeight); err != nil {
		return nil, fmt.Errorf("failed to serialize blockHeight: %w", err)
	}

	// 3. Send the RPC request (MethodTypeGetBlockByHeight)
	apiResponse, err := c.callJsonRPC(lib.MethodTypeGetBlockByHeight, serializer.Bytes(), requestID)
	if err != nil {
		return nil, err
	}

	// 4. Deserialize the postcard-encoded response body into a Block struct
	block, err := decodePostcardBody[api.Block](apiResponse.Body, "Block")
	if err != nil {
		return nil, err
	}

	return &GetBlockByHeightResult{
		HTTPResponseBody: apiResponse.Body,
		BodyBlock:        block,
	}, nil
}

func (c *rpcClientV1) GetTxByHash(txHashRelaxed any, options ...any) (*GetTxByHashResult, error) {
	// 1. Parse the optional arguments
	requestID, err := parseRequestID(options)
	if err != nil {
		return nil, err
	}

	// 2. Decode txHashRelaxed and serialize it in postcard format
	txHash, err := api.NewTxHashFromRelaxed(txHashRelaxed)
	if err != nil {
		return nil, fmt.Errorf("failed to decode txHashRelaxed: %w", err)
	}
	serializer := postcard.NewSerializer()
	if err = serializer.SerializeBytes(txHash[:]); err != nil {
		return nil, fmt.Errorf("failed to serialize txHashRelaxed: %w", err)
	}

	// 3. Send the RPC request (MethodTypeGetTxByHash)
	apiResponse, err := c.callJsonRPC(lib.MethodTypeGetTxByHash, serializer.Bytes(), requestID)
	if err != nil {
		return nil, err
	}

	// 4. Deserialize the postcard-encoded response body into a TxHistory struct
	txHistory, err := decodePostcardBody[api.TxHistory](apiResponse.Body, "TxHistory")
	if err != nil {
		return nil, err
	}

	return &GetTxByHashResult{
		HTTPResponseBody: apiResponse.Body,
		BodyTxHistory:    txHistory,
	}, nil
}

func (c *rpcClientV1) GetAccount(accountRelaxed any, options ...any) (*GetAccountResult, error) {
	// 1. Parse the optional arguments
	requestID, err := parseRequestID(options)
	if err != nil {
		return nil, err
	}

	// 2. Decode accountRelaxed and serialize it in postcard format
	account, err := crypto.NewAddressFromRelaxed(accountRelaxed)
	if err != nil {
		return nil, fmt.Errorf("failed to decode accountRelaxed: %w", err)
	}
	serializer := postcard.NewSerializer()
	if err = account.MarshalPostcard(serializer); err != nil {
		return nil, fmt.Errorf("failed to serialize accountRelaxed: %w", err)
	}

	// 3. Send the RPC request (MethodTypeGetAccount)
	apiResponse, err := c.callJsonRPC(lib.MethodTypeGetAccount, serializer.Bytes(), requestID)
	if err != nil {
		return nil, err
	}

	// 4. Deserialize the postcard-encoded response body into an AccountView struct
	accountView, err := decodePostcardBody[api.AccountView](apiResponse.Body, "AccountView")
	if err != nil {
		return nil, err
	}

	return &GetAccountResult{
		HTTPResponseBody: apiResponse.Body,
		BodyAccountView:  accountView,
	}, nil
}

func (c *rpcClientV1) EventsByTxHash(txHashRelaxed any, typeTagFilter *uint64, options ...any) (*EventsByTxHashResult, error) {
	// 1. Parse the optional arguments
	requestID, err := parseRequestID(options)
	if err != nil {
		return nil, err
	}

	// 2. Serialize the request body in postcard format
	txHash, err := api.NewTxHashFromRelaxed(txHashRelaxed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse txHash: %w", err)
	}
	serializer := postcard.NewSerializer()
	eventsByTxHashRequest := api.EventsByTxHashReq{
		TxHash:        txHash,
		TypeTagFilter: typeTagFilter,
	}
	if err = eventsByTxHashRequest.MarshalPostcard(serializer); err != nil {
		return nil, fmt.Errorf("failed to serialize EventsByTxHashReq: %w", err)
	}

	// 3. Send the RPC request (MethodTypeEventsByTxHash)
	apiResponse, err := c.callJsonRPC(lib.MethodTypeEventsByTxHash, serializer.Bytes(), requestID)
	if err != nil {
		return nil, err
	}

	// 4. Deserialize the postcard-encoded response body into an EventsByTxHash struct
	eventsByTxHashResponse, err := decodePostcardBody[api.EventsByTxHash](apiResponse.Body, "EventsByTxHash")
	if err != nil {
		return nil, err
	}

	return &EventsByTxHashResult{
		HTTPResponseBody:   apiResponse.Body,
		BodyEventsByTxHash: eventsByTxHashResponse,
	}, nil
}

func (c *rpcClientV1) ListResourcePath(options ...any) (*ListResourcePathResult, error) {
	// 1. Parse the optional arguments
	requestID, err := parseRequestID(options)
	if err != nil {
		return nil, err
	}

	// 2. Send the RPC request (MethodTypeListResourcePath)
	apiResponse, err := c.callJsonRPC(lib.MethodTypeListResourcePath, []byte{}, requestID)
	if err != nil {
		return nil, err
	}

	// 3. Deserialize the JSON-encoded response body into []*api.ListResourcePathInfo struct
	var rawList [][]any
	if err = json.Unmarshal(apiResponse.Body, &rawList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ListResourcePathInfo response: %w", err)
	}
	listResourcePaths, err := api.UnmarshalListResourcePathListFromRawList(rawList)
	if err != nil {
		return nil, fmt.Errorf("failed to parse ListResourcePathInfo: %w", err)
	}

	return &ListResourcePathResult{
		HTTPResponseBody:      apiResponse.Body,
		BodyListResourcePaths: listResourcePaths,
	}, nil
}

func (c *rpcClientV1) GetResourcePathByHash(rsHash api.RsHash, options ...any) (*GetResourcePathByHashResult, error) {
	// 1. Parse the optional arguments
	requestID, err := parseRequestID(options)
	if err != nil {
		return nil, err
	}

	// 2. Send the RPC request (MethodTypeGetResourcePathByHash)
	apiResponse, err := c.callJsonRPC(lib.MethodTypeGetResourcePathByHash, rsHash[:], requestID)
	if err != nil {
		return nil, err
	}

	// 3. Deserialize the JSON-encoded response body into string struct
	var path string
	if err = json.Unmarshal(apiResponse.Body, &path); err != nil {
		return nil, fmt.Errorf("failed to unmarshal GetResourcePathByHash body: %w", err)
	}

	return &GetResourcePathByHashResult{
		HTTPResponseBody: apiResponse.Body,
		Path:             path,
	}, nil
}

func (c *rpcClientV1) GetAccessValue(blobHashList []api.BlobHash, options ...any) (*GetAccessValueResult, error) {
	// 1. Parse the optional arguments
	requestID, err := parseRequestID(options)
	if err != nil {
		return nil, err
	}

	// 2. Serialize blobHashList as postcard format
	serializer := postcard.NewSerializer()
	if err := postcard.SerializeSeq(serializer, blobHashList, func(s *postcard.Serializer, bh api.BlobHash) error {
		s.SerializeFixedBytes(bh[:])
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to serialize blobHashList: %w", err)
	}

	// 3. Send the RPC request (MethodTypeGetAccessValue)
	apiResponse, err := c.callJsonRPC(lib.MethodTypeGetAccessValue, serializer.Bytes(), requestID)
	if err != nil {
		return nil, err
	}

	// 4. Deserialize the postcard-encoded response body into []GetAccessValueInfo struct
	deserializer := postcard.NewDeserializer(apiResponse.Body)
	accessValues, err := postcard.DeserializeSeq(deserializer, func(d *postcard.Deserializer) (*api.GetAccessValueInfo, error) {
		var info api.GetAccessValueInfo
		if err := info.UnmarshalPostcard(d); err != nil {
			return nil, fmt.Errorf("failed to deserialize GetAccessValueInfo: %w", err)
		}
		return &info, nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize GetAccessValue sequence: %w", err)
	}

	return &GetAccessValueResult{
		HTTPResponseBody:    apiResponse.Body,
		BodyGetAccessValues: accessValues,
	}, nil
}

func (c *rpcClientV1) BatchGetResourcePathByHash(rsHashList []api.RsHash, options ...any) (*BatchGetResourcePathByHashResult, error) {
	// 1. Parse the optional arguments
	requestID, err := parseRequestID(options)
	if err != nil {
		return nil, err
	}

	// 2. Serialize rsHashList as postcard format
	serializer := postcard.NewSerializer()
	if err := postcard.SerializeSeq(serializer, rsHashList, func(s *postcard.Serializer, rsHash api.RsHash) error {
		s.SerializeFixedBytes(rsHash[:])
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to serialize rsHashList: %w", err)
	}

	// 3. Send the RPC request (MethodTypeBatchGetResourcePathByHash)
	apiResponse, err := c.callJsonRPC(lib.MethodTypeBatchGetResourcePathByHash, serializer.Bytes(), requestID)
	if err != nil {
		return nil, err
	}

	// 4. Deserialize the JSON-encoded response body into []*api.BatchGetResourcePathInfo struct
	var rawList [][]any
	if err = json.Unmarshal(apiResponse.Body, &rawList); err != nil {
		return nil, fmt.Errorf("failed to unmarshal BatchGetResourcePathByHash body: %w", err)
	}
	bodyBatchResourcePathList, err := api.UnmarshalBatchResourcePathListFromRawList(rawList)
	if err != nil {
		return nil, fmt.Errorf("failed to parse BatchGetResourcePathByHash: %w", err)
	}

	return &BatchGetResourcePathByHashResult{
		HTTPResponseBody:          apiResponse.Body,
		BodyBatchResourcePathList: bodyBatchResourcePathList,
	}, nil
}

func (c *rpcClientV1) WaitForTransaction(txHash any, options ...any) (*GetTxByHashResult, error) {
	// Parse the optional arguments
	pollPeriod := c.pollPeriod
	pollTimeout := c.pollTimeout
	requestID := lib.RequestID(time.Now().UnixMilli())

	for _, opt := range options {
		switch v := opt.(type) {
		case PollPeriod:
			pollPeriod = time.Duration(v)
		case PollTimeout:
			pollTimeout = time.Duration(v)
		case lib.RequestID:
			requestID = v
		default:
			return nil, fmt.Errorf("unknown option type: %T", opt)
		}
	}

	deadline := time.Now().Add(pollTimeout)

	// Poll until the transaction is completed or the timeout is reached: query first, then wait, to avoid waiting in vain for the first round
	for {
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("WaitForTransaction timeout after %v", pollTimeout)
		}

		result, err := c.GetTxByHash(txHash, requestID)
		if err == nil && result.BodyTxHistory.Receipt.State != api.TxStatePending {
			return result, nil
		}

		time.Sleep(pollPeriod)
	}
}
