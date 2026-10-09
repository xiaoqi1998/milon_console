package mcpserver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// 镜像 REST request struct 的输入类型（json tag 与 REST 逐字一致）。
// 无参数的工具统一用 emptyArgs。

// emptyArgs 对应无参数的 REST 调用。
type emptyArgs struct{}

type networkSwitchArgs struct {
	Network string `json:"network" jsonschema:"目标网络名：devNet 或 localNet"`
}

type accountGenerateArgs struct {
	// keyType 可缺省（REST 侧 account_handler.go GenerateAccount 空值缺省
	// secp256k1），标 omitempty 使 MCP schema 不把它列入 required——Task 8
	// 修复：此前缺 omitempty 导致客户端传 {} 被 schema 拒绝，与 REST 契约不一致。
	KeyType string `json:"keyType,omitempty" jsonschema:"可选：密钥曲线 secp256k1(缺省)/ed25519/fndsa512（抗量子）"`
}

type addressArgs struct {
	Address string `json:"address"`
}

type faucetClaimArgs struct {
	PrivateKey    string `json:"privateKey" jsonschema:"领款账户私钥（hex 或 base58）"`
	Address       string `json:"address" jsonschema:"领款账户地址"`
	SignatureMode any    `json:"signatureMode" jsonschema:"签名模式对象，如 {\"type\":\"pubkey\",\"publicKey\":\"0x..\"} 或 {\"type\":\"multisig\",\"publicKey\":\"..\",\"index\":N}"`
}

type txParseArgs struct {
	Hash   string `json:"hash"`
	Remote string `json:"remote,omitempty" jsonschema:"可选：true/1 时附取各 inline 写入资源的链上现值与外部 blob 值"`
}

type txEventsArgs struct {
	Hash    string `json:"hash"`
	TypeTag string `json:"typeTag,omitempty" jsonschema:"可选：按事件 typeTag 过滤（十进制整数）"`
}

type txWaitArgs struct {
	Hash        string `json:"hash"`
	TimeoutSecs string `json:"timeoutSecs,omitempty" jsonschema:"可选：等待超时秒数（十进制整数）"`
}

// txTrackArgs 镜像 TrackTransaction 的参数（hash 路径参数 + timeoutSecs query）。
type txTrackArgs struct {
	Hash        string `json:"hash" jsonschema:"必填：交易哈希（hex 或 base58）"`
	TimeoutSecs string `json:"timeoutSecs,omitempty" jsonschema:"可选：等待确认超时秒数（十进制整数，缺省 60）"`
}

// writeSafeArgs 与 contractWriteArgs 相同（请求体同 /api/write）。
type writeSafeArgs = contractWriteArgs

// errorLookupArgs 镜像 Lookup 的路径参数。
type errorLookupArgs struct {
	Query string `json:"query" jsonschema:"必填：十进制错误码或名字子串（如 521 / Cooldown）"`
}

// accountSummaryArgs 镜像 Summary 的路径参数。
type accountSummaryArgs struct {
	Address string `json:"address" jsonschema:"必填：账户地址（base58）"`
}

// transferMilArgs 镜像 transferMilRequest（POST /api/tool/transfer-mil 纯 body）。
type transferMilArgs struct {
	To         string `json:"to" jsonschema:"必填：接收方地址（base58）"`
	Amount     uint64 `json:"amount" jsonschema:"必填：转账数量（最小单位，6 位精度下 1 MIL = 10^6）"`
	PrivateKey string `json:"privateKey" jsonschema:"必填：转出方私钥（hex 或 base58）"`
	KeyType    string `json:"keyType,omitempty" jsonschema:"可选：私钥曲线 secp256k1(缺省)/ed25519/bls12381/fndsa512"`
	PublicKey  string `json:"publicKey,omitempty" jsonschema:"fndsa512 时必填（897 字节公钥，Go 无法从私钥派生；account_generate 的返回里有）"`
}

// idlMethodsArgs 镜像 AppMethods 的路径参数。
type idlMethodsArgs struct {
	AppName string `json:"appName" jsonschema:"必填：app 名（idl_apps 返回的 name，如 token）"`
}

type hashArgs struct {
	Hash string `json:"hash"`
}

type heightArgs struct {
	Height string `json:"height"`
}

type accessValueArgs struct {
	BlobHashes []string `json:"blobHashes"`
}

// ==================== Task 3：合约/视图/原始交易家族（13 个）====================
// 字段 json tag 逐字镜像 handler 侧 request struct（唯一事实源），注释标注来源行号。
// 注意：handler 侧的 json.RawMessage 字段（signatureMode 等）这里镜像为 any——
// 实验证明 SDK 的 schema 推断会把 json.RawMessage（[]byte）判成 "null|array"，
// 客户端传 JSON 对象会被校验拒绝；any 的 wire 序列化与 RawMessage 完全等价
// （原始 JSON 值原样透传给 REST handler），且 Task 1 的 faucetClaimArgs 已有先例。

// signerEntry 镜像 types.SignerEntry（types/request.go:17，multi_signer 模式的签名者）。
// address/signatureMode 保持 required（镜像 handler 的 binding:"required"）；
// privateKey 仅写交易需要（模拟走模拟签名），标 omitempty 允许省略。
type signerEntry struct {
	Address       string `json:"address" jsonschema:"签名者地址"`
	PrivateKey    string `json:"privateKey,omitempty" jsonschema:"签名者私钥（写交易必填；模拟走模拟签名可省）"`
	SignatureMode any    `json:"signatureMode" jsonschema:"签名模式对象：公钥模式 {\"type\":\"pubkey\",\"publicKey\":\"0x..\"} 或多签 {\"type\":\"multisig\",\"publicKey\":\"..\",\"index\":N}"`
}

// contractReadArgs 镜像 readContractRequest（handler/contract.go:43，POST /api/read）。
type contractReadArgs struct {
	AppName      string         `json:"appName" jsonschema:"IDL 里的 app 名，先用 idl_metadata 查"`
	MethodName   string         `json:"methodName" jsonschema:"IDL 里的方法名"`
	Args         map[string]any `json:"args,omitempty" jsonschema:"方法参数对象（键名见 idl_metadata）"`
	PayerAddress string         `json:"payerAddress,omitempty"`
}

// readContractMultiItemArgs 镜像 readContractMultiItem（handler/contract.go:107）。
type readContractMultiItemArgs struct {
	AppName    string         `json:"appName"`
	MethodName string         `json:"methodName"`
	Args       map[string]any `json:"args,omitempty"`
}

// contractReadMultiArgs 镜像 readContractMultiRequest（handler/contract.go:103，POST /api/read/multi）。
type contractReadMultiArgs struct {
	Instructions []readContractMultiItemArgs `json:"instructions"`
}

// contractSimulateArgs 镜像 simulateContractRequest（handler/contract.go:165，POST /api/simulate）。
type contractSimulateArgs struct {
	AppName         string         `json:"appName"`
	MethodName      string         `json:"methodName"`
	Args            map[string]any `json:"args,omitempty"`
	PaymentMode     string         `json:"paymentMode" jsonschema:"unified_payer_all/unified_dual_sign/unified_payer_only_gas/split/multi_signer/sponsored"`
	PayerAddress    string         `json:"payerAddress,omitempty"`
	SignatureMode   any            `json:"signatureMode,omitempty" jsonschema:"签名模式对象（如 {\"type\":\"pubkey\",\"publicKey\":\"0x..\"}）；缺省公钥模式"`
	IxAddress       string         `json:"ixAddress,omitempty" jsonschema:"unified_dual_sign 专用：指令执行账户地址"`
	IxSignatureMode any            `json:"ixSignatureMode,omitempty"`
	OwnerAddress    string         `json:"ownerAddress,omitempty" jsonschema:"split 专用：owner 地址，缺省取 payerAddress"`
	Signers         []signerEntry  `json:"signers,omitempty" jsonschema:"multi_signer 专用：签名者列表"`
	GasPayer        *signerEntry   `json:"gasPayer,omitempty" jsonschema:"multi_signer 可选：独立 gas 代付账户"`
}

// multiInstructionItemArgs 镜像 multiInstructionItem（handler/contract.go:1125）。
type multiInstructionItemArgs struct {
	AppName    string         `json:"appName"`
	MethodName string         `json:"methodName"`
	Args       map[string]any `json:"args,omitempty"`
}

// contractMultiArgs 镜像 multiContractRequest（handler/contract.go:1132，
// POST /api/simulate/multi 与 /api/write/multi 共用）。
type contractMultiArgs struct {
	Instructions    []multiInstructionItemArgs `json:"instructions"`
	PaymentMode     string                     `json:"paymentMode" jsonschema:"unified_payer_all/unified_dual_sign/unified_payer_only_gas/split/multi_signer/sponsored"`
	PayerPrivateKey string                     `json:"payerPrivateKey,omitempty"`
	PayerAddress    string                     `json:"payerAddress,omitempty"`
	SignatureMode   any                        `json:"signatureMode,omitempty"`
	IxAddress       string                     `json:"ixAddress,omitempty" jsonschema:"unified_dual_sign 专用：指令执行账户"`
	IxPrivateKey    string                     `json:"ixPrivateKey,omitempty"`
	IxSignatureMode any                        `json:"ixSignatureMode,omitempty"`
	OwnerPrivateKey string                     `json:"ownerPrivateKey,omitempty" jsonschema:"split 专用：owner 私钥，缺省取 payerPrivateKey"`
	OwnerAddress    string                     `json:"ownerAddress,omitempty"`
	Signers         []signerEntry              `json:"signers,omitempty"`
	GasPayer        *signerEntry               `json:"gasPayer,omitempty"`
}

// contractWriteArgs 镜像 writeContractRequest（handler/contract.go:789，
// POST /api/write、/api/write/multi-agent（WriteContractMultiAgent:845）、
// /api/write/multisig（WriteContractMultisig:881）共用）。
type contractWriteArgs struct {
	AppName         string         `json:"appName"`
	MethodName      string         `json:"methodName"`
	Args            map[string]any `json:"args,omitempty"`
	PaymentMode     string         `json:"paymentMode" jsonschema:"unified_payer_all/unified_dual_sign/unified_payer_only_gas/split/multi_signer/sponsored"`
	PayerPrivateKey string         `json:"payerPrivateKey,omitempty"`
	PayerAddress    string         `json:"payerAddress,omitempty"`
	SignatureMode   any            `json:"signatureMode,omitempty"`
	IxAddress       string         `json:"ixAddress,omitempty" jsonschema:"unified_dual_sign 专用：指令执行账户"`
	IxPrivateKey    string         `json:"ixPrivateKey,omitempty"`
	IxSignatureMode any            `json:"ixSignatureMode,omitempty"`
	OwnerPrivateKey string         `json:"ownerPrivateKey,omitempty" jsonschema:"split 专用：owner 私钥，缺省取 payerPrivateKey"`
	OwnerAddress    string         `json:"ownerAddress,omitempty" jsonschema:"split 专用：owner 地址，缺省取 payerAddress"`
	Signers         []signerEntry  `json:"signers,omitempty" jsonschema:"multi_signer 专用：签名者列表"`
	GasPayer        *signerEntry   `json:"gasPayer,omitempty" jsonschema:"multi_signer 可选：独立 gas 代付账户"`
}

// rawTransactionArgs 镜像 rawTransactionRequest（handler/transaction_handler.go:293，
// POST /api/transactions/simulate、/submit（SubmitTransaction:348）、
// /inspect（InspectTransaction:399）共用）。
type rawTransactionArgs struct {
	TransactionPostcard string `json:"transactionPostcard" jsonschema:"base64 编码的交易 postcard 原文"`
}

// rawViewArgs 镜像 rawViewRequest（handler/view_handler.go:48，
// POST /api/view/single 与 /api/view/multi（ViewMulti:93）共用）。
type rawViewArgs struct {
	TransactionPostcard string `json:"transactionPostcard" jsonschema:"base64 编码的视图 postcard 原文（单条或多条 wire 打包）"`
}

// ==================== Task 4：密钥与签名工具（5 个）====================
// 字段 json tag 逐字镜像 handler 侧 request struct（唯一事实源），注释标注来源行号。
// 已逐个核对 5 个 handler（DeriveAddress/DerivePublicKey/SignMessage/
// VerifySignature/GenerateVcAttestation）：全部只读 JSON body，
// 无 c.Query/c.Param，故 RESTMapping 均不带 PathParams/QueryParams。
// 本批 handler 无 json.RawMessage 字段，不涉及 any 镜像规则。

// utilDeriveAddressArgs 镜像 deriveAddressRequest（handler/util.go:25，
// POST /api/util/address/derive）。publicKey 必填（handler 空值 400）；
// keyType 可选（缺省按公钥自身曲线推断），标 omitempty。
type utilDeriveAddressArgs struct {
	PublicKey string `json:"publicKey" jsonschema:"公钥（hex 或 base58）"`
	KeyType   string `json:"keyType,omitempty" jsonschema:"可选：secp256k1/ed25519/fndsa512，缺省按公钥自身曲线推断"`
}

// utilDerivePublicKeyArgs 镜像 derivePublicKeyRequest（handler/util.go:81，
// POST /api/util/key/derive-public）。两字段均被 handler 强校验非空，必填不标 omitempty。
type utilDerivePublicKeyArgs struct {
	PrivateKey string `json:"privateKey" jsonschema:"32 字节私钥（hex 或 base58）"`
	KeyType    string `json:"keyType" jsonschema:"必填：secp256k1/ed25519/bls12381（fndsa512 无法从私钥派生公钥，不支持——公钥用 account_generate 的返回）"`
}

// utilSignArgs 镜像 signMessageRequest（handler/util.go:138，POST /api/util/sign）。
// 三字段均被 handler 强校验非空；服务端还需 ENABLE_UTIL_SIGN 开启，否则 403。
type utilSignArgs struct {
	PrivateKey string `json:"privateKey" jsonschema:"签名私钥（hex 或 base58）"`
	Message    string `json:"message" jsonschema:"被签消息：优先按 hex 解码，失败则按 UTF-8 文本"`
	KeyType    string `json:"keyType" jsonschema:"必填：secp256k1/ed25519/bls12381/fndsa512"`
	PublicKey  string `json:"publicKey,omitempty" jsonschema:"fndsa512 时必填（897 字节公钥，Go 无法从私钥派生；account_generate 返回里有）"`
}

// utilVerifyArgs 镜像 verifySignatureRequest（handler/util.go:214，POST /api/util/verify）。
// 三字段均被 handler 强校验非空。
type utilVerifyArgs struct {
	PublicKey string `json:"publicKey" jsonschema:"公钥（hex 或 base58）"`
	Message   string `json:"message" jsonschema:"验签消息（解码规则与 util_sign 一致：优先 hex，失败按 UTF-8）"`
	Signature string `json:"signature" jsonschema:"签名 hex"`
}

// vcAttestationArgs 镜像 generateVcAttestationRequest（handler/vc_attestation_handler.go:39，
// POST /api/util/vc-attestation）。issuerPrivateKey 必填；subjectPrivateKey 与
// subjectAddress 二选一（同时传以 subjectAddress 为准）；其余可选（handler 均有缺省）。
// 指针字段 chainId/issuerKeyId/validUntilMs 镜像 handler 的 *int64/*int 语义：
// 不传 = 缺省；显式传 0 有业务含义（validUntilMs=0 表示不过期）——omitempty 只吞
// nil 指针、不吞指向 0 的指针，0 值照常透传，与 handler 的判 nil 逻辑一致。
type vcAttestationArgs struct {
	IssuerPrivateKey  string `json:"issuerPrivateKey" jsonschema:"必填：issuer 私钥（32 字节 Ed25519 或 1281 字节 FN-DSA-512，hex/base58）"`
	IssuerPublicKey   string `json:"issuerPublicKey,omitempty" jsonschema:"issuer 为 FN-DSA-512 密钥时必填（897 字节公钥，hex/base58）"`
	ChainID           *int64 `json:"chainId,omitempty" jsonschema:"可选：链 ID，缺省 900000001"`
	SubjectPrivateKey string `json:"subjectPrivateKey,omitempty" jsonschema:"subject 私钥（hex）——与 subjectAddress 二选一"`
	SubjectAddress    string `json:"subjectAddress,omitempty" jsonschema:"subject 地址（bs58，20 字节）——与 subjectPrivateKey 二选一，同时传以本字段为准"`
	IssuerKeyID       *int   `json:"issuerKeyId,omitempty" jsonschema:"可选：issuer 密钥索引，缺省 0"`
	CredentialSchema  string `json:"credentialSchema,omitempty" jsonschema:"可选：凭证 schema，缺省 KycLevelCredential"`
	CredentialJson    string `json:"credentialJson,omitempty" jsonschema:"凭证规范化 JSON 字符串（sha256 作为 credential_hash；缺省用内置示例凭证）"`
	ValidUntilMs      *int64 `json:"validUntilMs,omitempty" jsonschema:"可选：有效期 13 位毫秒时间戳；0 表示不过期（与 validUntil 二选一，显式传入优先）"`
	ValidUntil        string `json:"validUntil,omitempty" jsonschema:"可选：有效期 ISO8601（如 2027-08-24T00:00:00.000Z），与 validUntilMs 二选一"`
	CredentialName    string `json:"credentialName,omitempty" jsonschema:"可选：凭证展示名称，缺省 KycLevel Credential"`
	CredentialDesc    string `json:"credentialDesc,omitempty" jsonschema:"可选：凭证描述"`
	IssuedAt          string `json:"issuedAt,omitempty" jsonschema:"可选：签发时间 ISO8601，缺省当前 UTC"`
}

// ==================== Task 5：DID 全生命周期（12 个）+ 保存指令（6 个）====================
// 字段 json tag 逐字镜像 handler 侧 request struct（唯一事实源），注释标注来源行号。
// 已逐个核对 12+6 个 handler：
//   - DID 细粒度端点（SetAlias/AddService/.../Deactivate）均为 POST 纯 JSON body，
//     公共字段经匿名嵌入 didMutateBase（handler/did_handler.go:60）绑定；
//   - Document 读 c.Param("address")（did_handler.go:804）、NameBinding 读
//     c.Query("name")（did_handler.go:835），分别进 PathParams/QueryParams；
//   - 保存指令：list 不读任何参数（saved_instruction_handler.go:278-281）、
//     get 读 c.Param("id")（:285）、update 读 c.Param("id") + JSON body
//     （updateSavedInstructionRequest:195，全字段 nil-able 部分更新）、
//     delete 只读 c.Param("id")（:364，无 body）、execute 读 c.Param("id") +
//     c.DefaultQuery "mode"（:395）/"wait"（:515）且不读 body。
// handler 侧 json.RawMessage 字段（signatureMode 等）按 Task 3 规则镜像为 any。

// didCreateArgs 镜像 didCreateRequest（handler/did_handler.go:48，POST /api/tool/did/create）。
// privateKey/address 被 validateDidCreateRequest（:188-194）校验必填，不标 omitempty；
// services 元素结构为 {label, serviceEndpoint}（didServiceSpec，did_handler.go:42），
// 按任务简报镜像为 []any 透传；suffix 为 *uint32 可选指针（覆盖服务端代填后缀）。
type didCreateArgs struct {
	PrivateKey  string  `json:"privateKey" jsonschema:"必填：私钥（hex 或 base58）"`
	PublicKey   string  `json:"publicKey,omitempty" jsonschema:"FN-DSA-512 私钥时必填（897 字节公钥）"`
	Address     string  `json:"address" jsonschema:"必填：32 字节私钥不同曲线派生不同地址，传 account_generate 返回的地址"`
	SubjectType string  `json:"subjectType,omitempty" jsonschema:"可选：Personal（缺省）/Organization"`
	Alias       string  `json:"alias,omitempty" jsonschema:"可选：非空时创建即绑定别名；「alias-数字」格式且全局唯一"`
	Suffix      *uint32 `json:"suffix,omitempty" jsonschema:"可选：覆盖服务端代填的数字后缀"`
	Services    []any   `json:"services,omitempty" jsonschema:"可选：创建时一并登记的服务端点，元素形如 {\"label\":\"...\",\"serviceEndpoint\":\"https://...\"}"`
	AvatarURI   string  `json:"avatarUri,omitempty" jsonschema:"可选：头像 URI；链上已有 DID 时仅在显式传入且不同才更新"`
}

// didMutateArgs 镜像 didMutateBase（handler/did_handler.go:60）：DID 细粒度
// 管理端点的公共身份字段。privateKey/address 被 bindMutate（:881-889）校验必填；
// publicKey 仅 FN-DSA-512 私钥时需要。did_deactivate 直接以本结构为入参。
type didMutateArgs struct {
	PrivateKey string `json:"privateKey" jsonschema:"必填：私钥（hex 或 base58）"`
	PublicKey  string `json:"publicKey,omitempty" jsonschema:"FN-DSA-512 私钥时必填（897 字节公钥）"`
	Address    string `json:"address" jsonschema:"必填：32 字节私钥在不同曲线下派生不同地址，传账户生成时返回的地址"`
}

// didSetAliasArgs 镜像 SetAlias 的匿名 request（handler/did_handler.go:465）。
// alias 被 handler（:473-477）校验必填；suffix 可选指针（缺省服务端代填并自动换号重试）。
type didSetAliasArgs struct {
	didMutateArgs
	Alias  string  `json:"alias" jsonschema:"必填：别名主体；最终别名为「alias-数字」格式"`
	Suffix *uint32 `json:"suffix,omitempty" jsonschema:"可选：数字后缀；缺省服务端代填，撞名自动换号重试"`
}

// didAddServiceArgs 镜像 AddService 的匿名 request（handler/did_handler.go:502）。
// label/serviceEndpoint 均 handler 校验必填（:514-517，链端错误 1047/1048）。
type didAddServiceArgs struct {
	didMutateArgs
	Label           string `json:"label" jsonschema:"必填：服务标签"`
	ServiceEndpoint string `json:"serviceEndpoint" jsonschema:"必填：服务端点绝对 URI（如 https://example.com）"`
}

// didUpdateServiceArgs 镜像 UpdateService 的匿名 request（handler/did_handler.go:540）。
// id 必填（handler :549 校验 nil 400），指针类型保持（0 是合法 service id）。
type didUpdateServiceArgs struct {
	didMutateArgs
	Id              *uint8 `json:"id" jsonschema:"必填：服务 id（从 did_document 的 services 列表获取）"`
	Label           string `json:"label" jsonschema:"必填：服务标签"`
	ServiceEndpoint string `json:"serviceEndpoint" jsonschema:"必填：服务端点绝对 URI"`
}

// didRemoveServiceArgs 镜像 RemoveService 的匿名 request（handler/did_handler.go:584）。
type didRemoveServiceArgs struct {
	didMutateArgs
	Id *uint8 `json:"id" jsonschema:"必填：服务 id（从 did_document 获取）"`
}

// didSetAvatarUriArgs 镜像 SetAvatarUri 的匿名 request（handler/did_handler.go:619）。
// avatarUri 被 handler（:626-630）校验必填（链端要求长度 1-512 字节，错误 1045）。
type didSetAvatarUriArgs struct {
	didMutateArgs
	AvatarURI string `json:"avatarUri" jsonschema:"必填：头像 URI（1-512 字节）"`
}

// didAddKeyArgs 镜像 AddKey 的匿名 request（handler/did_handler.go:655）。
// newPublicKey 必填（:663-667）；label 可选指针（缺省 = 链端 option<String>::None）。
type didAddKeyArgs struct {
	didMutateArgs
	NewPublicKey string  `json:"newPublicKey" jsonschema:"必填：要加入文档的公钥（base58）"`
	Label        *string `json:"label,omitempty" jsonschema:"可选：密钥标签；缺省不设"`
}

// didUpdateKeyArgs 镜像 UpdateKey 的匿名 request（handler/did_handler.go:698）。
// id 与 newPublicKey 均 handler 校验必填（:708-711）。
type didUpdateKeyArgs struct {
	didMutateArgs
	Id           *uint8  `json:"id" jsonschema:"必填：密钥 id（从 did_document 的 keys 列表获取）"`
	NewPublicKey string  `json:"newPublicKey" jsonschema:"必填：新公钥（base58）"`
	Label        *string `json:"label,omitempty" jsonschema:"可选：密钥标签"`
}

// didRemoveKeyArgs 镜像 RemoveKey 的匿名 request（handler/did_handler.go:742）。
type didRemoveKeyArgs struct {
	didMutateArgs
	Id *uint8 `json:"id" jsonschema:"必填：密钥 id（最后一把密钥链端拒绝，错误 1042）"`
}

// didNameBindingArgs 镜像 NameBinding 的 query 参数（handler/did_handler.go:835
// 读 c.Query("name")，空值 400）。
type didNameBindingArgs struct {
	Name string `json:"name" jsonschema:"必填：完整别名，「alias-数字」格式，如 alice-1024"`
}

// savedInstructionCreateArgs 镜像 createSavedInstructionRequest
// （handler/saved_instruction_handler.go:176，POST /api/saved-instructions）。
// name/appName/methodName 带 binding:"required"，不标 omitempty；
// 其余可选（entry 方法保存时 paymentMode 缺省 unified_payer_all）。
// signatureMode/ixSignatureMode 为 handler 侧 json.RawMessage，按规则镜像为 any。
type savedInstructionCreateArgs struct {
	Name            string         `json:"name" jsonschema:"必填：展示名称"`
	Description     string         `json:"description,omitempty"`
	AppName         string         `json:"appName" jsonschema:"必填：IDL app 名（先用 idl_metadata 查）"`
	MethodName      string         `json:"methodName" jsonschema:"必填：IDL 方法名"`
	Args            map[string]any `json:"args,omitempty" jsonschema:"方法参数对象（键名见 idl_metadata）"`
	PaymentMode     string         `json:"paymentMode,omitempty" jsonschema:"unified_payer_all/unified_dual_sign/unified_payer_only_gas/split/multi_signer/sponsored；entry 缺省 unified_payer_all"`
	PayerAddress    string         `json:"payerAddress,omitempty"`
	PayerPrivateKey string         `json:"payerPrivateKey,omitempty"`
	SignatureMode   any            `json:"signatureMode,omitempty" jsonschema:"签名模式对象（如 {\"type\":\"pubkey\",\"publicKey\":\"0x..\"}）"`
	IxAddress       string         `json:"ixAddress,omitempty" jsonschema:"unified_dual_sign 专用：指令执行账户地址"`
	IxPrivateKey    string         `json:"ixPrivateKey,omitempty" jsonschema:"unified_dual_sign 专用：指令执行账户私钥"`
	IxSignatureMode any            `json:"ixSignatureMode,omitempty"`
	OwnerAddress    string         `json:"ownerAddress,omitempty" jsonschema:"split 专用：owner 地址，缺省取 payerAddress"`
	OwnerPrivateKey string         `json:"ownerPrivateKey,omitempty" jsonschema:"split 专用：owner 私钥，缺省取 payerPrivateKey"`
	Signers         []signerEntry  `json:"signers,omitempty" jsonschema:"multi_signer 专用：签名者列表"`
	GasPayer        *signerEntry   `json:"gasPayer,omitempty" jsonschema:"multi_signer 可选：独立 gas 代付账户"`
}

// savedInstructionUpdateArgs 镜像 updateSavedInstructionRequest
// （handler/saved_instruction_handler.go:195，PUT /api/saved-instructions/{id}）。
// 部分更新契约：handler 侧全字段 nil-able（*string 指针 / map / RawMessage /
// slice / 指针元素）且无 binding required——不传 = 保持原值，镜像侧全标
// omitempty 保持同样的可选语义（schema required 集只含路径参数 id）。
// handler 不支持改 appName/methodName/kind（需换方法请删除后重建），
// 故镜像不含这三字段。signatureMode/ixSignatureMode 为 handler 侧
// json.RawMessage，按规则镜像为 any。
type savedInstructionUpdateArgs struct {
	Id              string         `json:"id" jsonschema:"必填：保存指令 ID"`
	Name            *string        `json:"name,omitempty" jsonschema:"可选：新的展示名称；不传保持不变"`
	Description     *string        `json:"description,omitempty" jsonschema:"可选：新的描述；不传保持不变"`
	Args            map[string]any `json:"args,omitempty" jsonschema:"可选：新的方法参数对象；不传保持不变"`
	PaymentMode     *string        `json:"paymentMode,omitempty" jsonschema:"可选：unified_payer_all/unified_dual_sign/unified_payer_only_gas/split/multi_signer/sponsored"`
	PayerAddress    *string        `json:"payerAddress,omitempty"`
	PayerPrivateKey *string        `json:"payerPrivateKey,omitempty"`
	SignatureMode   any            `json:"signatureMode,omitempty" jsonschema:"可选：签名模式对象（如 {\"type\":\"pubkey\",\"publicKey\":\"0x..\"}）"`
	IxAddress       *string        `json:"ixAddress,omitempty" jsonschema:"可选：unified_dual_sign 专用：指令执行账户地址"`
	IxPrivateKey    *string        `json:"ixPrivateKey,omitempty" jsonschema:"可选：unified_dual_sign 专用：指令执行账户私钥"`
	IxSignatureMode any            `json:"ixSignatureMode,omitempty"`
	OwnerAddress    *string        `json:"ownerAddress,omitempty" jsonschema:"可选：split 专用：owner 地址"`
	OwnerPrivateKey *string        `json:"ownerPrivateKey,omitempty" jsonschema:"可选：split 专用：owner 私钥"`
	Signers         []signerEntry  `json:"signers,omitempty" jsonschema:"可选：multi_signer 专用：签名者列表"`
	GasPayer        *signerEntry   `json:"gasPayer,omitempty" jsonschema:"可选：multi_signer 专用：独立 gas 代付账户"`
}

// savedInstructionIdArgs 镜像 GetSavedInstruction 的路径参数
// （handler/saved_instruction_handler.go:285 读 c.Param("id")）。
type savedInstructionIdArgs struct {
	Id string `json:"id" jsonschema:"必填：保存指令 ID"`
}

// savedInstructionExecuteArgs 镜像 ExecuteSavedInstruction 的参数
// （handler/saved_instruction_handler.go:383：c.Param("id") + DefaultQuery
// "mode"（:395，缺省 auto：view→read、entry→simulate）/"wait"（:515，仅 send
// 模式读取，缺省 true））。handler 不读 body，mode/wait 渲染为 query。
type savedInstructionExecuteArgs struct {
	Id   string `json:"id" jsonschema:"必填：保存指令 ID"`
	Mode string `json:"mode,omitempty" jsonschema:"可选：read/simulate/send；缺省 auto（view→read、entry→simulate）"`
	Wait string `json:"wait,omitempty" jsonschema:"可选：send 模式是否等待确认（true/1 等待），缺省 true"`
}

// ==================== Task 6：高层 flow 工具（4 个）====================
// 字段 json tag 逐字镜像 handler 侧 request struct（唯一事实源），注释标注来源行号。
// 已逐个核对 3 个 handler：
//   - VcFlow（handler/vc_flow_handler.go:184）与 SftFlow（handler/sft_flow_handler.go:290）
//     均为 POST 纯 JSON body，无 c.Query/c.Param；
//   - GetBulkTransferStatus（handler/bulk_transfer_handler.go:196）读 c.Param("id")
//     进 PathParams，不读 query；BulkTransfer（:90）为 POST 纯 JSON body。
// 嵌套 options struct 同样逐字镜像（指针保持指针、json tag 一致）；
// 本批 handler 无 json.RawMessage 字段，不涉及 any 镜像规则。

// vcFlowServiceSpecArgs 镜像 didServiceSpec（handler/did_handler.go:42，
// vcFlowDidOptions.Services 的元素）。label/serviceEndpoint 在细粒度 AddService
// 端点被校验必填；嵌入可选上下文，标 omitempty 由 handler 统一校验。
type vcFlowServiceSpecArgs struct {
	Label           string `json:"label,omitempty" jsonschema:"服务标签"`
	ServiceEndpoint string `json:"serviceEndpoint,omitempty" jsonschema:"服务端点绝对 URI（如 https://example.com）"`
}

// vcFlowDidOptionsArgs 镜像 vcFlowDidOptions（handler/did_handler.go:76，
// vc-flow 透传给某一方 DID 创建的可选项）。
type vcFlowDidOptionsArgs struct {
	Alias     string                  `json:"alias,omitempty" jsonschema:"可选：DID 别名主体；缺省服务端按角色自动生成（org-/user- + 地址片段）"`
	Suffix    *uint32                 `json:"suffix,omitempty" jsonschema:"可选：覆盖服务端代填的数字后缀"`
	Services  []vcFlowServiceSpecArgs `json:"services,omitempty" jsonschema:"可选：创建时一并登记的服务端点，元素形如 {\"label\":\"...\",\"serviceEndpoint\":\"https://...\"}"`
	AvatarURI string                  `json:"avatarUri,omitempty" jsonschema:"可选：头像 URI"`
	AutoAlias *bool                   `json:"autoAlias,omitempty" jsonschema:"可选：缺省 true；false 且未给别名时不绑定"`
}

// vcFlowArgs 镜像 vcFlowRequest（handler/vc_flow_handler.go:43，
// POST /api/tool/vc-flow）。issuerPrivateKey/userPrivateKey/userAddress 被
// validateVcFlowRequest（:65-79）校验必填，不标 omitempty；其余可选
// （credentialCount<=0 缺省 5、上限 20；validUntilMs 为 *int64：null/0=永久，
// omitempty 只吞 nil 指针、不吞指向 0 的指针，0 值照常透传）。
type vcFlowArgs struct {
	IssuerPrivateKey string                `json:"issuerPrivateKey" jsonschema:"必填：颁发者私钥（hex 或 base58）；VC 凭证签名仅支持 Ed25519/FN-DSA-512"`
	IssuerPublicKey  string                `json:"issuerPublicKey,omitempty" jsonschema:"颁发者为 FN-DSA-512 私钥时必填（897 字节公钥）"`
	IssuerAddress    string                `json:"issuerAddress,omitempty" jsonschema:"可选；显式传入时须与私钥派生地址一致"`
	UserPrivateKey   string                `json:"userPrivateKey" jsonschema:"必填：个人用户私钥（hex 或 base58）"`
	UserPublicKey    string                `json:"userPublicKey,omitempty" jsonschema:"用户为 FN-DSA-512 私钥时必填"`
	UserAddress      string                `json:"userAddress" jsonschema:"必填：32 字节私钥在不同曲线下派生不同地址，传 account_generate 返回的地址"`
	CredentialPrefix string                `json:"credentialPrefix,omitempty" jsonschema:"可选：凭证 schema 前缀，缺省 Test"`
	CredentialCount  int                   `json:"credentialCount,omitempty" jsonschema:"可选：签发凭证张数，缺省 5，上限 20"`
	ValidUntilMs     *int64                `json:"validUntilMs,omitempty" jsonschema:"可选：凭证有效期 13 位毫秒时间戳（须为未来值）；null 或 0 表示永久"`
	IssuerDid        *vcFlowDidOptionsArgs `json:"issuerDid,omitempty" jsonschema:"可选：issuer DID 的别名/服务/头像透传"`
	UserDid          *vcFlowDidOptionsArgs `json:"userDid,omitempty" jsonschema:"可选：user DID 的别名/服务/头像透传"`
}

// sftFlowSftOptionsArgs 镜像 sftFlowSftOptions（handler/sft_flow_handler.go:59，
// SFT 资源账户选项）。
type sftFlowSftOptionsArgs struct {
	Address    string `json:"address,omitempty" jsonschema:"可选：已有 SFT 地址；链上已存在则跳过创建"`
	PrivateKey string `json:"privateKey,omitempty" jsonschema:"传 address 且链上不存在时必填：SFT 资源账户私钥（签名 create_sft，gas 由 owner 代付）"`
	PublicKey  string `json:"publicKey,omitempty" jsonschema:"SFT 资源账户为 FN-DSA-512 私钥时必填"`
}

// sftFlowMetadataArgs 镜像 sftFlowMetadata（handler/sft_flow_handler.go:66，
// IDL Metadata）。name/symbol 链端强制（创建新 SFT 时必填，validateSftFlowRequest:125-131
// 前置拦截；复用已有 SFT 时不校验），交由 handler 统一校验，镜像侧标 omitempty。
type sftFlowMetadataArgs struct {
	Name      string  `json:"name,omitempty" jsonschema:"SFT 名称；创建新 SFT 时必填（链端 1 到 128 字符）"`
	Symbol    string  `json:"symbol,omitempty" jsonschema:"SFT 符号；创建新 SFT 时必填（链端 1 到 32 字符）"`
	CoverUrl  string  `json:"coverUrl,omitempty" jsonschema:"可选：封面 URL"`
	Metadata  string  `json:"metadata,omitempty" jsonschema:"可选：自由元数据"`
	Attribute *string `json:"attribute,omitempty" jsonschema:"可选：属性（option<String>；缺省不设）"`
}

// sftFlowMetadataOverrideArgs 镜像 sftFlowMetadataOverride
// （handler/sft_flow_handler.go:76，IDL MetadataOverride，全可选指针；
// 未提供的字段动态继承 SFT metadata）。
type sftFlowMetadataOverrideArgs struct {
	Name      *string `json:"name,omitempty"`
	Symbol    *string `json:"symbol,omitempty"`
	CoverUrl  *string `json:"coverUrl,omitempty"`
	Metadata  *string `json:"metadata,omitempty"`
	Attribute *string `json:"attribute,omitempty"`
}

// sftFlowSlotOptionsArgs 镜像 sftFlowSlotOptions（handler/sft_flow_handler.go:85）。
// slotId=0（或缺省）表示新建 slot，无独立业务含义，标 omitempty；
// 与 metadata/isTransferable 互斥（validateSftFlowRequest:133-137）。
type sftFlowSlotOptionsArgs struct {
	SlotId         uint64                       `json:"slotId,omitempty" jsonschema:"复用已有 slot 的 id；缺省/0 = 新建 slot"`
	Metadata       *sftFlowMetadataOverrideArgs `json:"metadata,omitempty" jsonschema:"可选：仅新建 slot 时的元数据覆盖（未提供字段动态继承 SFT metadata）"`
	IsTransferable *bool                        `json:"isTransferable,omitempty" jsonschema:"可选：仅新建 slot 时生效，缺省 true"`
}

// sftFlowDistributionArgs 镜像 sftFlowDistribution（handler/sft_flow_handler.go:92，
// 一笔分发 = 一次 Mint）。to/amount 被 validateSftFlowRequest（:145-155）校验必填，
// 不标 omitempty；单次最多 20 笔（每笔一笔链上交易）。
type sftFlowDistributionArgs struct {
	To       string                       `json:"to" jsonschema:"必填：接收方地址"`
	Amount   uint64                       `json:"amount" jsonschema:"必填：铸出份额（正数），每笔铸出独立 token_id"`
	Metadata *sftFlowMetadataOverrideArgs `json:"metadata,omitempty" jsonschema:"可选：token 级元数据覆盖"`
}

// sftFlowMergeOptionsArgs 镜像 sftFlowMergeOptions（handler/sft_flow_handler.go:99）。
// 两字段都传（>0）才执行合并；0 = 跳过（与缺省等价，标 omitempty）。
type sftFlowMergeOptionsArgs struct {
	FromTokenId uint64 `json:"fromTokenId,omitempty" jsonschema:"合并源 token id（须持有份额）"`
	ToTokenId   uint64 `json:"toTokenId,omitempty" jsonschema:"合并目标 token id（同 slot）"`
}

// sftFlowTransferOptionsArgs 镜像 sftFlowTransferOptions
// （handler/sft_flow_handler.go:105）。tokenId/to 被 validateSftFlowRequest
// （:166-175）校验必填，不标 omitempty；amount 为 *uint64：nil=该 token 全额份额，
// omitempty 只吞 nil、不吞指向 0 的指针（显式 0 照常透传并被 handler 拒绝）。
type sftFlowTransferOptionsArgs struct {
	TokenId uint64  `json:"tokenId" jsonschema:"必填：转移的 token id"`
	To      string  `json:"to" jsonschema:"必填：接收方地址"`
	Amount  *uint64 `json:"amount,omitempty" jsonschema:"可选：转移份额；缺省 = 该 token 全额份额"`
}

// sftFlowArgs 镜像 sftFlowRequest（handler/sft_flow_handler.go:45，
// POST /api/tool/sft-flow）。ownerPrivateKey/ownerAddress 被
// validateSftFlowRequest（:116-121）校验必填，不标 omitempty；
// 步骤开关 = 参数存在性：不传某步的参数即跳过该步。
type sftFlowArgs struct {
	OwnerPrivateKey string                      `json:"ownerPrivateKey" jsonschema:"必填：owner 私钥（hex 或 base58）"`
	OwnerPublicKey  string                      `json:"ownerPublicKey,omitempty" jsonschema:"owner 为 FN-DSA-512 私钥时必填"`
	OwnerAddress    string                      `json:"ownerAddress" jsonschema:"必填：32 字节私钥在不同曲线下派生不同地址，传 account_generate 返回的地址"`
	Sft             *sftFlowSftOptionsArgs      `json:"sft,omitempty" jsonschema:"可选：SFT 资源账户；缺省服务端生成新 Ed25519 资源账户（私钥随响应返回）"`
	SftMetadata     *sftFlowMetadataArgs        `json:"sftMetadata,omitempty" jsonschema:"创建 SFT 时的元数据（新 SFT 时 name/symbol 必填）"`
	RoyaltyBps      uint16                      `json:"royaltyBps,omitempty" jsonschema:"可选：二级市场版税万分比，缺省 0"`
	Slot            *sftFlowSlotOptionsArgs     `json:"slot,omitempty" jsonschema:"可选：slot 步骤；缺省跳过"`
	Distributions   []sftFlowDistributionArgs   `json:"distributions,omitempty" jsonschema:"可选：分发列表（逐笔 Mint 直发，最多 20 笔；需要 slot）"`
	Merge           *sftFlowMergeOptionsArgs    `json:"merge,omitempty" jsonschema:"可选：合并（fromTokenId→toTokenId，仅合并自己持有的份额）"`
	Transfer        *sftFlowTransferOptionsArgs `json:"transfer,omitempty" jsonschema:"可选：转移（tokenId→to，amount 缺省全额）"`
}

// bulkTransferArgs 镜像 bulkTransferRequest（handler/bulk_transfer_handler.go:39，
// POST /api/tool/bulk-transfer）。count/toAddress 带 binding:"required"
// （count 限 1..=5000），不标 omitempty；concurrency 可选（缺省 16，上限 128）。
type bulkTransferArgs struct {
	Count       int    `json:"count" jsonschema:"必填：批量生成账户数（1 到 5000）"`
	ToAddress   string `json:"toAddress" jsonschema:"必填：归集目标地址（每账户领水 10000 MIL 后转出 9800，预留 200 gas）"`
	Concurrency int    `json:"concurrency,omitempty" jsonschema:"可选：并发数，缺省 16，上限 128"`
}

// bulkTransferStatusArgs 镜像 GetBulkTransferStatus 的路径参数
// （handler/bulk_transfer_handler.go:196 读 c.Param("id")，不读 query）。
type bulkTransferStatusArgs struct {
	Id string `json:"id" jsonschema:"必填：批量转账任务 ID（bulk_transfer 返回的 jobId）"`
}

// ==================== 工具清单导出（GET /api/mcp/tools 数据源）====================

// ToolInfo 是单个 MCP 工具的清单条目；由 registerTool 在注册时单点收集，
// 供前端「MCP 接入」页面渲染工具总览——新增工具自动进入清单，无需手动同步。
type ToolInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Group       string `json:"group"`
	Method      string `json:"method"`
	Path        string `json:"path"`
}

// inventory / inventorySeen 只在 RegisterTools（进程启动或测试）串行调用时
// 写入，无需加锁；同名去重保证多次构建 server 不重复累积。
var (
	inventory     []ToolInfo
	inventorySeen = map[string]bool{}
	inventoryOnce sync.Once
)

// ToolInventory 返回已注册工具的清单副本（按注册顺序）。若进程尚未构建过
// MCP server（例如仅以 REST 方式部署、/mcp 从未挂载），惰性构建一次以填充
// 清单——BuildServer/NewExecutor 均无副作用、不发网络请求。
func ToolInventory() []ToolInfo {
	inventoryOnce.Do(func() {
		if len(inventory) == 0 {
			BuildServer("http://127.0.0.1:1")
		}
	})
	out := make([]ToolInfo, len(inventory))
	copy(out, inventory)
	return out
}

// toolGroupOf 按工具名前缀推导展示分组；case 顺序敏感（_raw 与 flow 类
// 须先于通用 tx_/vc_ 前缀匹配）。未匹配返回「其他」兜底——新工具忘加
// 前缀只会归组变粗，不会从清单消失（有 TestToolInventory 空字段断言兜底）。
func toolGroupOf(name string) string {
	switch {
	case name == "vc_flow", name == "sft_flow", strings.HasPrefix(name, "bulk_transfer"):
		return "高层编排"
	case name == "error_lookup" || strings.HasPrefix(name, "error_"):
		return "错误参考"
	case strings.HasPrefix(name, "transfer_"):
		return "高层编排"
	case strings.HasPrefix(name, "saved_instruction_"):
		return "保存指令"
	case strings.HasPrefix(name, "did_"):
		return "DID"
	case strings.HasPrefix(name, "vc_"):
		return "VC 凭证"
	case strings.HasPrefix(name, "util_"):
		return "密钥与签名"
	case strings.HasPrefix(name, "contract_"), strings.HasPrefix(name, "view_"):
		return "合约调用"
	case strings.HasPrefix(name, "tx_") && strings.HasSuffix(name, "_raw"):
		return "原始交易"
	case strings.HasPrefix(name, "tx_"):
		return "交易查询"
	case strings.HasPrefix(name, "idl_"):
		return "IDL"
	case strings.HasPrefix(name, "rpc_"):
		return "RPC 底层"
	case strings.HasPrefix(name, "faucet_"):
		return "水龙头"
	case strings.HasPrefix(name, "account_"):
		return "账户"
	case strings.HasPrefix(name, "network_"):
		return "网络"
	default:
		return "其他"
	}
}

// registerTool 把"输入结构 → REST 映射"注册为 MCP 工具。
func registerTool[In any](srv *mcp.Server, exec *Executor, name, desc string, m RESTMapping) {
	if !inventorySeen[name] {
		inventorySeen[name] = true
		inventory = append(inventory, ToolInfo{
			Name:        name,
			Description: desc,
			Group:       toolGroupOf(name),
			Method:      m.Method,
			Path:        m.PathTemplate,
		})
	}
	mcp.AddTool[In, any](srv, &mcp.Tool{Name: name, Description: desc},
		func(ctx context.Context, req *mcp.CallToolRequest, args In) (*mcp.CallToolResult, any, error) {
			raw, err := json.Marshal(args)
			if err != nil {
				return nil, nil, err
			}
			out, err := exec.Call(ctx, m, raw)
			if err != nil {
				return nil, nil, err
			}
			return &mcp.CallToolResult{
				IsError: out.IsError,
				Content: []mcp.Content{&mcp.TextContent{Text: string(out.Body)}},
			}, nil, nil
		})
}

// BuildServer 构造注册了全部工具的 MCP Server。
// Task 7 的 server.go 会复用；当前先放 tools.go。
func BuildServer(baseURL string) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "milon-api-server", Version: "1.0"}, nil)
	RegisterTools(srv, NewExecutor(baseURL, nil))
	return srv
}

// NewMCPHandler 构造挂到 gin 的 /mcp handler；token 为空则不鉴权。
// 回环基地址优先取 MILON_REST_BASE_URL（测试注入），缺省 http://127.0.0.1:$SERVER_PORT。
func NewMCPHandler(token string) http.Handler {
	baseURL := os.Getenv("MILON_REST_BASE_URL")
	if baseURL == "" {
		port := os.Getenv("SERVER_PORT")
		if port == "" {
			port = "8080"
		}
		baseURL = "http://127.0.0.1:" + port
	}
	srv := BuildServer(baseURL)
	// Stateless：免 initialize、每请求临时会话；JSONResponse：POST 响应用
	// application/json 而非 SSE 流（协议 §2.1.5 允许，便于非流式客户端直读）。
	inner := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	// 请求级网络（Task 4）：X-Milon-Network 头注入 request context，工具回调
	// 的 ctx（go-sdk 从 r.Context() 传播）→ Executor.Call 回环透传同名头。
	// 注入层放最外（先注入再鉴权）：401 路径响应不受影响，行为不变。
	withNetwork := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if net := r.Header.Get("X-Milon-Network"); net != "" {
			r = r.WithContext(context.WithValue(r.Context(), ctxKeyNetwork{}, net))
		}
		inner.ServeHTTP(w, r)
	})
	if token == "" {
		return withNetwork
	}
	return bearerAuth(token, withNetwork)
}

// bearerAuth 校验 Authorization: Bearer <token> 后转发给 MCP handler。
// M-3（Task 2 审查遗留）：token 比较改用 subtle.ConstantTimeCompare，
// 避免逐字节短路比较的时序侧信道泄露正确 token 的前缀；长度不等时
// ConstantTimeCompare 直接返回 0（仅泄露长度，token 长度非秘密，可接受）。
func bearerAuth(token string, next http.Handler) http.Handler {
	want := []byte("Bearer " + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"code":401,"message":"invalid MCP token"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RegisterTools 注册全部工具；基础 17 个 + Task 3 合约/视图/原始交易 13 个 +
// Task 4 密钥/签名/VC 5 个 + Task 5 DID 12 个与保存指令 6 个 + Task 6 高层
// flow 工具 4 个（共 57 个）。
func RegisterTools(srv *mcp.Server, exec *Executor) {
	registerTool[emptyArgs](srv, exec, "network_list", "列出内置网络（devNet/localNet）及当前网络", RESTMapping{Method: "GET", PathTemplate: "/api/network/list"})
	registerTool[emptyArgs](srv, exec, "network_current", "查询当前网络", RESTMapping{Method: "GET", PathTemplate: "/api/network/current"})
	registerTool[networkSwitchArgs](srv, exec, "network_switch", "设置服务端默认网络（仅影响未携带 X-Milon-Network 头的请求；devNet/localNet）", RESTMapping{Method: "POST", PathTemplate: "/api/network/switch"})
	registerTool[accountGenerateArgs](srv, exec, "account_generate", "生成新账户（返回私钥/公钥/地址）", RESTMapping{Method: "POST", PathTemplate: "/api/accounts/generate"})
	registerTool[addressArgs](srv, exec, "account_info", "查询账户信息", RESTMapping{Method: "GET", PathTemplate: "/api/accounts/{address}", PathParams: []string{"address"}})
	registerTool[addressArgs](srv, exec, "account_resources", "查询账户链上资源", RESTMapping{Method: "GET", PathTemplate: "/api/accounts/{address}/resources", PathParams: []string{"address"}})
	registerTool[accountSummaryArgs](srv, exec, "account_summary",
		"地址全景聚合：MIL 余额 + 冻结量 + faucet 冷却剩余 + DID 文档一站返回；子项失败不整体失败（字段置 null 并在 errors 说明，DID 未创建属正常态记 did:null）",
		RESTMapping{Method: "GET", PathTemplate: "/api/accounts/{address}/summary", PathParams: []string{"address"}})
	registerTool[faucetClaimArgs](srv, exec, "faucet_claim", "从 faucet 领取代币（需私钥签名，等待确认后返回 txHash）", RESTMapping{Method: "POST", PathTemplate: "/api/faucet/claim"})
	registerTool[addressArgs](srv, exec, "faucet_balance", "查询账户余额（faucet 口径）", RESTMapping{Method: "GET", PathTemplate: "/api/faucet/balance/{address}", PathParams: []string{"address"}})
	registerTool[hashArgs](srv, exec, "tx_get", "按哈希查询交易", RESTMapping{Method: "GET", PathTemplate: "/api/transactions/{hash}", PathParams: []string{"hash"}})
	registerTool[txParseArgs](srv, exec, "tx_parse", "解析交易（结构化 postcard）", RESTMapping{Method: "GET", PathTemplate: "/api/transactions/{hash}/parse", PathParams: []string{"hash"}, QueryParams: []string{"remote"}})
	registerTool[txEventsArgs](srv, exec, "tx_events", "查询交易事件", RESTMapping{Method: "GET", PathTemplate: "/api/transactions/{hash}/events", PathParams: []string{"hash"}, QueryParams: []string{"typeTag"}})
	registerTool[txWaitArgs](srv, exec, "tx_wait", "等待交易确认（可能长轮询）", RESTMapping{Method: "GET", PathTemplate: "/api/transactions/{hash}/wait", PathParams: []string{"hash"}, QueryParams: []string{"timeoutSecs"}})
	registerTool[txTrackArgs](srv, exec, "tx_track",
		"交易全链路追踪（tx_get+tx_wait+tx_events 聚合）：查存在→等确认（缺省 60s）→返回 status（not_found/timeout/confirmed）+ 交易摘要 + 事件，AI 发完交易后的收尾一步到位",
		RESTMapping{Method: "GET", PathTemplate: "/api/transactions/{hash}/track", PathParams: []string{"hash"}, QueryParams: []string{"timeoutSecs"}})
	registerTool[heightArgs](srv, exec, "rpc_block", "按高度查区块头", RESTMapping{Method: "GET", PathTemplate: "/api/rpc/blocks/{height}", PathParams: []string{"height"}})
	registerTool[hashArgs](srv, exec, "rpc_resource", "按哈希查链上资源原文", RESTMapping{Method: "GET", PathTemplate: "/api/rpc/resources/{hash}", PathParams: []string{"hash"}})
	registerTool[accessValueArgs](srv, exec, "rpc_access_value", "按 blob 哈希批量取 access value", RESTMapping{Method: "POST", PathTemplate: "/api/rpc/access-value"})
	registerTool[hashArgs](srv, exec, "rpc_resource_path", "按哈希查资源路径", RESTMapping{Method: "GET", PathTemplate: "/api/rpc/resource-paths/{hash}", PathParams: []string{"hash"}})
	registerTool[emptyArgs](srv, exec, "idl_metadata", "IDL 元数据发现：列出全部 app/方法/参数/返回值/signer 角色（调用合约工具前的第一站）", RESTMapping{Method: "GET", PathTemplate: "/api/idl/metadata"})
	registerTool[emptyArgs](srv, exec, "idl_apps",
		"IDL app 轻量清单（名称/简介/方法数）——idl_metadata 全量约 200KB 易撑爆上下文，建议先调本工具看有什么，再按需调 idl_methods",
		RESTMapping{Method: "GET", PathTemplate: "/api/idl/apps"})
	registerTool[idlMethodsArgs](srv, exec, "idl_methods",
		"查单个 app 的全量方法明细（参数/返回值/signer 角色/错误码）；注意返回的 name 是 PascalCase（如 Transfer），而 contract_read/write 的 methodName 用 snake_case 的 handler 名（如 transfer）",
		RESTMapping{Method: "GET", PathTemplate: "/api/idl/apps/{appName}/methods", PathParams: []string{"appName"}})

	// ---- Task 3：合约/视图/原始交易家族（13 个，全部 POST 纯 JSON body，无 query/path 参数）----

	// 合约只读（view）
	registerTool[contractReadArgs](srv, exec, "contract_read", "只读合约调用（按 IDL 方法名查询链上状态）", RESTMapping{Method: "POST", PathTemplate: "/api/read"})
	registerTool[contractReadMultiArgs](srv, exec, "contract_read_multi", "批量只读合约调用（多指令打包单次查询）", RESTMapping{Method: "POST", PathTemplate: "/api/read/multi"})

	// 合约模拟（dry-run，不消耗 gas）
	registerTool[contractSimulateArgs](srv, exec, "contract_simulate", "模拟执行合约写调用（dry-run，模拟签名，无需私钥）", RESTMapping{Method: "POST", PathTemplate: "/api/simulate"})
	registerTool[contractMultiArgs](srv, exec, "contract_simulate_multi", "多指令打包模拟执行（单笔交易原子 dry-run）", RESTMapping{Method: "POST", PathTemplate: "/api/simulate/multi"})

	// 合约写（真实上链）
	registerTool[contractWriteArgs](srv, exec, "contract_write", "构建并提交合约写交易（按 paymentMode 签名，真实上链）", RESTMapping{Method: "POST", PathTemplate: "/api/write"})
	registerTool[contractMultiArgs](srv, exec, "contract_write_multi", "多指令打包写交易（单笔交易原子上链）", RESTMapping{Method: "POST", PathTemplate: "/api/write/multi"})
	registerTool[contractWriteArgs](srv, exec, "contract_write_multi_agent", "双账户写交易（unified_dual_sign：付 gas 与指令执行账户不同）", RESTMapping{Method: "POST", PathTemplate: "/api/write/multi-agent"})
	registerTool[contractWriteArgs](srv, exec, "contract_write_multisig", "split 模式写交易（owner 付 gas 并签指令）", RESTMapping{Method: "POST", PathTemplate: "/api/write/multisig"})
	registerTool[writeSafeArgs](srv, exec, "contract_write_safe",
		"安全版合约写：先模拟（不消耗 gas）再真签提交——模拟失败返回 stage=simulate_failed 并阻止上链，模拟通过才走与 contract_write 完全相同的提交路径；请求参数与 contract_write 一致",
		RESTMapping{Method: "POST", PathTemplate: "/api/write-safe"})

	// 原始 postcard 交易（simulate/submit/inspect 共用 rawTransactionRequest）
	registerTool[rawTransactionArgs](srv, exec, "tx_simulate_raw", "模拟执行 postcard 原始交易（不消耗 gas）", RESTMapping{Method: "POST", PathTemplate: "/api/transactions/simulate"})
	registerTool[rawTransactionArgs](srv, exec, "tx_submit_raw", "提交 postcard 原始交易上链", RESTMapping{Method: "POST", PathTemplate: "/api/transactions/submit"})
	registerTool[rawTransactionArgs](srv, exec, "tx_inspect_raw", "解析 postcard 原始交易（返回 txHash/ixHashes/payer/valid，不上链）", RESTMapping{Method: "POST", PathTemplate: "/api/transactions/inspect"})

	// 低层视图（预构建 postcard）
	registerTool[rawViewArgs](srv, exec, "view_single", "低层视图调用（预构建 postcard wire）", RESTMapping{Method: "POST", PathTemplate: "/api/view/single"})
	registerTool[rawViewArgs](srv, exec, "view_multi", "低层批量视图调用（预构建 postcard 多 wire 打包）", RESTMapping{Method: "POST", PathTemplate: "/api/view/multi"})

	// ---- Task 4：密钥与签名（5 个，全部 POST 纯 JSON body，无 query/path 参数）----

	// 密钥/地址派生
	registerTool[utilDeriveAddressArgs](srv, exec, "util_derive_address",
		"由公钥派生链上地址（bs58）。注意：32 字节私钥在 secp256k1/ed25519/fn-dsa-512 下派生出不同地址（fndsa512 完整私钥为 1281 字节，不适用本规则），地址计算必须用本工具（或 account_generate 的返回），不要本地臆造；keyType 可选，缺省按公钥自身曲线推断",
		RESTMapping{Method: "POST", PathTemplate: "/api/util/address/derive"})
	registerTool[utilDerivePublicKeyArgs](srv, exec, "util_derive_public_key",
		"由 32 字节私钥按指定曲线派生公钥（keyType 必填：secp256k1/ed25519/bls12381；fndsa512 无法从私钥派生公钥，不支持——公钥用 account_generate 的返回）",
		RESTMapping{Method: "POST", PathTemplate: "/api/util/key/derive-public"})

	// 签名/验签（util_sign 受服务端 ENABLE_UTIL_SIGN 开关控制）
	registerTool[utilSignArgs](srv, exec, "util_sign",
		"用私钥对消息签名，返回签名 hex 与公钥（keyType 必填；message 优先按 hex 解码、失败按 UTF-8 文本；服务端需开启 ENABLE_UTIL_SIGN，否则 403）",
		RESTMapping{Method: "POST", PathTemplate: "/api/util/sign"})
	registerTool[utilVerifyArgs](srv, exec, "util_verify",
		"验证签名是否匹配指定公钥与消息（signature 为 hex，message 解码规则与 util_sign 一致）",
		RESTMapping{Method: "POST", PathTemplate: "/api/util/verify"})

	// VC 凭证披露
	registerTool[vcAttestationArgs](srv, exec, "vc_attestation",
		"生成 DiscloseVcAttestation 可验证凭证披露文档（milon-vc-disclosure 裸 JSON：issuer 私钥签名并本地验签，输出可直接用于链上披露；issuer 为 FN-DSA-512 密钥时须同时提供 issuerPublicKey）",
		RESTMapping{Method: "POST", PathTemplate: "/api/util/vc-attestation"})

	// ---- Task 5：DID 全生命周期（12 个）----

	// 创建（幂等聚合：链上已有则按请求补齐差异，可直接重跑）
	registerTool[didCreateArgs](srv, exec, "did_create",
		"创建 DID（幂等）：一步完成 创建（+可选别名）+服务+头像；链上已有 DID 时按请求补齐差异（缺别名 SetAlias/缺服务 AddService/头像不同 SetAvatarUri），不覆盖已有",
		RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/create"})

	// 别名
	registerTool[didSetAliasArgs](srv, exec, "did_set_alias",
		"为已有 DID 绑定/更换别名（「alias-数字」格式且全局唯一；suffix 缺省服务端代填，撞名自动换号重试；响应含最新 nameBinding）",
		RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/set-alias"})

	// 服务端点
	registerTool[didAddServiceArgs](srv, exec, "did_add_service",
		"向 DID 文档添加服务端点（label + 绝对 URI）；响应带最新文档，从中取链上分配的 service id",
		RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/add-service"})
	registerTool[didUpdateServiceArgs](srv, exec, "did_update_service",
		"更新 DID 文档中的服务端点（id 从 did_document 的 services 列表获取）",
		RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/update-service"})
	registerTool[didRemoveServiceArgs](srv, exec, "did_remove_service",
		"移除 DID 文档中的服务端点（id 从 did_document 获取）",
		RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/remove-service"})

	// 头像
	registerTool[didSetAvatarUriArgs](srv, exec, "did_set_avatar_uri",
		"设置 DID 头像 URI（长度 1-512 字节）；响应带最新文档",
		RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/set-avatar-uri"})

	// 密钥管理
	registerTool[didAddKeyArgs](srv, exec, "did_add_key",
		"向 DID 文档添加公钥（响应带最新文档，从中取链上分配的 key id）",
		RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/add-key"})
	registerTool[didUpdateKeyArgs](srv, exec, "did_update_key",
		"更新 DID 文档中的公钥（id 从 did_document 的 keys 列表获取）",
		RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/update-key"})
	registerTool[didRemoveKeyArgs](srv, exec, "did_remove_key",
		"移除 DID 文档中的公钥（最后一把密钥链端拒绝，错误 1042）",
		RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/remove-key"})

	// 停用
	registerTool[didMutateArgs](srv, exec, "did_deactivate",
		"停用 DID（不可逆；停用后 identity 写操作均被拒，错误 1026）",
		RESTMapping{Method: "POST", PathTemplate: "/api/tool/did/deactivate"})

	// 查询
	registerTool[didNameBindingArgs](srv, exec, "did_name_binding",
		"按别名反查 DID 绑定（完整别名「alias-数字」，如 alice-1024；未绑定时 404）",
		RESTMapping{Method: "GET", PathTemplate: "/api/tool/did/name-binding", QueryParams: []string{"name"}})
	registerTool[addressArgs](srv, exec, "did_document",
		"查询完整 DID 文档（含 keys/services/avatar/alias；未创建时 404）",
		RESTMapping{Method: "GET", PathTemplate: "/api/tool/did/{address}/document", PathParams: []string{"address"}})

	// ---- Task 5：保存指令（6 个）----

	registerTool[savedInstructionCreateArgs](srv, exec, "saved_instruction_create",
		"保存一条 IDL 方法调用（含账户/签名绑定）供后续反复执行；kind 由 IDL 自动检测（view/entry），entry 方法 paymentMode 缺省 unified_payer_all",
		RESTMapping{Method: "POST", PathTemplate: "/api/saved-instructions"})
	registerTool[emptyArgs](srv, exec, "saved_instruction_list",
		"列出全部保存的指令",
		RESTMapping{Method: "GET", PathTemplate: "/api/saved-instructions"})
	registerTool[savedInstructionIdArgs](srv, exec, "saved_instruction_get",
		"按 ID 查询单条保存的指令（含绑定的账户与签名信息）",
		RESTMapping{Method: "GET", PathTemplate: "/api/saved-instructions/{id}", PathParams: []string{"id"}})
	registerTool[savedInstructionUpdateArgs](srv, exec, "saved_instruction_update",
		"部分更新保存的指令：只改传入的字段，未传字段保持不变（如只换 name 或 args）；不支持改 appName/methodName/kind，需换方法请删除后重建",
		RESTMapping{Method: "PUT", PathTemplate: "/api/saved-instructions/{id}", PathParams: []string{"id"}})
	registerTool[savedInstructionIdArgs](srv, exec, "saved_instruction_delete",
		"按 ID 删除保存的指令（不可恢复）",
		RESTMapping{Method: "DELETE", PathTemplate: "/api/saved-instructions/{id}", PathParams: []string{"id"}})
	registerTool[savedInstructionExecuteArgs](srv, exec, "saved_instruction_execute",
		"执行保存的指令（mode 缺省 auto：view→read、entry→simulate；send 真实上链，可用 wait 控制是否等待确认）",
		RESTMapping{Method: "POST", PathTemplate: "/api/saved-instructions/{id}/execute", PathParams: []string{"id"}, QueryParams: []string{"mode", "wait"}})

	// ---- Task 6：高层 flow 工具（4 个）----

	// VC 全流程（服务端多步编排，同步执行；幂等可重跑）
	registerTool[vcFlowArgs](srv, exec, "vc_flow",
		"VC 签发全流程编排（服务端多步编排，一次调用完成）：双方领水 → issuer/user 创建 DID → issuer 注册组织（VcIssuer 角色 + schema 声明）→ 链下签发 N 张键值对凭证 → user 逐张披露上链 → 回读验证；每步幂等（DID/组织/凭证已存在自动跳过），可直接重跑",
		RESTMapping{Method: "POST", PathTemplate: "/api/tool/vc-flow"})

	// SFT 全流程（服务端多步编排，同步执行；步骤开关 = 参数存在性）
	registerTool[sftFlowArgs](srv, exec, "sft_flow",
		"SFT 全生命周期编排（服务端多步编排，一次调用完成）：owner 领水 → 创建/复用 SFT（缺省生成新资源账户并返回私钥）→ 创建/复用 slot → 逐笔 Mint 分发 → 合并 → 转移 → 回读验证；不传某步参数即跳过该步；注意分发/合并/转移是链上状态变更，重跑会重复生效",
		RESTMapping{Method: "POST", PathTemplate: "/api/tool/sft-flow"})

	// 批量转账（异步任务，返回 jobId 轮询）
	registerTool[bulkTransferArgs](srv, exec, "bulk_transfer",
		"批量生成账户 → 逐个领水 → 归集 MIL 到目标地址（异步任务：立即返回 jobId，用 bulk_transfer_status 轮询进度；每账户领水 10000 MIL 转出 9800、预留 200 gas）",
		RESTMapping{Method: "POST", PathTemplate: "/api/tool/bulk-transfer"})

	// 批量转账进度查询（GET + 路径参数 id）
	registerTool[errorLookupArgs](srv, exec, "error_lookup",
		"错误码翻译：按十进制错误码或名字子串查 API 层码表与全部 IDL app 的链上错误码（如 521=VcRequired / Cooldown=FaucetCooldownActive），返回来源(api/app:名)+码+名字+说明+处置建议",
		RESTMapping{Method: "GET", PathTemplate: "/api/errors/{query}", PathParams: []string{"query"}})
	registerTool[transferMilArgs](srv, exec, "transfer_mil",
		"MIL 转账快捷封装：只给 to/amount/privateKey，内部完成密钥派生地址 + 填充 token.Transfer 全部固定参数（MIL 代币地址/unified_payer_all/pubkey 签名模式自动修正），复用与 contract_write 完全相同的提交路径；返回 txHash，建议接着用 tx_track 跟踪确认",
		RESTMapping{Method: "POST", PathTemplate: "/api/tool/transfer-mil"})
	registerTool[bulkTransferStatusArgs](srv, exec, "bulk_transfer_status",
		"查询批量转账任务进度与结果（含 done/success/failed 计数、归集总额与逐账户明细；jobId 来自 bulk_transfer 的返回）",
		RESTMapping{Method: "GET", PathTemplate: "/api/tool/bulk-transfer/{id}", PathParams: []string{"id"}})
}
