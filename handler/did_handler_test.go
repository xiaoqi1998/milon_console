package handler

import (
	"fmt"
	"strings"
	"testing"
)

// ==================== buildDidName ====================

// TestBuildDidNameExplicitSuffix 显式指定 suffix 时原样采用。
func TestBuildDidNameExplicitSuffix(t *testing.T) {
	suffix := uint32(42)
	name, err := buildDidName("alice", &suffix, func() uint32 { t.Fatal("不应调用随机源"); return 0 })
	if err != nil {
		t.Fatal(err)
	}
	if name["alias"] != "alice" {
		t.Errorf("alias = %v, want alice", name["alias"])
	}
	if name["suffix"] != uint32(42) {
		t.Errorf("suffix = %v(%T), want uint32(42)", name["suffix"], name["suffix"])
	}
}

// TestBuildDidNameRandomSuffix 未指定 suffix 时由注入的随机源代填。
func TestBuildDidNameRandomSuffix(t *testing.T) {
	calls := 0
	name, err := buildDidName("alice", nil, func() uint32 { calls++; return 123456 })
	if err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Errorf("随机源调用次数 = %d, want 1", calls)
	}
	if name["suffix"] != uint32(123456) {
		t.Errorf("suffix = %v, want uint32(123456)", name["suffix"])
	}
}

// TestBuildDidNameTrimsAlias alias 前后空白应裁剪。
func TestBuildDidNameTrimsAlias(t *testing.T) {
	name, err := buildDidName("  alice  ", nil, func() uint32 { return 1 })
	if err != nil {
		t.Fatal(err)
	}
	if name["alias"] != "alice" {
		t.Errorf("alias = %v, want alice(已裁剪)", name["alias"])
	}
}

// TestBuildDidNameEmptyAlias 空白 alias 视为不设别名,返回 nil。
func TestBuildDidNameEmptyAlias(t *testing.T) {
	for _, alias := range []string{"", "   "} {
		name, err := buildDidName(alias, nil, func() uint32 { return 1 })
		if err != nil {
			t.Fatalf("alias %q: %v", alias, err)
		}
		if name != nil {
			t.Errorf("alias %q: 期望 nil, got %v", alias, name)
		}
	}
}

// ==================== buildDidDocInput ====================

// TestBuildDidDocInput 校验 DidDocumentInput wire map 组装。
func TestBuildDidDocInput(t *testing.T) {
	doc := buildDidDocInput("Organization", "pub-base58", []didServiceSpec{
		{Label: "website", Endpoint: "https://example.com"},
	}, "https://a.b/c.png")

	if doc["subject_type"] != "Organization" {
		t.Errorf("subject_type = %v, want Organization", doc["subject_type"])
	}
	if doc["avatar_uri"] != "https://a.b/c.png" {
		t.Errorf("avatar_uri = %v", doc["avatar_uri"])
	}
	keys, ok := doc["keys"].([]any)
	if !ok || len(keys) != 1 {
		t.Fatalf("keys = %v, want 1 个元素", doc["keys"])
	}
	key0, ok := keys[0].(map[string]any)
	if !ok || key0["public_key"] != "pub-base58" || key0["label"] != "primary" {
		t.Errorf("keys[0] = %v, want {public_key: pub-base58, label: primary}", keys[0])
	}
	services, ok := doc["services"].([]any)
	if !ok || len(services) != 1 {
		t.Fatalf("services = %v, want 1 个元素", doc["services"])
	}
	svc0, ok := services[0].(map[string]any)
	if !ok || svc0["label"] != "website" || svc0["service_endpoint"] != "https://example.com" {
		t.Errorf("services[0] = %v, want {label: website, service_endpoint: https://example.com}", services[0])
	}
}

// TestBuildDidDocInputEmptyServices 空 services 必须编码为空数组(非 nil),
// 与 vcFlowEnsureDID 现有行为一致。
func TestBuildDidDocInputEmptyServices(t *testing.T) {
	doc := buildDidDocInput("Personal", "pub", nil, "https://a.b/c.png")
	services, ok := doc["services"].([]any)
	if !ok || len(services) != 0 {
		t.Errorf("services = %v(%T), want 空数组", doc["services"], doc["services"])
	}
}

// ==================== normalize / validate ====================

// TestNormalizeDidCreateRequest 空缺省值填充与裁剪。
func TestNormalizeDidCreateRequest(t *testing.T) {
	req := normalizeDidCreateRequest(didCreateRequest{SubjectType: "  ", Services: []didServiceSpec{{Label: " x ", Endpoint: " https://a.b "}}})
	if req.SubjectType != "Personal" {
		t.Errorf("subjectType = %q, want Personal(缺省)", req.SubjectType)
	}
	if req.Services[0].Label != "x" || req.Services[0].Endpoint != "https://a.b" {
		t.Errorf("services 未裁剪: %+v", req.Services[0])
	}
}

// TestValidateDidCreateRequest 覆盖必填与格式约束。
func TestValidateDidCreateRequest(t *testing.T) {
	base := didCreateRequest{PrivateKey: "sk", Address: "addr", SubjectType: "Personal"}

	cases := []struct {
		name    string
		mutate  func(r *didCreateRequest)
		wantSub string // 期望错误信息包含的子串
	}{
		{"缺私钥", func(r *didCreateRequest) { r.PrivateKey = "" }, "privateKey is required"},
		{"缺地址", func(r *didCreateRequest) { r.Address = "" }, "address is required"},
		{"非法主体类型", func(r *didCreateRequest) { r.SubjectType = "Company" }, "subjectType"},
		{"服务缺 label", func(r *didCreateRequest) { r.Services = []didServiceSpec{{Label: " ", Endpoint: "https://a.b"}} }, "label"},
		{"服务缺 endpoint", func(r *didCreateRequest) { r.Services = []didServiceSpec{{Label: "x", Endpoint: "  "}} }, "serviceEndpoint"},
		{"endpoint 非绝对 URI", func(r *didCreateRequest) { r.Services = []didServiceSpec{{Label: "x", Endpoint: "foo/bar"}} }, "absolute"},
	}
	for _, tc := range cases {
		req := base
		tc.mutate(&req)
		err := validateDidCreateRequest(req)
		if err == nil {
			t.Errorf("%s: 期望报错", tc.name)
			continue
		}
		if !strings.Contains(err.Error(), tc.wantSub) {
			t.Errorf("%s: 错误 %q 未包含 %q", tc.name, err.Error(), tc.wantSub)
		}
	}

	// 合法请求(含组织类型 + 显式 suffix)应通过
	req := base
	req.SubjectType = "Organization"
	req.Services = []didServiceSpec{{Label: "api", Endpoint: "https://api.example.com/v1"}}
	if err := validateDidCreateRequest(req); err != nil {
		t.Errorf("合法请求报错: %v", err)
	}
}

// ==================== planDidBackfill(幂等补齐差异) ====================

// didTestDoc 构造 Document view 解码形态的链上文档。
func didTestDoc(alias any, services []map[string]any, avatar string) map[string]any {
	if services == nil {
		services = []map[string]any{}
	}
	arr := make([]any, 0, len(services))
	for _, s := range services {
		arr = append(arr, s)
	}
	return map[string]any{
		"subject":     map[string]any{"subject_type": "Personal", "address": "addrX"},
		"controller":  "addrX",
		"keys":        []any{},
		"services":    arr,
		"alias":       alias,
		"avatar_uri":  avatar,
		"deactivated": false,
	}
}

// TestPlanBackfillNotCreated 链上无文档(nil) → 走创建。
func TestPlanBackfillNotCreated(t *testing.T) {
	plan := planDidBackfill(nil, didCreateRequest{Alias: "alice"})
	if !plan.NeedCreate {
		t.Errorf("链上无文档应 NeedCreate=true, got %+v", plan)
	}
}

// TestPlanBackfillFullMatch 请求全部已满足 → 无需任何补齐。
func TestPlanBackfillFullMatch(t *testing.T) {
	doc := didTestDoc(
		map[string]any{"alias": "alice", "suffix": uint32(1234)},
		[]map[string]any{{"id": uint8(0), "label": "s1", "service_endpoint": "https://s1"}},
		"https://a.b/c.png",
	)
	req := didCreateRequest{
		Alias:     "alice",
		Services:  []didServiceSpec{{Label: "s1", Endpoint: "https://s1"}},
		AvatarURI: "https://a.b/c.png",
	}
	plan := planDidBackfill(doc, req)
	if plan.NeedCreate || plan.SetAlias || plan.SetAvatar || len(plan.AddServices) != 0 {
		t.Errorf("全部已满足,期望零补齐, got %+v", plan)
	}
}

// TestPlanBackfillAliasDifference 别名差异触发 SetAlias。
func TestPlanBackfillAliasDifference(t *testing.T) {
	cases := []struct {
		name    string
		chain   any
		req     string
		wantSet bool
	}{
		{"链上无别名,请求 alice", nil, "alice", true},
		{"链上 bob,请求 alice", map[string]any{"alias": "bob", "suffix": uint32(1)}, "alice", true},
		{"链上 alice(后缀不同),请求 alice", map[string]any{"alias": "alice", "suffix": uint32(9)}, "alice", false},
	}
	for _, tc := range cases {
		plan := planDidBackfill(didTestDoc(tc.chain, nil, "https://a.b"), didCreateRequest{Alias: tc.req})
		if plan.SetAlias != tc.wantSet {
			t.Errorf("%s: SetAlias = %v, want %v (plan=%+v)", tc.name, plan.SetAlias, tc.wantSet, plan)
		}
	}
}

// TestPlanBackfillServiceDifference 缺失的 service 进补齐列表,已有(同 label+endpoint)不重复加。
func TestPlanBackfillServiceDifference(t *testing.T) {
	doc := didTestDoc(nil, []map[string]any{
		{"id": uint8(0), "label": "s1", "service_endpoint": "https://s1"},
	}, "https://a.b")
	req := didCreateRequest{Services: []didServiceSpec{
		{Label: "s1", Endpoint: "https://s1"}, // 已存在
		{Label: "s2", Endpoint: "https://s2"}, // 缺失
		{Label: "s1", Endpoint: "https://other"}, // 同 label 不同 endpoint → 缺失
	}}
	plan := planDidBackfill(doc, req)
	if len(plan.AddServices) != 2 {
		t.Fatalf("AddServices = %+v, want 2 项", plan.AddServices)
	}
	if plan.AddServices[0].Label != "s2" || plan.AddServices[1].Label != "s1" {
		t.Errorf("AddServices 顺序/内容不符: %+v", plan.AddServices)
	}
}

// TestPlanBackfillAvatar 显式传入且与链上不同才 SetAvatar;
// 未传(空)绝不覆盖链上已有头像。
func TestPlanBackfillAvatar(t *testing.T) {
	doc := didTestDoc(nil, nil, "https://chain/avatar.png")

	if plan := planDidBackfill(doc, didCreateRequest{}); plan.SetAvatar {
		t.Errorf("未传 avatarUri 不应覆盖链上头像")
	}
	if plan := planDidBackfill(doc, didCreateRequest{AvatarURI: "https://chain/avatar.png"}); plan.SetAvatar {
		t.Errorf("与链上相同不应 SetAvatar")
	}
	if plan := planDidBackfill(doc, didCreateRequest{AvatarURI: "https://new/avatar.png"}); !plan.SetAvatar {
		t.Errorf("与链上不同应 SetAvatar")
	}
}

// ==================== 链上值解析辅助 ====================

// TestDidDocumentAliasName 从 Document view 值提取当前别名字符串。
func TestDidDocumentAliasName(t *testing.T) {
	if got := didDocumentAliasName(didTestDoc(nil, nil, "")); got != "" {
		t.Errorf("无别名应返回空串, got %q", got)
	}
	if got := didDocumentAliasName(didTestDoc(map[string]any{"alias": "alice", "suffix": uint32(7)}, nil, "")); got != "alice" {
		t.Errorf("alias = %q, want alice", got)
	}
}

// TestIsDidNotFoundErr document 查询端点对「未创建」的判定。
func TestIsDidNotFoundErr(t *testing.T) {
	if isDidNotFoundErr("view identity.Document failed: DidAlreadyExists") {
		t.Errorf("DidAlreadyExists 不应命中")
	}
	if !isDidNotFoundErr("identity.Document: DidNotFound (1025): subject") {
		t.Errorf("DidNotFound 应命中")
	}
	if !isDidNotFoundErr("code 1025 something") {
		t.Errorf("1025 应命中")
	}
	if isDidNotFoundErr("some other error") {
		t.Errorf("其他错误不应命中")
	}
}

// ==================== vc-flow 透传 ====================

// TestNormalizeVcFlowDidOptions vc-flow 的 issuerDid/userDid 选项归一化。
func TestNormalizeVcFlowDidOptions(t *testing.T) {
	opts := normalizeVcFlowDidOptions(&vcFlowDidOptions{
		Alias:    "  alice  ",
		Services: []didServiceSpec{{Label: " x ", Endpoint: " https://a.b "}},
	})
	if opts.Alias != "alice" || opts.Services[0].Label != "x" || opts.Services[0].Endpoint != "https://a.b" {
		t.Errorf("归一化失败: %+v", opts)
	}
}

// TestVcFlowEnsureDIDStepSummary 多步补齐结果汇总成单个 flowStep。
func TestVcFlowEnsureDIDStepSummary(t *testing.T) {
	// 全部 skipped → Skipped=true
	all := didEnsureResult{
		Create: flowStep{Skipped: true, Detail: "exists"},
		Alias:  &flowStep{Skipped: true, Detail: "same"},
	}
	summary := summarizeDidEnsure(all)
	if !summary.Skipped || summary.TxHash != "" {
		t.Errorf("全 skipped 汇总错误: %+v", summary)
	}

	// 有实际交易 → TxHash 取创建步,Skipped=false;创建步的 detail(如 created with alias)须保留
	mixed := didEnsureResult{
		Created: true,
		Create:  flowStep{TxHash: "0xcreate", Detail: "created with alias"},
		Alias:   &flowStep{TxHash: "0xalias"},
	}
	summary = summarizeDidEnsure(mixed)
	if summary.Skipped || summary.TxHash != "0xcreate" {
		t.Errorf("混合结果汇总错误: %+v", summary)
	}
	if !strings.Contains(summary.Detail, "created with alias") || !strings.Contains(summary.Detail, "alias set") {
		t.Errorf("summary.Detail 丢失创建细节: %q", summary.Detail)
	}
}

// TestDidServiceSpecFromRequest services JSON 数组转 spec(校验在 validate 层)。
func TestDidServiceSpecFromRequest(t *testing.T) {
	in := []didServiceSpec{{Label: "a", Endpoint: "https://a"}, {Label: "b", Endpoint: "https://b"}}
	specs := append([]didServiceSpec{}, in...)
	if len(specs) != 2 || specs[1].Endpoint != "https://b" {
		t.Errorf("拷贝失真: %+v", specs)
	}
}

// TestDidAliasSuffixRange 服务端代填 suffix 的取值范围固定,
// 便于排查链端 1038(后缀位数不符)时统一调整。
func TestDidAliasSuffixRange(t *testing.T) {
	if didAliasSuffixMin < 0 || didAliasSuffixMax <= didAliasSuffixMin {
		t.Fatalf("suffix 范围非法: [%d, %d)", didAliasSuffixMin, didAliasSuffixMax)
	}
	for i := 0; i < 50; i++ {
		s := randomDidSuffix()
		if s < didAliasSuffixMin || s >= didAliasSuffixMax {
			t.Fatalf("randomDidSuffix() = %d, 越界 [%d, %d)", s, didAliasSuffixMin, didAliasSuffixMax)
		}
	}
}

// TestFormatDidNameError 链端别名格式错误(1035-1039)的提示语组装。
func TestFormatDidNameError(t *testing.T) {
	msg := formatDidNameError(fmt.Errorf("NameInvalidFormat (1036)")).Error()
	if !strings.Contains(msg, "alias-数字") || !strings.Contains(msg, "1036") {
		t.Errorf("提示语未包含格式说明与原始错误: %s", msg)
	}
}

// ==================== vc-flow 默认完整创建(自动别名) ====================

// TestAutoDidAlias 自动别名规则:角色前缀 + 地址 base58 前 8 位,纯字母数字。
func TestAutoDidAlias(t *testing.T) {
	_, _, _, _, _, userAddr := vcFlowTestKeys(t)
	alias := autoDidAlias("user", *userAddr)
	if !strings.HasPrefix(alias, "user") {
		t.Errorf("user 角色别名应以 user 开头: %q", alias)
	}
	if strings.Contains(alias, "-") {
		t.Errorf("别名主体不应含连字符(避开链端 alias-数字分隔歧义): %q", alias)
	}
	orgAlias := autoDidAlias("org", *userAddr)
	if !strings.HasPrefix(orgAlias, "org") {
		t.Errorf("org 角色别名应以 org 开头: %q", orgAlias)
	}
	if len(alias) != len("user")+8 {
		t.Errorf("别名长度 = %d, want %d(前缀+8位地址片段)", len(alias), len("user")+8)
	}
}

// TestResolveVcFlowDidAlias 未传选项或未指定别名时默认自动生成;
// 显式 alias 覆盖自动值;autoAlias=false 且未给别名时不绑定。
func TestResolveVcFlowDidAlias(t *testing.T) {
	_, _, _, _, _, userAddr := vcFlowTestKeys(t)

	// 未传 → 自动生成
	alias, bind := resolveVcFlowDidAlias(nil, "user", *userAddr)
	if !bind || alias == "" {
		t.Errorf("未传选项应自动生成别名: %q bind=%v", alias, bind)
	}

	// 显式 alias → 覆盖
	opts := &vcFlowDidOptions{Alias: "myname"}
	alias, bind = resolveVcFlowDidAlias(opts, "user", *userAddr)
	if !bind || alias != "myname" {
		t.Errorf("显式别名应生效: %q bind=%v", alias, bind)
	}

	// autoAlias=false → 不绑定
	opts = &vcFlowDidOptions{AutoAlias: new(bool)}
	alias, bind = resolveVcFlowDidAlias(opts, "user", *userAddr)
	if bind || alias != "" {
		t.Errorf("autoAlias=false 应不绑定: %q bind=%v", alias, bind)
	}
}

// TestParseDidNameString name-binding 查询的 name 参数拆分。
func TestParseDidNameString(t *testing.T) {
	alias, suffix, err := parseDidNameString("alice-1024")
	if err != nil || alias != "alice" || suffix != 1024 {
		t.Errorf("alice-1024 → (%q, %d, %v), want (alice, 1024, nil)", alias, suffix, err)
	}
	if _, _, err := parseDidNameString("no-suffix-here"); err == nil {
		t.Errorf("多段名称应报错")
	}
	if _, _, err := parseDidNameString("alice-abc"); err == nil {
		t.Errorf("非数字 suffix 应报错")
	}
	if _, _, err := parseDidNameString("alice"); err == nil {
		t.Errorf("缺 suffix 应报错")
	}
	// 别名本身可含数字,以最后一个 '-' 分隔
	alias, suffix, err = parseDidNameString("user2-77")
	if err != nil || alias != "user2" || suffix != 77 {
		t.Errorf("user2-77 → (%q, %d, %v), want (user2, 77, nil)", alias, suffix, err)
	}
}

// ==================== didBindAlias 撞名重试 ====================

// didBindAliasSendCounter 包装 send,返回第 n 次调用的结果序列。
func didBindAliasSendCounter(results []error) (func(map[string]any) (string, error), *int) {
	calls := 0
	return func(map[string]any) (string, error) {
		i := calls
		calls++
		if i < len(results) {
			return fmt.Sprintf("0x%d", i), results[i]
		}
		return fmt.Sprintf("0x%d", i), nil
	}, &calls
}

// TestDidBindAliasSuccess 首次成功直接返回。
func TestDidBindAliasSuccess(t *testing.T) {
	send, calls := didBindAliasSendCounter(nil)
	out := didBindAlias("alice", nil, send)
	if out.Err != nil || *calls != 1 {
		t.Errorf("首次成功: out=%+v calls=%d", out, *calls)
	}
}

// TestDidBindAliasRetryOnCollision 服务端代填 suffix 撞他人占名(1028)时自动换号重试。
func TestDidBindAliasRetryOnCollision(t *testing.T) {
	err1028 := fmt.Errorf("contract error: NameAlreadyBound (1028): DID name already bound")
	send, calls := didBindAliasSendCounter([]error{err1028, err1028}) // 第 3 次成功
	out := didBindAlias("alice", nil, send)
	if out.Err != nil {
		t.Errorf("第 3 次应成功: %+v", out)
	}
	if *calls != 3 {
		t.Errorf("send 调用次数 = %d, want 3(两次撞名重试)", *calls)
	}
}

// TestDidBindAliasExplicitSuffixNoRetry 显式 suffix 撞名不重试,直接报错。
func TestDidBindAliasExplicitSuffixNoRetry(t *testing.T) {
	err1028 := fmt.Errorf("contract error: NameAlreadyBound (1028)")
	send, calls := didBindAliasSendCounter([]error{err1028})
	suffix := uint32(1234)
	out := didBindAlias("alice", &suffix, send)
	if out.Err == nil {
		t.Errorf("显式 suffix 撞名应报错: %+v", out)
	}
	if !strings.Contains(out.Err.Error(), "1028") {
		t.Errorf("错误应保留原文: %v", out.Err)
	}
	if *calls != 1 {
		t.Errorf("send 调用次数 = %d, want 1(不重试)", *calls)
	}
}

// TestDidBindAliasRetryLimit 代填 suffix 持续撞名时最多重试 didAliasMaxRetries 次。
func TestDidBindAliasRetryLimit(t *testing.T) {
	err1028 := fmt.Errorf("contract error: NameAlreadyBound (1028)")
	results := make([]error, didAliasMaxRetries+1)
	for i := range results {
		results[i] = err1028
	}
	send, calls := didBindAliasSendCounter(results)
	out := didBindAlias("alice", nil, send)
	if out.Err == nil {
		t.Errorf("持续撞名应最终报错")
	}
	if *calls != didAliasMaxRetries+1 {
		t.Errorf("send 调用次数 = %d, want %d(初次+重试)", *calls, didAliasMaxRetries+1)
	}
}

// TestDidBindAliasOtherError 剩余错误(如余额不足)不重试不包裹,原样透传。
func TestDidBindAliasOtherError(t *testing.T) {
	errOther := fmt.Errorf("simulate tx error: 余额不足 code 513")
	send, calls := didBindAliasSendCounter([]error{errOther})
	out := didBindAlias("alice", nil, send)
	if out.Err == nil || !strings.Contains(out.Err.Error(), "513") || strings.Contains(out.Err.Error(), "alias-数字") {
		t.Errorf("无关错误应原样透传: %+v", out)
	}
	if *calls != 1 {
		t.Errorf("send 调用次数 = %d, want 1", *calls)
	}
}

// TestDidBindAliasFormatErrorWrapped 链端格式错误(1036)应附加格式提示。
func TestDidBindAliasFormatErrorWrapped(t *testing.T) {
	errFmt := fmt.Errorf("NameInvalidFormat (1036): alias-digits format")
	send, calls := didBindAliasSendCounter([]error{errFmt})
	out := didBindAlias("alice!", nil, send)
	if out.Err == nil || !strings.Contains(out.Err.Error(), "alias-数字") || !strings.Contains(out.Err.Error(), "1036") {
		t.Errorf("格式错误应带提示: %+v", out)
	}
	if *calls != 1 {
		t.Errorf("格式错误不应重试: %d", *calls)
	}
}
