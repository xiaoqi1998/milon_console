package handler

import (
	"strings"
	"testing"
)

// testSftFlowAddr 从测试私钥派生一个合法地址,供请求体填充用。
func testSftFlowAddr(t *testing.T) string {
	t.Helper()
	_, _, addr, _, _, _ := vcFlowTestKeys(t)
	return addr.ToBase58()
}

// TestValidateSftFlowRequest 覆盖 /api/tool/sft-flow 请求体的纯结构校验。
func TestValidateSftFlowRequest(t *testing.T) {
	validFull := sftFlowRequest{
		OwnerPrivateKey: "000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f",
		OwnerAddress:    testSftFlowAddr(t),
		SftMetadata: &sftFlowMetadata{
			Name:   "Demo SFT",
			Symbol: "DSFT",
		},
		Slot: &sftFlowSlotOptions{},
		Distributions: []sftFlowDistribution{
			{To: testSftFlowAddr(t), Amount: 100},
		},
	}

	t.Run("合法全流程请求通过", func(t *testing.T) {
		if err := validateSftFlowRequest(validFull); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("ownerPrivateKey 必填", func(t *testing.T) {
		req := validFull
		req.OwnerPrivateKey = ""
		err := validateSftFlowRequest(req)
		if err == nil || !strings.Contains(err.Error(), "ownerPrivateKey is required") {
			t.Fatalf("expected ownerPrivateKey required error, got %v", err)
		}
	})

	t.Run("ownerAddress 必填", func(t *testing.T) {
		req := validFull
		req.OwnerAddress = ""
		err := validateSftFlowRequest(req)
		if err == nil || !strings.Contains(err.Error(), "ownerAddress is required") {
			t.Fatalf("expected ownerAddress required error, got %v", err)
		}
	})

	t.Run("未传已有 sft 地址时 name/symbol 必填", func(t *testing.T) {
		req := validFull
		req.SftMetadata = nil
		err := validateSftFlowRequest(req)
		if err == nil || !strings.Contains(err.Error(), "sftMetadata.name") {
			t.Fatalf("expected sftMetadata.name required error, got %v", err)
		}

		req.SftMetadata = &sftFlowMetadata{Name: "OnlyName"}
		err = validateSftFlowRequest(req)
		if err == nil || !strings.Contains(err.Error(), "sftMetadata.symbol") {
			t.Fatalf("expected sftMetadata.symbol required error, got %v", err)
		}
	})

	t.Run("传已有 sft 地址时无需 sftMetadata", func(t *testing.T) {
		req := validFull
		req.Sft = &sftFlowSftOptions{Address: testSftFlowAddr(t)}
		req.SftMetadata = nil
		if err := validateSftFlowRequest(req); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("复用已有 sft 地址时不传 sftPrivateKey 合法", func(t *testing.T) {
		req := validFull
		req.Sft = &sftFlowSftOptions{Address: testSftFlowAddr(t)}
		if err := validateSftFlowRequest(req); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("分发依赖 slot", func(t *testing.T) {
		req := validFull
		req.Slot = nil
		err := validateSftFlowRequest(req)
		if err == nil || !strings.Contains(err.Error(), "slot") {
			t.Fatalf("expected slot dependency error, got %v", err)
		}
	})

	t.Run("仅复用 slot 也可分发", func(t *testing.T) {
		req := validFull
		req.Slot = &sftFlowSlotOptions{SlotId: 3}
		if err := validateSftFlowRequest(req); err != nil {
			t.Fatalf("expected nil, got %v", err)
		}
	})

	t.Run("slotId 与创建参数互斥", func(t *testing.T) {
		req := validFull
		req.Slot = &sftFlowSlotOptions{SlotId: 3, Metadata: &sftFlowMetadataOverride{}}
		err := validateSftFlowRequest(req)
		if err == nil || !strings.Contains(err.Error(), "slotId") {
			t.Fatalf("expected slotId conflict error, got %v", err)
		}
	})

	t.Run("分发条目 amount 必须为正", func(t *testing.T) {
		req := validFull
		req.Distributions = []sftFlowDistribution{{To: testSftFlowAddr(t)}}
		err := validateSftFlowRequest(req)
		if err == nil || !strings.Contains(err.Error(), "amount") {
			t.Fatalf("expected amount error, got %v", err)
		}
	})

	t.Run("分发条目 to 必填", func(t *testing.T) {
		req := validFull
		req.Distributions = []sftFlowDistribution{{Amount: 10}}
		err := validateSftFlowRequest(req)
		if err == nil || !strings.Contains(err.Error(), "to") {
			t.Fatalf("expected to required error, got %v", err)
		}
	})

	t.Run("分发条目数上限", func(t *testing.T) {
		req := validFull
		for i := 0; i <= sftFlowMaxDistributions; i++ {
			req.Distributions = append(req.Distributions, sftFlowDistribution{
				To: testSftFlowAddr(t), Amount: 1,
			})
		}
		err := validateSftFlowRequest(req)
		if err == nil || !strings.Contains(err.Error(), "distributions") {
			t.Fatalf("expected distributions limit error, got %v", err)
		}
	})

	t.Run("merge 字段必须成对", func(t *testing.T) {
		req := validFull
		req.Merge = &sftFlowMergeOptions{FromTokenId: 2}
		err := validateSftFlowRequest(req)
		if err == nil || !strings.Contains(err.Error(), "fromTokenId and toTokenId") {
			t.Fatalf("expected merge pair error, got %v", err)
		}

		req.Merge = &sftFlowMergeOptions{ToTokenId: 1}
		err = validateSftFlowRequest(req)
		if err == nil || !strings.Contains(err.Error(), "fromTokenId and toTokenId") {
			t.Fatalf("expected merge pair error, got %v", err)
		}
	})

	t.Run("merge 源与目标不能相同", func(t *testing.T) {
		req := validFull
		req.Merge = &sftFlowMergeOptions{FromTokenId: 2, ToTokenId: 2}
		err := validateSftFlowRequest(req)
		if err == nil || !strings.Contains(err.Error(), "must differ") {
			t.Fatalf("expected merge differ error, got %v", err)
		}
	})

	t.Run("transfer 需要 tokenId 与 to", func(t *testing.T) {
		req := validFull
		req.Transfer = &sftFlowTransferOptions{To: testSftFlowAddr(t)}
		err := validateSftFlowRequest(req)
		if err == nil || !strings.Contains(err.Error(), "tokenId") {
			t.Fatalf("expected tokenId error, got %v", err)
		}

		req.Transfer = &sftFlowTransferOptions{TokenId: 1}
		err = validateSftFlowRequest(req)
		if err == nil || !strings.Contains(err.Error(), "to") {
			t.Fatalf("expected to error, got %v", err)
		}
	})

	t.Run("transfer 显式 amount 必须为正", func(t *testing.T) {
		req := validFull
		zero := uint64(0)
		req.Transfer = &sftFlowTransferOptions{
			TokenId: 1,
			To:      testSftFlowAddr(t),
			Amount:  &zero,
		}
		err := validateSftFlowRequest(req)
		if err == nil || !strings.Contains(err.Error(), "amount") {
			t.Fatalf("expected amount error, got %v", err)
		}
	})
}

// TestNormalizeSftFlowRequest 校验缺省值填充。
func TestNormalizeSftFlowRequest(t *testing.T) {
	t.Run("slot isTransferable 缺省 true", func(t *testing.T) {
		got := normalizeSftFlowRequest(sftFlowRequest{Slot: &sftFlowSlotOptions{}})
		if got.Slot.IsTransferable == nil || *got.Slot.IsTransferable != true {
			t.Fatalf("isTransferable default = %v, want true", got.Slot.IsTransferable)
		}
	})

	t.Run("显式 isTransferable=false 保留", func(t *testing.T) {
		f := false
		got := normalizeSftFlowRequest(sftFlowRequest{Slot: &sftFlowSlotOptions{IsTransferable: &f}})
		if got.Slot.IsTransferable == nil || *got.Slot.IsTransferable != false {
			t.Fatalf("isTransferable = %v, want false", got.Slot.IsTransferable)
		}
	})

	t.Run("distributions 空列表归一为 nil", func(t *testing.T) {
		got := normalizeSftFlowRequest(sftFlowRequest{Distributions: []sftFlowDistribution{}})
		if got.Distributions != nil {
			t.Fatalf("distributions = %v, want nil", got.Distributions)
		}
	})
}

// TestResolveSftFlowSlotId 校验 mint 使用的 slot_id 解析顺序:
// 显式复用 > 本次创建 > 报错。
func TestResolveSftFlowSlotId(t *testing.T) {
	t.Run("优先显式 slotId", func(t *testing.T) {
		got, err := resolveSftFlowSlotId(3, 9)
		if err != nil || got != 3 {
			t.Fatalf("got %d, %v; want 3, nil", got, err)
		}
	})

	t.Run("回退本次创建", func(t *testing.T) {
		got, err := resolveSftFlowSlotId(0, 9)
		if err != nil || got != 9 {
			t.Fatalf("got %d, %v; want 9, nil", got, err)
		}
	})

	t.Run("两者皆无报错", func(t *testing.T) {
		_, err := resolveSftFlowSlotId(0, 0)
		if err == nil || !strings.Contains(err.Error(), "slot") {
			t.Fatalf("expected slot error, got %v", err)
		}
	})
}
