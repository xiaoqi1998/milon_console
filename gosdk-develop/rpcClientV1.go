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

type SubmitTxResult struct {
	HTTPResponseBody []byte
	BodyTxHash       *api.TxHash
}

type SimulateTxResult struct {
	HTTPResponseBody    []byte
	BodySimulateReceipt *api.SimulateReceipt
}

type ViewResult struct {
	HTTPResponseBody []byte
	// BodyBatchViewItems holds one Result per instruction, in request order;
	// Data carries the encoded return value of an Ok item, Failure its error.
	BodyBatchViewItems []*api.BatchViewItem
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
	HTTPResponseBody   []byte
	BodyGetAccessValue *api.GetAccessValueInfo
}

type BatchGetAccessValueResult struct {
	HTTPResponseBody    []byte
	BodyGetAccessValues []*api.BatchGetAccessValueItem
}

type BatchGetResourcePathByHashResult struct {
	HTTPResponseBody          []byte
	BodyBatchResourcePathList []*api.BatchGetResourcePathInfo
}

type GetTxHistoryProofResult struct {
	HTTPResponseBody      []byte
	BodyGetTxHistoryProof *api.GetTxHistoryProof
}

type NodeCapabilityResult struct {
	HTTPResponseBody   []byte
	BodyNodeCapability *api.NodeCapabilityView
}

type BatchGetAccountResult struct {
	HTTPResponseBody      []byte
	BodyBatchAccountViews []*api.BatchGetAccountItem
}

type BatchEventsByTxHashResult struct {
	HTTPResponseBody        []byte
	BodyBatchEventsByTxHash []*api.BatchEventsByTxHashItem
}

type BatchGetBlockByHeightResult struct {
	HTTPResponseBody []byte
	BodyBatchBlocks  []*api.BatchGetBlockByHeightItem
}

type BatchGetTxByHashResult struct {
	HTTPResponseBody     []byte
	BodyBatchTxHistories []*api.BatchGetTxByHashItem
}

type BatchGetTxHistoryProofResult struct {
	HTTPResponseBody         []byte
	BodyBatchTxHistoryProofs []*api.BatchGetTxHistoryProofItem
}

type BatchGetResourceResult struct {
	HTTPResponseBody     []byte
	BodyBatchGetResource []*api.BatchGetResourceItem
}

type BatchSubmitTxResult struct {
	HTTPResponseBody  []byte
	BodyBatchTxHashes []*api.BatchSubmitTxItem
}

type BatchSimulateTxResult struct {
	HTTPResponseBody        []byte
	BodyBatchSimulateTxData []*api.BatchSimulateTxItem
}

type ViewSingleResult struct {
	HTTPResponseBody []byte
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

// rpcErrorBodyLimit caps how many response bytes an error message quotes, so a
// failed call stays diagnosable without dumping the whole payload.
const rpcErrorBodyLimit = 128

// bodySnippet renders the leading bytes of body as hex for error messages.
func bodySnippet(body []byte) string {
	if len(body) <= rpcErrorBodyLimit {
		return fmt.Sprintf("%x", body)
	}
	return fmt.Sprintf("%x...(total %d bytes)", body[:rpcErrorBodyLimit], len(body))
}

// encodeJsonRPCRequest builds the request envelope into buf:
// {"method":...,"request_id":...,"body":[...]}. The byte body is encoded as a
// decimal integer array, matching the wire format the node expects for the
// JSON content type. buf must come from jsonRPCBufferPool and be returned to
// the pool once the HTTP call completes.
func encodeJsonRPCRequest(bufPtr *[]byte, method lib.MethodType, requestID lib.RequestID, body []byte) []byte {
	buf := (*bufPtr)[:0]

	buf = append(buf, `{"method":`...)
	buf = strconv.AppendUint(buf, uint64(method), 10)
	buf = append(buf, `,"request_id":`...)
	buf = strconv.AppendUint(buf, uint64(requestID), 10)
	buf = append(buf, `,"body":[`...)
	// Each byte renders as at most three decimal digits plus a separator, so
	// reserve that up front to keep the loop from re-growing the slice on large
	// bodies (the pooled capacity only covers small requests).
	if cap(buf)-len(buf) < len(body)*4 {
		next := make([]byte, len(buf), len(buf)+len(body)*4+2)
		copy(next, buf)
		buf = next
	}
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
	bufPtr := jsonRPCBufferPool.Get().(*[]byte)
	payload := encodeJsonRPCRequest(bufPtr, method, requestID, body)
	httpStatusCode, httpResponseBytes, err := tools.HttpPostByBytes(
		ctx,
		c.network.RpcUrl,
		payload,
		map[string]string{
			"Content-Type": lib.ContentTypeMilonJson,
		},
	)
	// The pooled buffer is reused only after the HTTP call consumed it.
	jsonRPCBufferPool.Put(bufPtr)
	if err != nil {
		return nil, fmt.Errorf("%s RPC call failed: %w", method, err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned HTTP status %d, body: %s", method, httpStatusCode, bodySnippet(httpResponseBytes))
	}

	apiResponse := &lib.RpcResponse{}
	if err = json.Unmarshal(httpResponseBytes, apiResponse); err != nil {
		return nil, fmt.Errorf("%s returned malformed JSON response (%w), body: %s", method, err, bodySnippet(httpResponseBytes))
	}
	if apiResponse.RequestId != uint64(requestID) {
		return nil, fmt.Errorf("%s response request_id %d does not match request %d", method, apiResponse.RequestId, requestID)
	}
	if apiResponse.Status != lib.RpcResponseStatusOk {
		if apiResponse.Error != nil {
			return nil, fmt.Errorf("%s returned error status %d: %+v", method, apiResponse.Status, *apiResponse.Error)
		}
		return nil, fmt.Errorf("%s returned error status: %d", method, apiResponse.Status)
	}
	return apiResponse, nil
}

// callPostcardRPC sends an RPC request in postcard format and returns the
// parsed response, retrying transient failures.
func (c *rpcClientV1) callPostcardRPC(ctx context.Context, method lib.MethodType, body []byte, requestID lib.RequestID) (*lib.RpcResponse, error) {
	return c.callPostcardRPCWith(ctx, method, body, requestID, true)
}

// callPostcardRPCNoRetry is callPostcardRPC without automatic retries. It must
// be used by non-idempotent methods: retrying a submission whose response was
// lost could put the same transaction on chain twice.
func (c *rpcClientV1) callPostcardRPCNoRetry(ctx context.Context, method lib.MethodType, body []byte, requestID lib.RequestID) (*lib.RpcResponse, error) {
	return c.callPostcardRPCWith(ctx, method, body, requestID, false)
}

func (c *rpcClientV1) callPostcardRPCWith(ctx context.Context, method lib.MethodType, body []byte, requestID lib.RequestID, retry bool) (*lib.RpcResponse, error) {
	rpcReq := lib.NewRpcRequest(method, requestID, body)
	serializer := postcard.NewSerializerWithCap(len(body) + 32)
	if err := rpcReq.MarshalPostcard(serializer); err != nil {
		return nil, fmt.Errorf("%s: failed to serialize RPC request: %w", method, err)
	}
	rpcReqPostcard := serializer.Bytes()

	header := map[string]string{"Content-Type": lib.ContentTypeMilonPostcard}
	var httpStatusCode int
	var httpResponseBytes []byte
	var err error
	if retry {
		httpStatusCode, httpResponseBytes, err = tools.HttpPostByBytes(ctx, c.network.RpcUrl, rpcReqPostcard, header)
	} else {
		httpStatusCode, httpResponseBytes, err = tools.HttpPostByBytesNoRetry(ctx, c.network.RpcUrl, rpcReqPostcard, header)
	}
	if err != nil {
		return nil, fmt.Errorf("%s RPC call failed: %w", method, err)
	}
	if httpStatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s returned HTTP status %d, body: %s", method, httpStatusCode, bodySnippet(httpResponseBytes))
	}

	httpResponse, err := decodePostcardBody[lib.RpcResponse](httpResponseBytes, method.String()+" response", c.typeResolver)
	if err != nil {
		return nil, err
	}
	if httpResponse.RequestId != uint64(requestID) {
		return nil, fmt.Errorf("%s response request_id %d does not match request %d", method, httpResponse.RequestId, requestID)
	}
	if httpResponse.Status != lib.RpcResponseStatusOk {
		if httpResponse.Error != nil {
			return nil, fmt.Errorf("%s returned error status %d: %+v", method, httpResponse.Status, *httpResponse.Error)
		}
		return nil, fmt.Errorf("%s returned error status: %d", method, httpResponse.Status)
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

// newDeserializer returns a postcard deserializer over body with the client's
// TypeResolver injected. Values that carry a type_tag (access records, events)
// are stored without a length prefix, so their boundary can only be resolved
// through the loaded IDLs; without it those values fail to decode.
func (c *rpcClientV1) newDeserializer(body []byte) *postcard.Deserializer {
	deserializer := postcard.NewDeserializer(body)
	if c.typeResolver != nil {
		deserializer.SetTypeResolver(c.typeResolver)
	}
	return deserializer
}

// decodeBatchSeq decodes a batch response body (postcard Vec<T>) with the
// TypeResolver injected and rejects trailing bytes, which would otherwise
// mean the items were delimited incorrectly.
func decodeBatchSeq[T any](c *rpcClientV1, body []byte, name string, fn postcard.DeserializerFunc[T]) ([]T, error) {
	deserializer := c.newDeserializer(body)
	items, err := postcard.DeserializeSeq(deserializer, fn)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize %s sequence: %w", name, err)
	}
	if err := deserializer.AssertEnd(); err != nil {
		return nil, fmt.Errorf("failed to deserialize %s sequence: %w", name, err)
	}
	return items, nil
}

// decodeBatchItems decodes a batch response body (postcard Vec<Result<O, ..>>)
// whose item type O implements postcard.Unmarshaler on its pointer receiver.
// Every Batch* RPC shares this exact item envelope, so this removes the
// per-method "declare item / UnmarshalPostcard / wrap error" closure.
func decodeBatchItems[T any, PT interface {
	*T
	postcard.Unmarshaler
}](c *rpcClientV1, body []byte, name string) ([]*T, error) {
	return decodeBatchSeq(c, body, name, func(d *postcard.Deserializer) (*T, error) {
		var item T
		if err := PT(&item).UnmarshalPostcard(d); err != nil {
			return nil, fmt.Errorf("failed to deserialize %s item: %w", name, err)
		}
		return &item, nil
	})
}

// decodeSubmitTxHash decodes the response body of submit_tx (RPC method 5).
// handlers.rs::submit_tx returns the hash as [u8; 32] and dispatch.rs passes it
// on through Vec::to_vec, so the body is the bare 32 bytes: the length prefix of
// the response envelope was already consumed when RpcResponse.Body was decoded.
func decodeSubmitTxHash(body []byte) (*api.TxHash, error) {
	if len(body) != api.TxHashLen {
		return nil, fmt.Errorf("SubmitTx returned %d hash bytes, want %d", len(body), api.TxHashLen)
	}
	hash := &api.TxHash{}
	copy(hash[:], body)
	return hash, nil
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
	_, err = c.SubmitTxWithSponsorIxes(tx, []uint8{0})
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
	_, err = c.SubmitTx(tx)
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

// resolveBalanceToken applies the optional token argument of BalanceOf: an
// omitted token means api.MILToken, and at most one token may be supplied.
func resolveBalanceToken(token []*crypto.Address) (*crypto.Address, error) {
	switch len(token) {
	case 0:
		return api.MILToken, nil
	case 1:
		if token[0] == nil {
			return nil, fmt.Errorf("token address is nil")
		}
		return token[0], nil
	default:
		return nil, fmt.Errorf("BalanceOf accepts at most one token, got %d", len(token))
	}
}

// BalanceOf returns the balance of account for the given token. token is
// optional and defaults to api.MILToken; at most one token may be supplied.
func (c *rpcClientV1) BalanceOf(account *crypto.Address, token ...*crypto.Address) (uint64, error) {
	if account == nil {
		return 0, fmt.Errorf("account address is nil")
	}

	tokenAddr, err := resolveBalanceToken(token)
	if err != nil {
		return 0, err
	}

	balance, err := ViewValue(c, gen.Token.BalanceOf.Args(tokenAddr, account).Encode, gen.Token.BalanceOf.DecodeView)
	if err != nil {
		return 0, fmt.Errorf("failed to get BalanceOf: %w", err)
	}
	return balance, nil
}

func (c *rpcClientV1) ListAccountSigners(account *crypto.Address) ([]any, error) {
	out, err := ViewValue(c, gen.Account.ListSigners.Args(account).Encode, gen.Account.ListSigners.DecodeView)
	if err != nil {
		return nil, fmt.Errorf("failed to get ListSigners: %w", err)
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

func (c *rpcClientV1) TokenMetadata(token *crypto.Address) (gen.TokenMetadata, error) {
	metadata, err := ViewValue(c, gen.Token.Metadata.Args(token).Encode, gen.Token.Metadata.DecodeView)
	if err != nil {
		return gen.TokenMetadata{}, fmt.Errorf("failed to get TokenMetadata: %w", err)
	}
	return metadata, nil
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
func (c *rpcClientV1) SubmitTx(tx *lib.Transaction, opts ...RequestOption) (*SubmitTxResult, error) {
	return c.submitTx(tx, nil, opts...)
}

func (c *rpcClientV1) SubmitTxWithSponsorIxes(tx *lib.Transaction, sponsorIxes []uint8, opts ...RequestOption) (*SubmitTxResult, error) {
	return c.submitTx(tx, sponsorIxes, opts...)
}

// SubmitAndWait submits a transaction and waits until its receipt is no longer
// pending, returning the full receipt. Submission, waiting and on-chain
// execution all collapse into a single error, so a caller that only needs the
// committed transaction writes:
//
//	txHistory, err := client.SubmitAndWait(tx)
//
// Polling uses the client defaults (WithClientPollPeriod / WithClientPollTimeout)
// and can be overridden per call with WaitOption.
func (c *rpcClientV1) SubmitAndWait(tx *lib.Transaction, opts ...WaitOption) (*api.TxHistory, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is nil")
	}

	if _, err := c.SubmitTx(tx); err != nil {
		return nil, fmt.Errorf("failed to submit transaction: %w", err)
	}

	result, err := c.WaitForTransaction(tx.TxHash(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for transaction: %w", err)
	}

	// The state check mirrors helper.CheckTxSuccess; only the api package can be
	// imported here, because helper imports this package for its RPC helpers.
	txHistory := result.BodyTxHistory
	if txHistory == nil {
		return nil, fmt.Errorf("transaction history is nil")
	}
	if txHistory.Receipt.State != api.TxStateSuccess {
		code := uint16(0)
		if txHistory.Receipt.Error != nil {
			code = *txHistory.Receipt.Error
		}
		return nil, fmt.Errorf("transaction failed on chain: error code = %d", code)
	}
	return txHistory, nil
}

// submitTx implements SubmitTx (sponsorIxes == nil) and
// SubmitTxWithSponsorIxes. ValidateWireWith treats nil and empty sponsor lists
// alike, so both forms share the same validation path. Submission is not
// retried automatically: a duplicate write would be worse than a caller-level
// retry.
func (c *rpcClientV1) submitTx(tx *lib.Transaction, sponsorIxes []uint8, opts ...RequestOption) (*SubmitTxResult, error) {
	if tx == nil {
		return nil, fmt.Errorf("transaction is nil")
	}
	if err := tx.ValidateWireWith(sponsorIxes); err != nil {
		return nil, fmt.Errorf("transaction validation failed: %w", err)
	}

	txPostcard, err := tx.ToBytes()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize transaction: %w", err)
	}

	o := applyRequestOptions(opts)

	apiResponse, err := c.callPostcardRPCNoRetry(o.ctx, lib.MethodTypeSubmitTx, txPostcard, o.requestID)
	if err != nil {
		return nil, err
	}

	txHash, err := decodeSubmitTxHash(apiResponse.Body)
	if err != nil {
		return nil, err
	}

	return &SubmitTxResult{
		HTTPResponseBody: apiResponse.Body,
		BodyTxHash:       txHash,
	}, nil
}

// SimulateTx simulates a transaction without submitting it. Wire validation is
// intentionally skipped: simulations are commonly run on transactions whose
// signatures are still placeholders.
func (c *rpcClientV1) SimulateTx(transaction *lib.Transaction, opts ...RequestOption) (*SimulateTxResult, error) {
	if transaction == nil {
		return nil, fmt.Errorf("transaction is nil")
	}

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

// BatchView executes one view instruction per entry, in request order (RPC
// method 16). Each item is a Result<Vec<u8>, ApiFailure>; an Ok item's Data
// holds the encoded return value that the single view method (15) returns.
func (c *rpcClientV1) BatchView(wires []api.PackedInstruction, opts ...RequestOption) (*ViewResult, error) {
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

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeBatchView, serializer.Bytes(), o.requestID)
	if err != nil {
		return nil, err
	}

	items, err := decodeBatchItems[api.BatchViewItem](c, apiResponse.Body, "BatchView")
	if err != nil {
		return nil, err
	}

	return &ViewResult{
		HTTPResponseBody:   apiResponse.Body,
		BodyBatchViewItems: items,
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

// GetAccessValue fetches one raw access value (RPC method 165). The request
// body is a bare BlobHash and the response body is the single tuple
// (BlobHash, Option<Vec<u8>>).
func (c *rpcClientV1) GetAccessValue(blobHash api.BlobHash, opts ...RequestOption) (*GetAccessValueResult, error) {
	o := applyRequestOptions(opts)

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeGetAccessValue, blobHash[:], o.requestID)
	if err != nil {
		return nil, err
	}

	accessValue, err := decodePostcardBody[api.GetAccessValueInfo](apiResponse.Body, "GetAccessValue", c.typeResolver)
	if err != nil {
		return nil, err
	}

	return &GetAccessValueResult{
		HTTPResponseBody:   apiResponse.Body,
		BodyGetAccessValue: accessValue,
	}, nil
}

// BatchGetAccessValue fetches raw access values (RPC method 166). Each response
// item is a Result, so a per-item failure does not abort the whole batch.
func (c *rpcClientV1) BatchGetAccessValue(blobHashList []api.BlobHash, opts ...RequestOption) (*BatchGetAccessValueResult, error) {
	o := applyRequestOptions(opts)

	serializer := postcard.NewSerializer()
	if err := postcard.SerializeSeq(serializer, blobHashList, func(s *postcard.Serializer, bh api.BlobHash) error {
		s.SerializeFixedBytes(bh[:])
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to serialize blobHashList: %w", err)
	}

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeBatchGetAccessValue, serializer.Bytes(), o.requestID)
	if err != nil {
		return nil, err
	}

	accessValues, err := decodeBatchItems[api.BatchGetAccessValueItem](c, apiResponse.Body, "BatchGetAccessValue")
	if err != nil {
		return nil, err
	}

	return &BatchGetAccessValueResult{
		HTTPResponseBody:    apiResponse.Body,
		BodyGetAccessValues: accessValues,
	}, nil
}

// GetNodeCapability fetches the node's local MIP-33 capability view (RPC
// method 30). Request body is empty; response body is postcard
// NodeCapabilityView.
func (c *rpcClientV1) GetNodeCapability(opts ...RequestOption) (*NodeCapabilityResult, error) {
	o := applyRequestOptions(opts)

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeNodeCapability, []byte{}, o.requestID)
	if err != nil {
		return nil, err
	}

	capability, err := decodePostcardBody[api.NodeCapabilityView](apiResponse.Body, "NodeCapability", c.typeResolver)
	if err != nil {
		return nil, err
	}

	return &NodeCapabilityResult{
		HTTPResponseBody:   apiResponse.Body,
		BodyNodeCapability: capability,
	}, nil
}

// BatchGetAccount fetches account views (RPC method 21). Each response item is
// a Result so a per-address failure (e.g. AccountNotFound) does not abort the
// whole batch.
func (c *rpcClientV1) BatchGetAccount(accountRelaxedList []any, opts ...RequestOption) (*BatchGetAccountResult, error) {
	o := applyRequestOptions(opts)

	serializer := postcard.NewSerializer()
	if err := serializer.SerializeU32(uint32(len(accountRelaxedList))); err != nil {
		return nil, fmt.Errorf("failed to serialize account list length: %w", err)
	}
	for i, relaxed := range accountRelaxedList {
		account, err := crypto.NewAddressFromRelaxed(relaxed)
		if err != nil {
			return nil, fmt.Errorf("failed to decode accountRelaxed at index %d: %w", i, err)
		}
		if err = account.MarshalPostcard(serializer); err != nil {
			return nil, fmt.Errorf("failed to serialize account at index %d: %w", i, err)
		}
	}

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeBatchGetAccount, serializer.Bytes(), o.requestID)
	if err != nil {
		return nil, err
	}

	items, err := decodeBatchItems[api.BatchGetAccountItem](c, apiResponse.Body, "BatchGetAccount")
	if err != nil {
		return nil, err
	}

	return &BatchGetAccountResult{
		HTTPResponseBody:      apiResponse.Body,
		BodyBatchAccountViews: items,
	}, nil
}

// BatchEventsByTxHash fetches events for multiple transactions (RPC method 26).
func (c *rpcClientV1) BatchEventsByTxHash(reqList []api.EventsByTxHashReq, opts ...RequestOption) (*BatchEventsByTxHashResult, error) {
	o := applyRequestOptions(opts)

	serializer := postcard.NewSerializer()
	if err := postcard.SerializeSeq(serializer, reqList, func(s *postcard.Serializer, req api.EventsByTxHashReq) error {
		return req.MarshalPostcard(s)
	}); err != nil {
		return nil, fmt.Errorf("failed to serialize EventsByTxHashReq list: %w", err)
	}

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeBatchEventsByTxHash, serializer.Bytes(), o.requestID)
	if err != nil {
		return nil, err
	}

	items, err := decodeBatchItems[api.BatchEventsByTxHashItem](c, apiResponse.Body, "BatchEventsByTxHash")
	if err != nil {
		return nil, err
	}

	return &BatchEventsByTxHashResult{
		HTTPResponseBody:        apiResponse.Body,
		BodyBatchEventsByTxHash: items,
	}, nil
}

// BatchGetBlockByHeight fetches blocks by height (RPC method 51). Each Ok
// item's inner Vec<u8> is decoded into a Block; failures are exposed via
// item.Failure.
func (c *rpcClientV1) BatchGetBlockByHeight(heights []uint64, opts ...RequestOption) (*BatchGetBlockByHeightResult, error) {
	o := applyRequestOptions(opts)

	serializer := postcard.NewSerializer()
	if err := postcard.SerializeSeq(serializer, heights, func(s *postcard.Serializer, h uint64) error {
		return s.SerializeU64(h)
	}); err != nil {
		return nil, fmt.Errorf("failed to serialize heights: %w", err)
	}

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeBatchGetBlockByHeight, serializer.Bytes(), o.requestID)
	if err != nil {
		return nil, err
	}

	items, err := decodeBatchItems[api.BatchGetBlockByHeightItem](c, apiResponse.Body, "BatchGetBlockByHeight")
	if err != nil {
		return nil, err
	}

	return &BatchGetBlockByHeightResult{
		HTTPResponseBody: apiResponse.Body,
		BodyBatchBlocks:  items,
	}, nil
}

// BatchGetTxByHash fetches TxHistory for multiple identifiers (RPC method 56).
// Each element follows the same relaxed parsing as GetTxByHash (base58/hex
// string, api.TxHash, api.TxId or raw 12/32 bytes).
func (c *rpcClientV1) BatchGetTxByHash(txHashOrTxIdRelaxedList []any, opts ...RequestOption) (*BatchGetTxByHashResult, error) {
	o := applyRequestOptions(opts)

	body, err := serializeBatchTxIdBody(txHashOrTxIdRelaxedList)
	if err != nil {
		return nil, err
	}

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeBatchGetTxByHash, body, o.requestID)
	if err != nil {
		return nil, err
	}

	items, err := decodeBatchItems[api.BatchGetTxByHashItem](c, apiResponse.Body, "BatchGetTxByHash")
	if err != nil {
		return nil, err
	}

	return &BatchGetTxByHashResult{
		HTTPResponseBody:     apiResponse.Body,
		BodyBatchTxHistories: items,
	}, nil
}

// BatchGetTxHistoryProof fetches TxHistory inclusion proofs for multiple
// identifiers (RPC method 61). Each element follows the same relaxed parsing
// as GetTxHistoryProof.
func (c *rpcClientV1) BatchGetTxHistoryProof(txHashOrTxIdRelaxedList []any, opts ...RequestOption) (*BatchGetTxHistoryProofResult, error) {
	o := applyRequestOptions(opts)

	body, err := serializeBatchTxIdBody(txHashOrTxIdRelaxedList)
	if err != nil {
		return nil, err
	}

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeBatchGetTxHistoryProof, body, o.requestID)
	if err != nil {
		return nil, err
	}

	items, err := decodeBatchItems[api.BatchGetTxHistoryProofItem](c, apiResponse.Body, "BatchGetTxHistoryProof")
	if err != nil {
		return nil, err
	}

	return &BatchGetTxHistoryProofResult{
		HTTPResponseBody:         apiResponse.Body,
		BodyBatchTxHistoryProofs: items,
	}, nil
}

// BatchGetResource fetches raw resource values by RsHash (RPC method 151).
// Each Ok item's inner Vec<u8> is decoded into an api.GetResource; failures
// are exposed via item.Failure.
func (c *rpcClientV1) BatchGetResource(rsHashList []api.RsHash, opts ...RequestOption) (*BatchGetResourceResult, error) {
	o := applyRequestOptions(opts)

	serializer := postcard.NewSerializer()
	if err := postcard.SerializeSeq(serializer, rsHashList, func(s *postcard.Serializer, rsHash api.RsHash) error {
		s.SerializeFixedBytes(rsHash[:])
		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed to serialize rsHashList: %w", err)
	}

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeBatchGetResource, serializer.Bytes(), o.requestID)
	if err != nil {
		return nil, err
	}

	items, err := decodeBatchItems[api.BatchGetResourceItem](c, apiResponse.Body, "BatchGetResource")
	if err != nil {
		return nil, err
	}

	return &BatchGetResourceResult{
		HTTPResponseBody:     apiResponse.Body,
		BodyBatchGetResource: items,
	}, nil
}

// serializeBatchTxIdBody encodes the request body shared by BatchGetTxByHash
// (56) and BatchGetTxHistoryProof (61): a postcard Vec of tx identifiers, each
// parsed from its relaxed form (base58/hex string, api.TxHash, api.TxId or raw
// bytes) exactly like the single-identifier methods. The list length is a
// varint followed by each identifier as a length-prefixed byte blob.
func serializeBatchTxIdBody(txHashOrTxIdRelaxedList []any) ([]byte, error) {
	serializer := postcard.NewSerializer()
	if err := serializer.SerializeU32(uint32(len(txHashOrTxIdRelaxedList))); err != nil {
		return nil, fmt.Errorf("failed to serialize identifier list length: %w", err)
	}
	for i, relaxed := range txHashOrTxIdRelaxedList {
		id, err := api.NewTxHashOrTxIdFromRelaxed(relaxed)
		if err != nil {
			return nil, fmt.Errorf("failed to decode txHashOrTxId at index %d: %w", i, err)
		}
		if err = serializer.SerializeBytes(id); err != nil {
			return nil, fmt.Errorf("failed to serialize txHashOrTxId at index %d: %w", i, err)
		}
	}
	return serializer.Bytes(), nil
}

// serializeBatchTxBody encodes the request body shared by BatchSubmitTx (6)
// and BatchSimulateTx (11): a postcard Vec<Transaction>. The node decodes
// the transactions as structs (not byte blobs), so each one is serialized
// inline after the length varint, exactly like the single-method body.
func serializeBatchTxBody(txs []*lib.Transaction, validateWire bool) ([]byte, error) {
	serializer := postcard.NewSerializer()
	if err := serializer.SerializeU32(uint32(len(txs))); err != nil {
		return nil, fmt.Errorf("failed to serialize tx list length: %w", err)
	}
	for i, tx := range txs {
		if tx == nil {
			return nil, fmt.Errorf("transaction at index %d is nil", i)
		}
		if validateWire {
			if err := tx.ValidateWire(); err != nil {
				return nil, fmt.Errorf("transaction[%d] validation failed: %w", i, err)
			}
		}
		if err := tx.MarshalPostcard(serializer); err != nil {
			return nil, fmt.Errorf("failed to serialize transaction at index %d: %w", i, err)
		}
	}
	return serializer.Bytes(), nil
}

// BatchSubmitTx submits multiple signed transactions (RPC method 6). Each
// Ok item carries the 32-byte hash of the transaction accepted into the
// verify queue; per-transaction failures are exposed via item.Failure.
func (c *rpcClientV1) BatchSubmitTx(txs []*lib.Transaction, opts ...RequestOption) (*BatchSubmitTxResult, error) {
	body, err := serializeBatchTxBody(txs, true)
	if err != nil {
		return nil, err
	}

	o := applyRequestOptions(opts)

	apiResponse, err := c.callPostcardRPCNoRetry(o.ctx, lib.MethodTypeBatchSubmitTx, body, o.requestID)
	if err != nil {
		return nil, err
	}

	items, err := decodeBatchItems[api.BatchSubmitTxItem](c, apiResponse.Body, "BatchSubmitTx")
	if err != nil {
		return nil, err
	}

	return &BatchSubmitTxResult{
		HTTPResponseBody:  apiResponse.Body,
		BodyBatchTxHashes: items,
	}, nil
}

// BatchSimulateTx simulates multiple transactions (RPC method 11). Each Ok
// item's inner Vec<u8> is the framed SimulateReceipt body the single
// simulate_tx method returns; failures are exposed via item.Failure.
func (c *rpcClientV1) BatchSimulateTx(txs []*lib.Transaction, opts ...RequestOption) (*BatchSimulateTxResult, error) {
	body, err := serializeBatchTxBody(txs, false)
	if err != nil {
		return nil, err
	}

	o := applyRequestOptions(opts)

	apiResponse, err := c.callPostcardRPC(o.ctx, lib.MethodTypeBatchSimulateTx, body, o.requestID)
	if err != nil {
		return nil, err
	}

	items, err := decodeBatchItems[api.BatchSimulateTxItem](c, apiResponse.Body, "BatchSimulateTx")
	if err != nil {
		return nil, err
	}

	return &BatchSimulateTxResult{
		HTTPResponseBody:        apiResponse.Body,
		BodyBatchSimulateTxData: items,
	}, nil
}

// View executes a single view instruction (RPC method 15). The response body is
// the plain encoded return value (no Vec<Result<..>> envelope); decode it with
// provider.Provider.DecodeViewValue, which the generated DecodeView methods do.
func (c *rpcClientV1) View(wire api.PackedInstruction, opts ...RequestOption) (*ViewSingleResult, error) {
	serializer := postcard.NewSerializer()
	if err := serializer.SerializeBytes(wire); err != nil {
		return nil, fmt.Errorf("failed to serialize wire: %w", err)
	}

	o := applyRequestOptions(opts)

	apiResponse, err := c.callJsonRPC(o.ctx, lib.MethodTypeView, serializer.Bytes(), o.requestID)
	if err != nil {
		return nil, err
	}

	return &ViewSingleResult{
		HTTPResponseBody: apiResponse.Body,
	}, nil
}

// ViewCaller is the minimal client surface ViewValue needs; both *Client and
// the concrete RPC implementation satisfy it.
type ViewCaller interface {
	View(wire api.PackedInstruction, opts ...RequestOption) (*ViewSingleResult, error)
}

// validatePollConfig rejects polling settings that would never wait: a
// non-positive period spins the loop and a non-positive timeout expires before
// the first attempt is made.
func validatePollConfig(period, timeout time.Duration) error {
	if period <= 0 {
		return fmt.Errorf("invalid poll period %v, must be positive", period)
	}
	if timeout <= 0 {
		return fmt.Errorf("invalid poll timeout %v, must be positive", timeout)
	}
	return nil
}

func (c *rpcClientV1) WaitForTransaction(txHashOrTxIdRelaxed any, opts ...WaitOption) (*GetTxByHashResult, error) {
	o := c.applyWaitOptions(opts)

	if err := validatePollConfig(o.pollPeriod, o.pollTimeout); err != nil {
		return nil, fmt.Errorf("WaitForTransaction: %w", err)
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
		// Each attempt gets a fresh request id unless the caller pinned one:
		// reusing the id lets a node answer from a per-request cache, which
		// would keep returning the same stale pending receipt.
		requestID := o.requestID
		if !o.requestIDPinned {
			requestID = nextRequestID()
		}

		result, err := c.GetTxByHash(txHashOrTxId, WithContext(o.ctx), WithRequestID(requestID))
		if err == nil {
			if result != nil && result.BodyTxHistory != nil {
				if result.BodyTxHistory.Receipt.State != api.TxStatePending {
					return result, nil
				}
			} else {
				lastErr = fmt.Errorf("empty transaction history in response")
			}
		} else {
			lastErr = err
		}

		// Checked after the attempt so the last poll inside the timeout window
		// is always performed.
		if time.Now().After(deadline) {
			if lastErr != nil {
				return nil, fmt.Errorf("WaitForTransaction timeout after %v, last error: %w", o.pollTimeout, lastErr)
			}
			return nil, fmt.Errorf("WaitForTransaction timeout after %v", o.pollTimeout)
		}

		select {
		case <-o.ctx.Done():
			return nil, fmt.Errorf("WaitForTransaction cancelled: %w", o.ctx.Err())
		case <-ticker.C:
		}
	}
}

// ViewValue runs one view instruction end to end: it encodes the instruction,
// calls View and decodes the response body with the generated DecodeView, so a
// single view can be read in one call:
//
//	balance, err := milon.ViewValue(client,
//		gen.Token.BalanceOf.Args(token, account).Encode,
//		gen.Token.BalanceOf.DecodeView)
//
// The generated DecodeView already unwraps a TxFailurePayload, so a rejected
// view surfaces as an error instead of a zero value. Go methods cannot carry
// type parameters, so the type-safe form is a package-level function.
func ViewValue[T any](client ViewCaller, encode func() (api.PackedInstruction, error), decode func([]byte) (T, error), opts ...RequestOption) (T, error) {
	var zero T
	if client == nil {
		return zero, fmt.Errorf("client is nil")
	}
	if encode == nil || decode == nil {
		return zero, fmt.Errorf("encode and decode funcs are required")
	}

	wire, err := encode()
	if err != nil {
		return zero, fmt.Errorf("failed to encode view instruction: %w", err)
	}

	result, err := client.View(wire, opts...)
	if err != nil {
		return zero, fmt.Errorf("failed to call view: %w", err)
	}

	out, err := decode(result.HTTPResponseBody)
	if err != nil {
		return zero, fmt.Errorf("failed to decode view: %w", err)
	}
	return out, nil
}
