package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"milon-api-server/client"
	"milon-api-server/types"

	"github.com/gin-gonic/gin"
	milon "github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/provider"
)

// ==================== SFT 全流程工具(POST /api/tool/sft-flow) ====================
//
// 给定 owner 私钥,自动完成 sftoken 全生命周期:
//
//	[1] owner 领水(余额充足则跳过;SFT 资源账户与接收者无需 gas)
//	[2] 创建 SFT(缺省服务端生成 Ed25519 资源账户并返回私钥;
//	    传 sft.address 且链上已存在则跳过,复用已有 SFT)
//	[3] 创建 slot(slot_id 链上递增分配,从 SlotCreatedEvent 提取;
//	    传 slot.slotId 则复用已有 slot,跳过创建)
//	[4] 分发:对 distributions 逐笔 Mint 直发,每笔 (to, amount) 铸出独立 token_id
//	[5] 合并:merge.from_token_id → merge.to_token_id(仅合并签名者自己持有的份额)
//	[6] 转移:transfer.token_id → transfer.to,amount 缺省为该 token 全额份额
//	[7] 回读验证(SftMetadata / SlotInfo / BalanceOf)
//
// 步骤开关 = 参数存在性:不传某步的参数即跳过该步。
// 幂等边界:create_sft 与 slot 复用可幂等重跑;分发/合并/转移是链上状态变更
// 操作,重复调用会重复生效,不具备「已存在即跳过」语义,重跑前须确认。

const (
	sftFlowIdentityApp       = "sftoken"
	sftFlowMaxDistributions  = 20 // 单次分发笔数上限(每笔一笔链上交易)
)

// sftFlowRequest 是 POST /api/tool/sft-flow 的请求体。
type sftFlowRequest struct {
	OwnerPrivateKey string                 `json:"ownerPrivateKey"` // owner 私钥,必填
	OwnerPublicKey  string                 `json:"ownerPublicKey"`  // owner 公钥;FN-DSA-512 私钥时必填
	OwnerAddress    string                 `json:"ownerAddress"`    // 必填;32字节私钥在不同曲线下派生不同地址,显式地址用于锁定正确公钥
	Sft             *sftFlowSftOptions     `json:"sft"`             // 缺省生成新 SFT 资源账户
	SftMetadata     *sftFlowMetadata       `json:"sftMetadata"`     // 创建 SFT 时的元数据(name/symbol 链端强制)
	RoyaltyBps      uint16                 `json:"royaltyBps"`      // 二级市场版税万分比,缺省 0
	Slot            *sftFlowSlotOptions    `json:"slot"`            // 缺省跳过 slot 步骤
	Distributions   []sftFlowDistribution  `json:"distributions"`   // 缺省不分发
	Merge           *sftFlowMergeOptions   `json:"merge"`           // 缺省不合并
	Transfer        *sftFlowTransferOptions `json:"transfer"`       // 缺省不转移
}

// sftFlowSftOptions SFT 资源账户选项。
type sftFlowSftOptions struct {
	Address    string `json:"address"`    // 已有 SFT 地址;链上已存在则跳过创建
	PrivateKey string `json:"privateKey"` // SFT 不存在时必填(签名 create_sft,gas 由 owner 代付)
	PublicKey  string `json:"publicKey"`  // FN-DSA-512 私钥时必填
}

// sftFlowMetadata SFT 创建元数据(IDL Metadata:name/symbol 链端强制)。
type sftFlowMetadata struct {
	Name      string  `json:"name"`
	Symbol    string  `json:"symbol"`
	CoverUrl  string  `json:"coverUrl"`
	Metadata  string  `json:"metadata"`
	Attribute *string `json:"attribute"` // option<String>
}

// sftFlowMetadataOverride slot/mint 的元数据覆盖(IDL MetadataOverride,全可选;
// 未提供的字段动态继承 SFT metadata)。
type sftFlowMetadataOverride struct {
	Name      *string `json:"name"`
	Symbol    *string `json:"symbol"`
	CoverUrl  *string `json:"coverUrl"`
	Metadata  *string `json:"metadata"`
	Attribute *string `json:"attribute"`
}

// sftFlowSlotOptions slot 步骤选项:slotId>0 复用已有 slot,否则创建新 slot。
type sftFlowSlotOptions struct {
	SlotId         uint64                   `json:"slotId"`
	Metadata       *sftFlowMetadataOverride `json:"metadata"`       // 仅创建时生效
	IsTransferable *bool                    `json:"isTransferable"` // 仅创建时生效,缺省 true
}

// sftFlowDistribution 一笔分发 = 一次 Mint(to 铸出 amount 份额,独立 token_id)。
type sftFlowDistribution struct {
	To       string                   `json:"to"`
	Amount   uint64                   `json:"amount"`
	Metadata *sftFlowMetadataOverride `json:"metadata"` // token 级元数据覆盖
}

// sftFlowMergeOptions 合并选项:两字段都传才执行。
type sftFlowMergeOptions struct {
	FromTokenId uint64 `json:"fromTokenId"`
	ToTokenId   uint64 `json:"toTokenId"`
}

// sftFlowTransferOptions 转移选项:tokenId+to 齐备才执行;Amount 缺省 = 全额。
type sftFlowTransferOptions struct {
	TokenId uint64  `json:"tokenId"`
	To      string  `json:"to"`
	Amount  *uint64 `json:"amount"` // nil = 该 token 全额份额
}

// validateSftFlowRequest 校验请求体结构约束(纯函数,便于单测)。
// ownerAddress 必填:32 字节私钥按不同曲线解释派生不同地址(对齐 vc-flow 教训);
// 需要 create_sft 时 name/symbol 必填(链端强制,前置拦截避免白跑领水);
// slotId 与创建参数互斥;分发依赖 slot(显式复用或本次创建)。
func validateSftFlowRequest(req sftFlowRequest) error {
	if strings.TrimSpace(req.OwnerPrivateKey) == "" {
		return fmt.Errorf("ownerPrivateKey is required")
	}
	if strings.TrimSpace(req.OwnerAddress) == "" {
		return fmt.Errorf("ownerAddress is required(必填:32字节私钥在不同曲线下派生不同地址,请传账户生成时返回的地址,服务端按其自动匹配曲线)")
	}

	needCreate := req.Sft == nil || strings.TrimSpace(req.Sft.Address) == ""
	if needCreate {
		if req.SftMetadata == nil || strings.TrimSpace(req.SftMetadata.Name) == "" {
			return fmt.Errorf("sftMetadata.name is required(链端强制 1..=128 字符,创建新 SFT 时必填;复用已有 SFT 请传 sft.address)")
		}
		if strings.TrimSpace(req.SftMetadata.Symbol) == "" {
			return fmt.Errorf("sftMetadata.symbol is required(链端强制 1..=32 字符,创建新 SFT 时必填;复用已有 SFT 请传 sft.address)")
		}
	}

	if req.Slot != nil {
		if req.Slot.SlotId > 0 && (req.Slot.Metadata != nil || req.Slot.IsTransferable != nil) {
			return fmt.Errorf("slot.slotId 与 slot.metadata/isTransferable 互斥:传 slotId 表示复用已有 slot,传创建参数表示新建 slot,不可同时指定")
		}
	}

	if len(req.Distributions) > 0 && req.Slot == nil {
		return fmt.Errorf("distributions 需要 slot:分发目标 token 必须挂载在某个 slot 下,请传 slot.slotId(复用)或 slot 创建参数(新建)")
	}
	if len(req.Distributions) > sftFlowMaxDistributions {
		return fmt.Errorf("distributions 最多 %d 笔(每笔一笔链上交易),收到 %d 笔", sftFlowMaxDistributions, len(req.Distributions))
	}
	for i, d := range req.Distributions {
		if strings.TrimSpace(d.To) == "" {
			return fmt.Errorf("distributions[%d].to is required", i)
		}
		if _, err := types.ParseAddress(d.To); err != nil {
			return fmt.Errorf("distributions[%d].to %q: %w", i, d.To, err)
		}
		if d.Amount == 0 {
			return fmt.Errorf("distributions[%d].amount 必须为正数", i)
		}
	}

	if req.Merge != nil {
		if (req.Merge.FromTokenId == 0) != (req.Merge.ToTokenId == 0) {
			return fmt.Errorf("merge.fromTokenId and toTokenId 必须同时提供(或同时省略以跳过合并)")
		}
		if req.Merge.FromTokenId > 0 && req.Merge.FromTokenId == req.Merge.ToTokenId {
			return fmt.Errorf("merge.fromTokenId and toTokenId must differ(源与目标不能是同一个 token)")
		}
	}

	if req.Transfer != nil {
		if req.Transfer.TokenId == 0 {
			return fmt.Errorf("transfer.tokenId is required")
		}
		if strings.TrimSpace(req.Transfer.To) == "" {
			return fmt.Errorf("transfer.to is required")
		}
		if _, err := types.ParseAddress(req.Transfer.To); err != nil {
			return fmt.Errorf("transfer.to %q: %w", req.Transfer.To, err)
		}
		if req.Transfer.Amount != nil && *req.Transfer.Amount == 0 {
			return fmt.Errorf("transfer.amount 必须为正数;缺省(不传)表示转移该 token 全额份额")
		}
	}

	return nil
}

// normalizeSftFlowRequest 填充缺省值(isTransferable 缺省 true 等)。
func normalizeSftFlowRequest(req sftFlowRequest) sftFlowRequest {
	if req.Slot != nil && req.Slot.IsTransferable == nil {
		v := true
		req.Slot.IsTransferable = &v
	}
	if len(req.Distributions) == 0 {
		req.Distributions = nil
	}
	return req
}

// resolveSftFlowSlotId 解析 mint 使用的 slot_id(纯函数):
// 显式复用的 slotId 优先,其次本次创建的 slot_id,两者皆无则报错。
func resolveSftFlowSlotId(explicit, created uint64) (uint64, error) {
	if explicit > 0 {
		return explicit, nil
	}
	if created > 0 {
		return created, nil
	}
	return 0, fmt.Errorf("no slot available for mint:请传 slot.slotId(复用已有 slot)或 slot 创建参数(新建 slot)")
}

// ==================== 响应结构 ====================

// sftFlowSftResult SFT 创建/复用结果。
type sftFlowSftResult struct {
	Address    string `json:"address"`
	Owner      string `json:"owner"`
	PrivateKey string `json:"privateKey,omitempty"` // 仅服务端新生成资源账户时返回,便于复用
	Created    bool   `json:"created"`
	Skipped    bool   `json:"skipped"` // 链上已存在,跳过创建
	TxHash     string `json:"txHash,omitempty"`
	Detail     string `json:"detail,omitempty"`
}

// sftFlowSlotResult slot 创建/复用结果。
type sftFlowSlotResult struct {
	SlotId  uint64 `json:"slotId"`
	Created bool   `json:"created"`
	Reused  bool   `json:"reused"`
	TxHash  string `json:"txHash,omitempty"`
}

// sftFlowMintResult 一笔分发(= 一次 Mint)的结果。
type sftFlowMintResult struct {
	To      string `json:"to"`
	Amount  uint64 `json:"amount"`
	TokenId uint64 `json:"tokenId"`
	TxHash  string `json:"txHash,omitempty"`
}

// sftFlowMergeResult 合并结果。
type sftFlowMergeResult struct {
	FromTokenId  uint64 `json:"fromTokenId"`
	ToTokenId    uint64 `json:"toTokenId"`
	MergedAmount uint64 `json:"mergedAmount,omitempty"` // 从 TokenTransferredEvent 提取
	TxHash       string `json:"txHash,omitempty"`
}

// sftFlowTransferResult 转移结果。
type sftFlowTransferResult struct {
	TokenId uint64 `json:"tokenId"`
	To      string `json:"to"`
	Amount  uint64 `json:"amount"`
	TxHash  string `json:"txHash,omitempty"`
}

// sftFlowBalance 回读验证:某个 (token_id, holder) 的份额。
type sftFlowBalance struct {
	TokenId uint64 `json:"tokenId"`
	Holder  string `json:"holder"`
	Balance uint64 `json:"balance"`
}

// sftFlowVerification 回读验证结果。
type sftFlowVerification struct {
	SftMetadata map[string]any   `json:"sftMetadata,omitempty"`
	SlotInfo    map[string]any   `json:"slotInfo,omitempty"`
	Balances    []sftFlowBalance `json:"balances"`
}

// sftFlowResponse 全流程结果明细。
type sftFlowResponse struct {
	Owner         string                  `json:"owner"`
	Faucet        flowFaucetStep          `json:"faucet"`
	Sft           sftFlowSftResult        `json:"sft"`
	Slot          *sftFlowSlotResult      `json:"slot,omitempty"`
	Distributions []sftFlowMintResult     `json:"distributions"`
	Merge         *sftFlowMergeResult     `json:"merge,omitempty"`
	Transfer      *sftFlowTransferResult  `json:"transfer,omitempty"`
	Verification  *sftFlowVerification    `json:"verification,omitempty"`
}

// SftFlowHandler 提供 SFT 全流程工具。
type SftFlowHandler struct {
	nm *client.NetworkManager
}

// NewSftFlowHandler 创建绑定到 NetworkManager 的 SftFlowHandler。
func NewSftFlowHandler(nm *client.NetworkManager) *SftFlowHandler {
	return &SftFlowHandler{nm: nm}
}

// SftFlow 处理 POST /api/tool/sft-flow,同步执行全流程并返回每步明细。
func (h *SftFlowHandler) SftFlow(c *gin.Context) {
	var req sftFlowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logParamError(c, "SftFlow", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid request body: "+err.Error(), nil))
		return
	}
	if err := validateSftFlowRequest(req); err != nil {
		logParamError(c, "SftFlow", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, err.Error(), nil))
		return
	}
	req = normalizeSftFlowRequest(req)

	ownerSK, ownerPub, ownerAddr, err := resolveFlowParty("owner", req.OwnerPrivateKey, req.OwnerPublicKey, req.OwnerAddress)
	if err != nil {
		logParamError(c, "SftFlow", err)
		c.JSON(http.StatusBadRequest, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "invalid owner identity: "+err.Error(), nil))
		return
	}

	mc, _ := h.nm.GetCurrent()
	if mc == nil {
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, "no current network", nil))
		return
	}

	resp := sftFlowResponse{
		Owner:         ownerAddr.ToBase58(),
		Distributions: []sftFlowMintResult{},
	}
	fail := func(stage string, err error) {
		logSDKError(c, "SftFlow", fmt.Errorf("%s: %w", stage, err))
		c.JSON(http.StatusInternalServerError, types.ErrorResponse(types.ERR_SDK_ERROR, stage+": "+err.Error(), resp))
	}

	// [1] owner 领水(幂等:余额充足即跳过)
	if resp.Faucet, err = flowEnsureFaucet(mc, ownerAddr, ownerSK, ownerPub); err != nil {
		fail("owner faucet", err)
		return
	}

	// [2] 创建/复用 SFT
	sftAddr, sftStep, serr := sftFlowEnsureSft(mc, req, ownerAddr, ownerSK, ownerPub)
	resp.Sft = sftStep
	if serr != nil {
		fail("sft create", serr)
		return
	}

	// [3] 创建/复用 slot
	var createdSlotId uint64
	if req.Slot != nil {
		slotResult, serr := sftFlowEnsureSlot(mc, req.Slot, sftAddr, ownerAddr, ownerSK, ownerPub)
		resp.Slot = slotResult
		if serr != nil {
			fail("slot", serr)
			return
		}
		if slotResult.Created {
			createdSlotId = slotResult.SlotId
		}
	}
	// [4] 分发:逐笔 Mint 直发
	if len(req.Distributions) > 0 {
		slotId, serr := resolveSftFlowSlotId(req.Slot.SlotId, createdSlotId)
		if serr != nil {
			fail("resolve slot", serr)
			return
		}
		for i, d := range req.Distributions {
			mint, merr := sftFlowMintOne(mc, sftAddr, slotId, d, ownerAddr, ownerSK, ownerPub)
			resp.Distributions = append(resp.Distributions, mint)
			if merr != nil {
				fail(fmt.Sprintf("distributions[%d]", i), merr)
				return
			}
		}
	}

	// [5] 合并
	if req.Merge != nil && req.Merge.FromTokenId > 0 {
		merge, merr := sftFlowMerge(mc, sftAddr, req.Merge, ownerAddr, ownerSK, ownerPub)
		resp.Merge = merge
		if merr != nil {
			fail("merge", merr)
			return
		}
	}

	// [6] 转移
	if req.Transfer != nil {
		transfer, terr := sftFlowTransfer(mc, sftAddr, req.Transfer, ownerAddr, ownerSK, ownerPub)
		resp.Transfer = transfer
		if terr != nil {
			fail("transfer", terr)
			return
		}
	}

	// [7] 回读验证
	resp.Verification = sftFlowVerify(mc, sftAddr, resp.Slot, resp.Distributions)

	logBusinessInfo(c, "SftFlow",
		"owner", resp.Owner, "sft", resp.Sft.Address,
		"distributions", len(resp.Distributions))
	c.JSON(http.StatusOK, types.SuccessResponse(resp, "ok"))
}

// ==================== 链上步骤 ====================

// sftFlowEnsureSft 确保 SFT 存在:
//   - 未传 sft.address:服务端生成 Ed25519 资源账户(私钥随响应返回)并创建;
//   - 传 sft.address 且链上已存在(SftOwnerOf 可读):跳过;
//   - 传 sft.address 但不存在:须传 sft.privateKey 签名 create_sft(gas 由 owner 代付)。
func sftFlowEnsureSft(mc *milon.Client, req sftFlowRequest, ownerAddr crypto.Address, ownerSK crypto.SecretKeyer, ownerPub *crypto.PublicKey) (crypto.Address, sftFlowSftResult, error) {
	result := sftFlowSftResult{Owner: ownerAddr.ToBase58()}

	if req.Sft != nil && strings.TrimSpace(req.Sft.Address) != "" {
		addr, err := types.ParseAddress(req.Sft.Address)
		if err != nil {
			return crypto.Address{}, result, fmt.Errorf("invalid sft.address %q: %w", req.Sft.Address, err)
		}
		result.Address = addr.ToBase58()
		if value, verr := flowCallView(mc, sftFlowIdentityApp, "SftOwnerOf", provider.Args{"sft": addr.ToBase58()}); verr == nil && flowViewOK(value) {
			result.Skipped = true
			result.Detail = "sft already exists on chain"
			return addr, result, nil
		}
		// 链上不存在:需要资源账户私钥签名创建
		if strings.TrimSpace(req.Sft.PrivateKey) == "" {
			return crypto.Address{}, result, fmt.Errorf(
				"sft %s 不存在于链上,创建它需要 sft.privateKey(SFT 资源账户私钥,仅用于 create_sft 指令签名,gas 由 owner 代付);或省略 sft.address 由服务端生成新资源账户", addr.ToBase58())
		}
		sftSK, sftPub, sftAddr, err := resolveFlowParty("sft", req.Sft.PrivateKey, req.Sft.PublicKey, req.Sft.Address)
		if err != nil {
			return crypto.Address{}, result, err
		}
		txHash, err := sftFlowCreateSft(mc, sftAddr, sftSK, sftPub, req.SftMetadata, req.RoyaltyBps, ownerAddr, ownerSK, ownerPub)
		if err != nil {
			return crypto.Address{}, result, err
		}
		result.Created = true
		result.TxHash = txHash
		return sftAddr, result, nil
	}

	// 全新生成资源账户(Ed25519,与 SDK sft_demo 一致)
	sk := crypto.NewClassicalSecretKey()
	classicalSk := crypto.AsClassicalSecretKey(sk)
	pk := classicalSk.Ed25519Public()
	if pk == nil {
		return crypto.Address{}, result, fmt.Errorf("failed to derive ed25519 public key for sft account")
	}
	addr, err := crypto.NewAddressFromPublicKey(pk)
	if err != nil {
		return crypto.Address{}, result, fmt.Errorf("failed to derive sft address: %w", err)
	}
	txHash, err := sftFlowCreateSft(mc, *addr, classicalSk, pk, req.SftMetadata, req.RoyaltyBps, ownerAddr, ownerSK, ownerPub)
	if err != nil {
		return crypto.Address{}, result, err
	}
	result.Address = addr.ToBase58()
	result.PrivateKey = sk.ToHex()
	result.Created = true
	result.TxHash = txHash
	result.Detail = "generated new sft resource account (ed25519)"
	return *addr, result, nil
}

// sftFlowCreateSft 提交 create_sft:sft 资源账户签 ix0,owner 代付 gas。
func sftFlowCreateSft(mc *milon.Client, sftAddr crypto.Address, sftSK crypto.SecretKeyer, sftPub *crypto.PublicKey, meta *sftFlowMetadata, royaltyBps uint16, ownerAddr crypto.Address, ownerSK crypto.SecretKeyer, ownerPub *crypto.PublicKey) (string, error) {
	pd, ok := mc.GetAllPd()[sftFlowIdentityApp]
	if !ok {
		return "", fmt.Errorf("IDL app %q not found", sftFlowIdentityApp)
	}
	// IDL Metadata 的 name/symbol/cover_url/metadata 都是非 option 的 String,
	// 动态编码器要求全部显式出现(缺字段报 missing struct field),空串 = 无内容;
	// attribute 是 option<String>:nil = None,编码器同样要求键存在
	metadataArgs := map[string]any{
		"name":      meta.Name,
		"symbol":    meta.Symbol,
		"cover_url": meta.CoverUrl,
		"metadata":  meta.Metadata,
		"attribute": nil,
	}
	if meta.Attribute != nil {
		metadataArgs["attribute"] = *meta.Attribute
	}
	wire, err := encodeWithCoercion(pd, "CreateSft", provider.Args{
		"sft":         sftAddr.ToBase58(),
		"owner":       ownerAddr.ToBase58(),
		"metadata":    metadataArgs,
		"royalty_bps": royaltyBps,
	})
	if err != nil {
		return "", fmt.Errorf("failed to encode CreateSft: %w", err)
	}

	sftMode, err := normalizeSignatureModeForAccount(mc, sftAddr, lib.PubKeySignatureMode{PublicKey: *sftPub})
	if err != nil {
		return "", fmt.Errorf("failed to resolve sft signature mode: %w", err)
	}
	ownerMode, err := normalizeSignatureModeForAccount(mc, ownerAddr, lib.PubKeySignatureMode{PublicKey: *ownerPub})
	if err != nil {
		return "", fmt.Errorf("failed to resolve owner signature mode: %w", err)
	}

	tx, err := lib.NewTransactionBuilder([]api.PackedInstruction{wire}).
		WithPayer(&ownerAddr).
		AddIxesSig(sftAddr, sftSK, []uint8{0}, false, sftMode).
		AddPayerSig(ownerAddr, ownerSK, ownerMode).
		Build()
	if err != nil {
		return "", fmt.Errorf("failed to build CreateSft tx: %w", err)
	}
	_, txHash, err := sftFlowSendWait(mc, tx)
	return txHash, err
}

// sftFlowEnsureSlot 创建新 slot 或校验复用的 slot:
// 复用时先 view SlotInfo 确认存在,避免 mint 时才在链端报错。
func sftFlowEnsureSlot(mc *milon.Client, opts *sftFlowSlotOptions, sftAddr, ownerAddr crypto.Address, ownerSK crypto.SecretKeyer, ownerPub *crypto.PublicKey) (*sftFlowSlotResult, error) {
	if opts.SlotId > 0 {
		if value, err := flowCallView(mc, sftFlowIdentityApp, "SlotInfo", provider.Args{
			"sft":     sftAddr.ToBase58(),
			"slot_id": opts.SlotId,
		}); err != nil || !flowViewOK(value) {
			return nil, fmt.Errorf("slot %d 不存在或不可读(sft=%s),无法复用", opts.SlotId, sftAddr.ToBase58())
		}
		return &sftFlowSlotResult{SlotId: opts.SlotId, Reused: true}, nil
	}

	pd, ok := mc.GetAllPd()[sftFlowIdentityApp]
	if !ok {
		return nil, fmt.Errorf("IDL app %q not found", sftFlowIdentityApp)
	}
	// SlotData 仅一个字段 metadata(option<MetadataOverride>):编码器要求键存在,
	// nil = None(不覆盖,动态继承 SFT metadata)
	slotArgs := map[string]any{"metadata": nil}
	if opts.Metadata != nil {
		slotArgs["metadata"] = metadataOverrideArgs(opts.Metadata)
	}
	args := provider.Args{
		"creator":         ownerAddr.ToBase58(),
		"sft":             sftAddr.ToBase58(),
		"slot":            slotArgs,
		"is_transferable": *opts.IsTransferable,
	}
	tx, err := flowBuildTx(mc, sftFlowIdentityApp, "CreateSlot", args, ownerAddr, ownerSK, ownerPub)
	if err != nil {
		return nil, err
	}
	history, txHash, err := sftFlowSendWait(mc, tx)
	if err != nil {
		return nil, err
	}
	slotId, err := sftFlowEventU64(mc, pd, history, "SlotCreatedEvent", "slot_id")
	if err != nil {
		return nil, err
	}
	return &sftFlowSlotResult{SlotId: slotId, Created: true, TxHash: txHash}, nil
}

// sftFlowMintOne 执行一笔分发 Mint,token_id 从 TokenMintedEvent 提取。
func sftFlowMintOne(mc *milon.Client, sftAddr crypto.Address, slotId uint64, d sftFlowDistribution, ownerAddr crypto.Address, ownerSK crypto.SecretKeyer, ownerPub *crypto.PublicKey) (sftFlowMintResult, error) {
	result := sftFlowMintResult{To: d.To, Amount: d.Amount}
	pd, ok := mc.GetAllPd()[sftFlowIdentityApp]
	if !ok {
		return result, fmt.Errorf("IDL app %q not found", sftFlowIdentityApp)
	}
	args := provider.Args{
		"minter":  ownerAddr.ToBase58(),
		"sft":     sftAddr.ToBase58(),
		"slot_id": slotId,
		"to":      d.To,
		"amount":  d.Amount,
		// 顶层 option 参数同样要求键存在:nil = None(不覆盖)
		"metadata": nil,
	}
	if d.Metadata != nil {
		args["metadata"] = metadataOverrideArgs(d.Metadata)
	}
	tx, err := flowBuildTx(mc, sftFlowIdentityApp, "Mint", args, ownerAddr, ownerSK, ownerPub)
	if err != nil {
		return result, err
	}
	history, txHash, err := sftFlowSendWait(mc, tx)
	if err != nil {
		result.TxHash = txHash
		return result, err
	}
	result.TxHash = txHash
	tokenId, err := sftFlowEventU64(mc, pd, history, "TokenMintedEvent", "token_id")
	if err != nil {
		return result, err
	}
	result.TokenId = tokenId
	return result, nil
}

// sftFlowMerge 合并:把签名者在 from_token 的全部份额并入同 slot 的 to_token;
// 合并份额从 TokenTransferredEvent 提取。
func sftFlowMerge(mc *milon.Client, sftAddr crypto.Address, opts *sftFlowMergeOptions, ownerAddr crypto.Address, ownerSK crypto.SecretKeyer, ownerPub *crypto.PublicKey) (*sftFlowMergeResult, error) {
	result := &sftFlowMergeResult{FromTokenId: opts.FromTokenId, ToTokenId: opts.ToTokenId}
	pd, ok := mc.GetAllPd()[sftFlowIdentityApp]
	if !ok {
		return nil, fmt.Errorf("IDL app %q not found", sftFlowIdentityApp)
	}
	args := provider.Args{
		"owner":        ownerAddr.ToBase58(),
		"sft":          sftAddr.ToBase58(),
		"from_token_id": opts.FromTokenId,
		"to_token_id":  opts.ToTokenId,
	}
	tx, err := flowBuildTx(mc, sftFlowIdentityApp, "Merge", args, ownerAddr, ownerSK, ownerPub)
	if err != nil {
		return nil, err
	}
	history, txHash, err := sftFlowSendWait(mc, tx)
	if err != nil {
		result.TxHash = txHash
		return result, err
	}
	result.TxHash = txHash
	if amount, aerr := sftFlowEventU64(mc, pd, history, "TokenTransferredEvent", "amount"); aerr == nil {
		result.MergedAmount = amount
	}
	return result, nil
}

// sftFlowTransfer 转移:owner 的 token 份额转给 to;amount 缺省 = 全额
// (链上 BalanceOf 回读),余额为 0 时报错。
func sftFlowTransfer(mc *milon.Client, sftAddr crypto.Address, opts *sftFlowTransferOptions, ownerAddr crypto.Address, ownerSK crypto.SecretKeyer, ownerPub *crypto.PublicKey) (*sftFlowTransferResult, error) {
	result := &sftFlowTransferResult{TokenId: opts.TokenId, To: opts.To}
	amount := uint64(0)
	if opts.Amount != nil {
		amount = *opts.Amount
	} else {
		value, err := flowCallView(mc, sftFlowIdentityApp, "BalanceOf", provider.Args{
			"sft":      sftAddr.ToBase58(),
			"token_id": opts.TokenId,
			"owner":    ownerAddr.ToBase58(),
		})
		if err != nil {
			return nil, fmt.Errorf("failed to query balance of token %d: %w", opts.TokenId, err)
		}
		if !flowViewOK(value) {
			return nil, fmt.Errorf("token %d 不存在或不可读(sft=%s)", opts.TokenId, sftAddr.ToBase58())
		}
		balance, err := sftFlowToU64(value)
		if err != nil {
			return nil, fmt.Errorf("unexpected balance payload for token %d: %w", opts.TokenId, err)
		}
		if balance == 0 {
			return nil, fmt.Errorf("owner 在 token %d 上份额为 0,无份额可转移(sft=%s)", opts.TokenId, sftAddr.ToBase58())
		}
		amount = balance
	}
	result.Amount = amount

	args := provider.Args{
		"owner":    ownerAddr.ToBase58(),
		"sft":      sftAddr.ToBase58(),
		"token_id": opts.TokenId,
		"to":       opts.To,
		"amount":   amount,
	}
	tx, err := flowBuildTx(mc, sftFlowIdentityApp, "Transfer", args, ownerAddr, ownerSK, ownerPub)
	if err != nil {
		return nil, err
	}
	_, txHash, err := sftFlowSendWait(mc, tx)
	result.TxHash = txHash
	if err != nil {
		return result, err
	}
	return result, nil
}

// sftFlowVerify 回读验证:SftMetadata / SlotInfo / 分发产物余额。
// 验证失败不中断流程,仅缺省对应字段。
func sftFlowVerify(mc *milon.Client, sftAddr crypto.Address, slot *sftFlowSlotResult, mints []sftFlowMintResult) *sftFlowVerification {
	v := &sftFlowVerification{Balances: []sftFlowBalance{}}
	if value, err := flowCallView(mc, sftFlowIdentityApp, "SftMetadata", provider.Args{"sft": sftAddr.ToBase58()}); err == nil && flowViewOK(value) {
		if m, ok := value.(map[string]any); ok {
			v.SftMetadata = m
		}
	}
	if slot != nil {
		if value, err := flowCallView(mc, sftFlowIdentityApp, "SlotInfo", provider.Args{
			"sft":     sftAddr.ToBase58(),
			"slot_id": slot.SlotId,
		}); err == nil && flowViewOK(value) {
			if m, ok := value.(map[string]any); ok {
				v.SlotInfo = m
			}
		}
	}
	seen := map[uint64]bool{}
	for _, m := range mints {
		if m.TokenId == 0 || seen[m.TokenId] {
			continue
		}
		seen[m.TokenId] = true
		b := sftFlowBalance{TokenId: m.TokenId, Holder: m.To}
		if value, err := flowCallView(mc, sftFlowIdentityApp, "BalanceOf", provider.Args{
			"sft":      sftAddr.ToBase58(),
			"token_id": m.TokenId,
			"owner":    m.To,
		}); err == nil && flowViewOK(value) {
			if balance, berr := sftFlowToU64(value); berr == nil {
				b.Balance = balance
			}
		}
		v.Balances = append(v.Balances, b)
	}
	return v
}

// ==================== 交易收发与事件提取 ====================

// sftFlowSendWait 提交交易、等待确认并校验链上执行成功。
// 返回回执与 txHash(失败时也返回,便于追踪已提交的交易)。
func sftFlowSendWait(mc *milon.Client, tx *lib.Transaction) (*api.TxHistory, string, error) {
	if err := mc.SubmitTx(tx, milon.WithRequestID(lib.RequestID(time.Now().UnixMilli()))); err != nil {
		return nil, "", err
	}
	txHash := txHashHex(tx)
	result, err := mc.WaitForTransaction(txHash, milon.WithWaitRequestID(lib.RequestID(1)))
	if err != nil {
		return nil, txHash, err
	}
	history := result.BodyTxHistory
	if history == nil || history.Receipt.State != api.TxStateSuccess {
		code := uint16(0)
		if history != nil && history.Receipt.Error != nil {
			code = *history.Receipt.Error
		}
		return history, txHash, fmt.Errorf("transaction failed on chain: error code = %d", code)
	}
	return history, txHash, nil
}

// sftFlowEventU64 从回执事件中提取指定事件的 u64 字段(slot_id/token_id/amount)。
// 事件 typeTag 从已绑定的 sftoken IDL 查询,不硬编码。
func sftFlowEventU64(mc *milon.Client, pd *provider.Provider, history *api.TxHistory, eventName, fieldName string) (uint64, error) {
	var typeTag uint64
	found := false
	for tag, event := range pd.EventByTypeTag {
		if event.Name == eventName {
			typeTag = tag
			found = true
			break
		}
	}
	if !found {
		return 0, fmt.Errorf("event %s not found in %s IDL", eventName, sftFlowIdentityApp)
	}
	for _, event := range history.Receipt.Events {
		if event.TypeTag != typeTag {
			continue
		}
		decoded, err := mc.GetProviderManager().DecodeEventDataByTag(event.TypeTag, event.Value)
		if err != nil {
			return 0, fmt.Errorf("failed to decode event %s: %w", eventName, err)
		}
		data, ok := decoded["data"].(map[string]any)
		if !ok {
			return 0, fmt.Errorf("decoded event %s has no data payload", eventName)
		}
		return sftFlowToU64(data[fieldName])
	}
	return 0, fmt.Errorf("event %s not found in receipt", eventName)
}

// sftFlowToU64 宽容地把 view/事件解码值转成 u64(动态解码可能给出
// uint64/int/float64/json.Number 等载体类型)。
func sftFlowToU64(v any) (uint64, error) {
	switch x := v.(type) {
	case uint64:
		return x, nil
	case uint32:
		return uint64(x), nil
	case int:
		if x < 0 {
			return 0, fmt.Errorf("negative value %d", x)
		}
		return uint64(x), nil
	case int64:
		if x < 0 {
			return 0, fmt.Errorf("negative value %d", x)
		}
		return uint64(x), nil
	case float64:
		if x < 0 {
			return 0, fmt.Errorf("negative value %v", x)
		}
		return uint64(x), nil
	case json.Number:
		n, err := x.Int64()
		if err != nil {
			return 0, err
		}
		if n < 0 {
			return 0, fmt.Errorf("negative value %d", n)
		}
		return uint64(n), nil
	default:
		return 0, fmt.Errorf("not an unsigned integer: %T(%v)", v, v)
	}
}

// metadataOverrideArgs 把可选元数据覆盖转成 IDL 编码参数。
// 动态编码器要求 struct 全字段键显式出现,option 字段以 nil 表示 None(继承 SFT 元数据)。
func metadataOverrideArgs(o *sftFlowMetadataOverride) map[string]any {
	out := map[string]any{
		"name":      nil,
		"symbol":    nil,
		"cover_url": nil,
		"metadata":  nil,
		"attribute": nil,
	}
	if o == nil {
		return out
	}
	if o.Name != nil {
		out["name"] = *o.Name
	}
	if o.Symbol != nil {
		out["symbol"] = *o.Symbol
	}
	if o.CoverUrl != nil {
		out["cover_url"] = *o.CoverUrl
	}
	if o.Metadata != nil {
		out["metadata"] = *o.Metadata
	}
	if o.Attribute != nil {
		out["attribute"] = *o.Attribute
	}
	return out
}
