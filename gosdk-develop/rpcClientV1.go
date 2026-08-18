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
	"strconv"
	"sync"
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

type GetTxHistoryProofResult struct {
	HTTPResponseBody      []byte
	BodyGetTxHistoryProof *api.GetTxHistoryProof
}

type rpcClientV1 struct {
	network           Network
	providerByIDLName map[string]*provider.Provider
	providerManager   *provider.IDLRegistry
	typeResolver      postcard.TypeResolver
	pollPeriod        time.Duration
	pollTimeout       time.Duration
}

// jsonRPCBufferPool reuses the scratch buffer used to build JSON RPC request
// envelopes, avoiding a fresh allocation (and repeated growth) per call.
var jsonRPCBufferPool = sync.Pool{
	New: func() any {
		buf := make([]byte, 0, 1024)
		return &buf
	},
}

// encodeJsonRPCRequest builds the request envelope:
// {"method":...,"request_id":...,"body":[...]}. The byte body is encoded as a
// decimal integer array, matching the wire format the node expects for the
// JSON content type. The returned slice borrows a pooled buffer; callers must
// return it via jsonRPCBufferPool.Put once the HTTP call completes.
func encodeJsonRPCRequest(method lib.MethodType, requestID lib.RequestID, body []byte) []byte {
	bufPtr := jsonRPCBufferPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]

	buf = append(buf, `{"method":`...)
	buf = strconv.AppendUint(buf, uint64(method), 10)
	buf = append(buf, `,"request_id":`...)
	buf = strconv.AppendUint(buf, uint64(requestID), 10)
	buf = append(buf, `,"body":[`...)
	for i, v := range body {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = strconv.AppendUint(buf, uint64(v), 10)
	}
	buf = append(buf, `]}`...)

	*bufPtr = buf
	return buf
}

// callJsonRPC sends an RPC request in JSON format and returns the parsed response.
func (c *rpcClientV1) callJsonRPC(ctx context.Context, method lib.MethodType, body []byte, requestID lib.RequestID) (*lib.RpcResponse, error) {
	payload := encodeJsonRPCRequest(method, requestID, body)
	httpStatusCode, httpResponseBytes, err := tools.HttpPostByBytes(
		ctx,
		c.network.RpcUrl,
		payload,
		map[string]string{
			"Content-Type": lib.ContentTypeMilonJson,
		},
	)
	jsonRPCBufferPool.Put(&payload)
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
	if apiResponse.RequestId != uint64(requestID) {
		return nil, fmt.Errorf("response request_id %d does not match request %d", apiResponse.RequestId, requestID)
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
func (c *rpcClientV1) callPostcardRPC(ctx context.Context, method lib.MethodType, body []byte, requestID lib.RequestID) (*lib.RpcResponse, error) {
	rpcReq := lib.NewRpcRequest(method, requestID, body)
	serializer := postcard.NewSerializerWithCap(len(body) + 32)
	if err := rpcReq.MarshalPostcard(serializer); err != nil {
		return nil, fmt.Errorf("failed to serialize RPC request: %w", err)
	}
	rpcReqPostcard := serializer.Bytes()

	httpStatusCode, httpResponseBytes, err := tools.HttpPostByBytes(
		ctx,
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

	httpResponse, err := decodePostcardBody[lib.RpcResponse](httpResponseBytes, "API response", c.typeResolver)
	if err != nil {
		return nil, err
	}
	if httpResponse.RequestId != uint64(requestID) {
		return nil, fmt.Errorf("response request_id %d does not match request %d", httpResponse.RequestId, requestID)
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
// T must implement postcard.Unmarshaler. The resolver is injected so
// type_tag-based values decode against the caller's loaded IDLs.
// Note: decoded values alias the input body (zero-copy); callers that
// hold results long-term and need the body freed should copy explicitly.
func decodePostcardBody[T any](body []byte, name string, resolver postcard.TypeResolver) (*T, error) {
	var value T
	decoded, err := postcard.DeserializePostcardWithResolver(body, func(d *postcard.Deserializer) (*T, error) {
		if u, ok := any(&value).(postcard.Unmarshaler); ok {
			if err := u.UnmarshalPostcard(d); err != nil {
				return nil, err
			}
			return &value, nil
		}
		return nil, fmt.Errorf("%T does not implement postcard.Unmarshaler", &value)
	}, false, resolver)
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

// GetAllPd returns a copy of all loaded Providers, indexed by IDL name.
func (c *rpcClientV1) GetAllPd() map[string]*provider.Provider {
	out := make(map[string]*provider.Provider, len(c.providerByIDLName))
	for name, pd := range c.providerByIDLName {
		out[name] = pd
	}
	return out
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
		AddIxesSig(*account, accountSk, []uint8{0}, false, mode).
		Build()
	if err != nil {
		return fmt.Errorf("failed to build split transaction: %w", err)
	}

	// 3. Submit the transaction to the chain
	err = c.SubmitTxWithSponsorIxes(tx, []uint8{0})
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
	// A single-signer account's bitmap has exactly one bit set.
	lowest := bm & -bm
	if bm != lowest {
		return types.NewBitmap64(0), fmt.Errorf("account %v bitmap %#x has multiple signer slots; use multisig signing instead", account, bm)
	}
	return types.NewBitmap64(lowest), nil
}

func (c *rpcClientV1) GetChainHead(opts ...RequestOption) (*ChainHeadResult, error) {
	o := applyRequestOptions(opts)

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeChainHead, []byte{}, o.requestID)
	if err != nil {
		return nil, err
	}

	chainHead, err := decodePostcardBody[api.ChainHead](apiResponse.Body, "ChainHead", c.typeResolver)
	if err != nil {
		return nil, err
	}

	return &ChainHeadResult{
		HTTPResponseBody: apiResponse.Body,
		BodyChainHead:    chainHead,
	}, nil
}
func (c *rpcClientV1) SubmitTx(tx *lib.Transaction, opts ...RequestOption) error {
	err := tx.ValidateWire()
	if err != nil {
		return fmt.Errorf("transaction validation failed: %w", err)
	}

	txPostcard, err := tx.ToBytes()
	if err != nil {
		return fmt.Errorf("failed to serialize transaction: %w", err)
	}

	o := applyRequestOptions(opts)

	_, err = c.callPostcardRPC(o.ctx, lib.MethodTypeSubmitTx, txPostcard, o.requestID)
	return err
}
func (c *rpcClientV1) SubmitTxWithSponsorIxes(tx *lib.Transaction, sponsorIxes []uint8, opts ...RequestOption) error {
	err := tx.ValidateWireWith(sponsorIxes)
	if err != nil {
		return fmt.Errorf("transaction validation failed: %w", err)
	}

	txPostcard, err := tx.ToBytes()
	if err != nil {
		return fmt.Errorf("failed to serialize transaction: %w", err)
	}

	o := applyRequestOptions(opts)

	_, err = c.callPostcardRPC(o.ctx, lib.MethodTypeSubmitTx, txPostcard, o.requestID)
	return err
}
func (c *rpcClientV1) SimulateTx(transaction *lib.Transaction, opts ...RequestOption) (*SimulateTxResult, error) {
	txPostcard, err := transaction.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	o := applyRequestOptions(opts)

	httpResponse, err := c.callPostcardRPC(o.ctx, lib.MethodTypeSimulateTx, txPostcard, o.requestID)
	if err != nil {
		return nil, err
	}

	simulateReceipt, err := decodePostcardBody[api.SimulateReceipt](httpResponse.Body, "SimulateReceipt", c.typeResolver)
	if err != nil {
		return nil, err
	}

	return &SimulateTxResult{
		HTTPResponseBody:    httpResponse.Body,
		BodySimulateReceipt: simulateReceipt,
	}, nil
}
func (c *rpcClientV1) View(wires []api.PackedInstruction, opts ...RequestOption) (*ViewResult, error) {
	serializer := postcard.NewSerializer()
	if err := serializer.SerializeU32(uint32(len(wires))); err != nil {
		return nil, fmt.Errorf("failed to serialize wires length: %w", err)
	}
	for _, w := range wires {
		if err := serializer.SerializeBytes(w); err != nil {
			return nil, fmt.Errorf("failed to serialize wire: %w", err)
		}
	}

	o := applyRequestOptions(opts)

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeView, serializer.Bytes(), o.requestID)
	if err != nil {
		return nil, err
	}

	return &ViewResult{
		HTTPResponseBody: apiResponse.Body,
	}, nil
}
func (c *rpcClientV1) GetAccount(accountRelaxed any, opts ...RequestOption) (*GetAccountResult, error) {
	o := applyRequestOptions(opts)

	account, err := crypto.NewAddressFromRelaxed(accountRelaxed)
	if err != nil {
		return nil, fmt.Errorf("failed to decode accountRelaxed: %w", err)
	}
	serializer := postcard.NewSerializer()
	if err = account.MarshalPostcard(serializer); err != nil {
		return nil, fmt.Errorf("failed to serialize accountRelaxed: %w", err)
	}

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeGetAccount, serializer.Bytes(), o.requestID)
	if err != nil {
		return nil, err
	}

	accountView, err := decodePostcardBody[api.AccountView](apiResponse.Body, "AccountView", c.typeResolver)
	if err != nil {
		return nil, err
	}

	return &GetAccountResult{
		HTTPResponseBody: apiResponse.Body,
		BodyAccountView:  accountView,
	}, nil
}
func (c *rpcClientV1) EventsByTxHash(txHashRelaxed any, typeTagFilter *uint64, opts ...RequestOption) (*EventsByTxHashResult, error) {
	o := applyRequestOptions(opts)

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

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeEventsByTxHash, serializer.Bytes(), o.requestID)
	if err != nil {
		return nil, err
	}

	eventsByTxHashResponse, err := decodePostcardBody[api.EventsByTxHash](apiResponse.Body, "EventsByTxHash", c.typeResolver)
	if err != nil {
		return nil, err
	}

	return &EventsByTxHashResult{
		HTTPResponseBody:   apiResponse.Body,
		BodyEventsByTxHash: eventsByTxHashResponse,
	}, nil
}

func (c *rpcClientV1) GetBlockByHeight(blockHeight uint64, opts ...RequestOption) (*GetBlockByHeightResult, error) {
	o := applyRequestOptions(opts)

	serializer := postcard.NewSerializer()
	if err := serializer.SerializeU64(blockHeight); err != nil {
		return nil, fmt.Errorf("failed to serialize blockHeight: %w", err)
	}

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeGetBlockByHeight, serializer.Bytes(), o.requestID)
	if err != nil {
		return nil, err
	}

	block, err := decodePostcardBody[api.Block](apiResponse.Body, "Block", c.typeResolver)
	if err != nil {
		return nil, err
	}

	return &GetBlockByHeightResult{
		HTTPResponseBody: apiResponse.Body,
		BodyBlock:        block,
	}, nil
}
func (c *rpcClientV1) GetTxByHash(txHashOrTxIdRelaxed any, opts ...RequestOption) (*GetTxByHashResult, error) {
	o := applyRequestOptions(opts)

	txHashOrTxId, err := api.NewTxHashOrTxIdFromRelaxed(txHashOrTxIdRelaxed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse txHashOrTxId: %w", err)
	}

	serializer := postcard.NewSerializer()
	if err = serializer.SerializeBytes(txHashOrTxId); err != nil {
		return nil, fmt.Errorf("failed to serialize txHashOrTxId: %w", err)
	}

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeGetTxByHash, serializer.Bytes(), o.requestID)
	if err != nil {
		return nil, err
	}

	txHistory, err := decodePostcardBody[api.TxHistory](apiResponse.Body, "TxHistory", c.typeResolver)
	if err != nil {
		return nil, err
	}

	return &GetTxByHashResult{
		HTTPResponseBody: apiResponse.Body,
		BodyTxHistory:    txHistory,
	}, nil
}
func (c *rpcClientV1) GetTxHistoryProof(txHashOrTxIdRelaxed any, opts ...RequestOption) (*GetTxHistoryProofResult, error) {
	o := applyRequestOptions(opts)

	txHashOrTxId, err := api.NewTxHashOrTxIdFromRelaxed(txHashOrTxIdRelaxed)
	if err != nil {
		return nil, fmt.Errorf("failed to parse txHashOrTxId: %w", err)
	}

	serializer := postcard.NewSerializer()
	if err = serializer.SerializeBytes(txHashOrTxId); err != nil {
		return nil, fmt.Errorf("failed to serialize txHashOrTxId: %w", err)
	}

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeGetTxHistoryProof, serializer.Bytes(), o.requestID)
	if err != nil {
		return nil, err
	}

	getTxHistoryProof, err := decodePostcardBody[api.GetTxHistoryProof](apiResponse.Body, "GetTxHistoryProof", c.typeResolver)
	if err != nil {
		return nil, err
	}

	return &GetTxHistoryProofResult{
		HTTPResponseBody:      apiResponse.Body,
		BodyGetTxHistoryProof: getTxHistoryProof,
	}, nil
}

func (c *rpcClientV1) GetResource(rsHash api.RsHash, opts ...RequestOption) (*GetResourceResult, error) {
	o := applyRequestOptions(opts)

	serializer := postcard.NewSerializer()
	serializer.SerializeFixedBytes(rsHash[:])

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeGetResource, serializer.Bytes(), o.requestID)
	if err != nil {
		return nil, err
	}

	getResource, err := decodePostcardBody[api.GetResource](apiResponse.Body, "GetResource", c.typeResolver)
	if err != nil {
		return nil, err
	}

	return &GetResourceResult{
		HTTPResponseBody: apiResponse.Body,
		BodyGetResource:  getResource,
	}, nil
}
func (c *rpcClientV1) GetResourcePathByHash(rsHash api.RsHash, opts ...RequestOption) (*GetResourcePathByHashResult, error) {
	o := applyRequestOptions(opts)

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeGetResourcePathByHash, rsHash[:], o.requestID)
	if err != nil {
		return nil, err
	}

	var path string
	if err = json.Unmarshal(apiResponse.Body, &path); err != nil {
		return nil, fmt.Errorf("failed to unmarshal GetResourcePathByHash body: %w", err)
	}

	return &GetResourcePathByHashResult{
		HTTPResponseBody: apiResponse.Body,
		Path:             path,
	}, nil
}
func (c *rpcClientV1) BatchGetResourcePathByHash(rsHashList []api.RsHash, opts ...RequestOption) (*BatchGetResourcePathByHashResult, error) {
	o := applyRequestOptions(opts)

	serializer := postcard.NewSerializer()
	if err := postcard.SerializeSeq(serializer, rsHashList, func(s *postcard.Serializer, rsHash api.RsHash) error {
		s.SerializeFixedBytes(rsHash[:])
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to serialize rsHashList: %w", err)
	}

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeBatchGetResourcePathByHash, serializer.Bytes(), o.requestID)
	if err != nil {
		return nil, err
	}

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
func (c *rpcClientV1) GetAccessValue(blobHashList []api.BlobHash, opts ...RequestOption) (*GetAccessValueResult, error) {
	o := applyRequestOptions(opts)

	serializer := postcard.NewSerializer()
	if err := postcard.SerializeSeq(serializer, blobHashList, func(s *postcard.Serializer, bh api.BlobHash) error {
		s.SerializeFixedBytes(bh[:])
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to serialize blobHashList: %w", err)
	}

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeGetAccessValue, serializer.Bytes(), o.requestID)
	if err != nil {
		return nil, err
	}

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

	return &GetAccessValueResult{
		HTTPResponseBody:    apiResponse.Body,
		BodyGetAccessValues: accessValues,
	}, nil
}

func (c *rpcClientV1) WaitForTransaction(txHashOrTxIdRelaxed any, opts ...WaitOption) (*GetTxByHashResult, error) {
	o := c.applyWaitOptions(opts)

	if o.pollPeriod <= 0 {
		return nil, fmt.Errorf("WaitForTransaction: invalid poll period %v, must be positive", o.pollPeriod)
	}

	txHashOrTxId, err := api.NewTxHashOrTxIdFromRelaxed(txHashOrTxIdRelaxed)
	if err != nil {
		return nil, fmt.Errorf("failed to decode txHash: %w", err)
	}

	deadline := time.Now().Add(o.pollTimeout)
	var lastErr error

	ticker := time.NewTicker(o.pollPeriod)
	defer ticker.Stop()

	for {
		if time.Now().After(deadline) {
			if lastErr != nil {
				return nil, fmt.Errorf("WaitForTransaction timeout after %v, last error: %w", o.pollTimeout, lastErr)
			}
			return nil, fmt.Errorf("WaitForTransaction timeout after %v", o.pollTimeout)
		}

		result, err := c.GetTxByHash(txHashOrTxId, WithContext(o.ctx), WithRequestID(o.requestID))
		if err == nil && result.BodyTxHistory.Receipt.State != api.TxStatePending {
			return result, nil
		}
		if err != nil {
			lastErr = err
		}

		select {
		case <-o.ctx.Done():
			return nil, fmt.Errorf("WaitForTransaction cancelled: %w", o.ctx.Err())
		case <-ticker.C:
		}
	}
}
