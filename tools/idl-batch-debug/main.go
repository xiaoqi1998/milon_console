// idl-batch-debug 遍历所有 IDL entry 方法，自动生成参数并逐个 simulate，
// 输出每个方法的通过/失败报告，用于批量定位 IDL 调用问题（如 requires signer、缺参数等）。
//
// 用法：
//
//	go run ./tools/idl-batch-debug -network devNet
//
// 可选参数：
//
//	-network  网络名 (localNet | devNet)，默认 devNet
//	-app      只跑指定 app（如 token），默认全部
//	-method   只跑指定方法（配合 -app），默认全部
//	-json     以 JSON 输出报告
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	milon "github.com/milon-labs/milon-go-sdk"
	"github.com/milon-labs/milon-go-sdk/api"
	"github.com/milon-labs/milon-go-sdk/crypto"
	"github.com/milon-labs/milon-go-sdk/lib"
	"github.com/milon-labs/milon-go-sdk/provider"
)

// 为模拟生成一个测试账户（随机密钥对 -> 地址 + 公钥）。
// simulate 不需要真实账户/私钥，只要求 公钥派生地址 == 签名地址。
type testAccount struct {
	addr *crypto.Address
	pk   *crypto.PublicKey
	mode lib.PubKeySignatureMode
}

func newTestAccount() (*testAccount, error) {
	sk := crypto.AsClassicalSecretKey(crypto.NewPureClassicalSecretKey())
	pk := sk.Ed25519Public()
	addr, err := crypto.NewAddressFromPublicKey(pk)
	if err != nil {
		return nil, err
	}
	return &testAccount{
		addr: addr,
		pk:   pk,
		mode: lib.PubKeySignatureMode{PublicKey: *pk},
	}, nil
}

type result struct {
	App      string `json:"app"`
	Method   string `json:"method"`
	Handler  string `json:"handler"`
	OK       bool   `json:"ok"`
	Gas      uint64 `json:"gas,omitempty"`
	Err      string `json:"err,omitempty"`
	ErrCode  uint16 `json:"errCode,omitempty"`
	Elapsed  string `json:"elapsed"`
	Simulate bool   `json:"simulate"`
}

func main() {
	networkFlag := flag.String("network", "devNet", "network: localNet | devNet")
	appFlag := flag.String("app", "", "only run this app (e.g. token)")
	methodFlag := flag.String("method", "", "only run this method (with -app)")
	jsonFlag := flag.Bool("json", false, "output JSON")
	flag.Parse()

	var network milon.Network
	switch *networkFlag {
	case "localNet", "LocalNet":
		network = milon.LocalNet
	case "devNet", "DevNet":
		network = milon.DevNet
	default:
		fmt.Printf("unknown network %q (use localNet|devNet)\n", *networkFlag)
		os.Exit(1)
	}

	client := milon.NewClient(network)
	allPd := client.GetAllPd()

	// 按 appId 排序，保证输出稳定
	appNames := make([]string, 0, len(allPd))
	for name := range allPd {
		appNames = append(appNames, name)
	}
	sort.Strings(appNames)

	results := []result{}
	total := 0
	passed := 0
	failed := 0
	skipped := 0

	for _, appName := range appNames {
		if *appFlag != "" && appName != *appFlag {
			continue
		}
		pd := allPd[appName]
		for _, ix := range pd.IDL.Instructions {
			if ix.Kind != "entry" {
				continue
			}
			if *methodFlag != "" && ix.Name != *methodFlag {
				continue
			}
			total++
			r := runOne(client, pd, appName, ix)
			results = append(results, r)
			if r.OK {
				passed++
			} else if r.Err == "SKIPPED" {
				skipped++
			} else {
				failed++
			}
		}
	}

	if *jsonFlag {
		out, _ := json.MarshalIndent(map[string]any{
			"network": *networkFlag,
			"total":   total,
			"passed":  passed,
			"failed":  failed,
			"skipped": skipped,
			"results": results,
		}, "", "  ")
		fmt.Println(string(out))
		return
	}

	// 文本报告
	fmt.Printf("\n======== IDL Batch Debug Report ========\n")
	fmt.Printf("network: %s\n", *networkFlag)
	fmt.Printf("total: %d, passed: %d, failed: %d, skipped: %d\n\n", total, passed, failed, skipped)

	if passed > 0 {
		fmt.Println("--- PASSED ---")
		for _, r := range results {
			if r.OK {
				fmt.Printf("  ✓ %s.%s (gas=%d, %s)\n", r.App, r.Method, r.Gas, r.Elapsed)
			}
		}
		fmt.Println()
	}
	if failed > 0 {
		fmt.Println("--- FAILED ---")
		for _, r := range results {
			if !r.OK && r.Err != "SKIPPED" {
				fmt.Printf("  ✗ %s.%s (code=%d, %s)\n      %s\n", r.App, r.Method, r.ErrCode, r.Elapsed, r.Err)
			}
		}
		fmt.Println()
	}
	if skipped > 0 {
		fmt.Println("--- SKIPPED (无法编码，参数生成失败) ---")
		for _, r := range results {
			if r.Err == "SKIPPED" {
				fmt.Printf("  - %s.%s : %s\n", r.App, r.Method, r.Err)
			}
		}
		fmt.Println()
	}
}

// runOne 对单个 entry 方法构建 simulate 交易并跑链上模拟。
func runOne(client *milon.Client, pd *provider.Provider, appName string, ix provider.Instruction) result {
	start := time.Now()
	r := result{App: appName, Method: ix.Name, Handler: ix.Handler, Simulate: true}

	// 生成 args + 签名者集合
	gen := &argGen{pd: pd, accs: map[string]*testAccount{}}
	args, signers, err := gen.build(ix)
	if err != nil {
		r.Err = "SKIPPED"
		r.Elapsed = time.Since(start).Round(time.Millisecond).String()
		return r
	}

	// 编码指令
	wireBytes, err := pd.Encode(ix.Name, args)
	if err != nil {
		r.Err = "encode failed: " + err.Error()
		r.Elapsed = time.Since(start).Round(time.Millisecond).String()
		return r
	}
	instructions := []api.PackedInstruction{api.PackedInstruction(wireBytes)}

	// 构建 simulate 交易：
	//  - 第一个签名者作为 payer（sign bit63 + bit0，如果它也是 signer）
	//  - 其余签名者签 bit0
	// 如果没有显式 signer，用主测试账户做 payer + signer
	builder := lib.NewTransactionBuilder(instructions)

	if len(signers) == 0 {
		// 无 signer 参数（罕见）：用主测试账户，payer 签 bit0+gas
		mainAcc, err := newTestAccount()
		if err != nil {
			r.Err = "gen account failed: " + err.Error()
			r.Elapsed = time.Since(start).Round(time.Millisecond).String()
			return r
		}
		builder.WithPayer(mainAcc.addr).
			AddSimulateIxAndPayerSig(*mainAcc.addr, 0, mainAcc.mode)
	} else {
		builder.WithPayer(signers[0].addr)
		builder.AddSimulateIxAndPayerSig(*signers[0].addr, 0, signers[0].mode)
		for i := 1; i < len(signers); i++ {
			builder.AddSimulateIxesSig(*signers[i].addr, []uint8{0}, false, signers[i].mode)
		}
	}

	tx, err := builder.Build()
	if err != nil {
		r.Err = "build failed: " + err.Error()
		r.Elapsed = time.Since(start).Round(time.Millisecond).String()
		return r
	}
	if err := tx.ValidateWire(); err != nil {
		r.Err = "validate failed: " + err.Error()
		r.Elapsed = time.Since(start).Round(time.Millisecond).String()
		return r
	}

	res, err := client.SimulateTx(tx)
	if err != nil {
		r.Err = "simulate failed: " + err.Error()
		r.Elapsed = time.Since(start).Round(time.Millisecond).String()
		return r
	}
	r.Elapsed = time.Since(start).Round(time.Millisecond).String()
	if res == nil || res.BodySimulateReceipt == nil {
		r.Err = "empty simulate receipt"
		return r
	}
	if res.BodySimulateReceipt.State == api.TxStateSuccess {
		r.OK = true
		r.Gas = res.BodySimulateReceipt.GasCharged
	} else if res.BodySimulateReceipt.Error != nil {
		r.Err = res.BodySimulateReceipt.Error.Message
		r.ErrCode = res.BodySimulateReceipt.Error.Code
	} else {
		r.Err = fmt.Sprintf("state=%d (failed, no error payload)", res.BodySimulateReceipt.State)
	}
	return r
}

// argGen 根据 IDL 类型自动生成 args，并收集需要签名的 signer 账户。
type argGen struct {
	pd    *provider.Provider
	accs  map[string]*testAccount // 每个 signer 参数名 -> 独立账户
	depth int                     // 递归深度保护，防止循环引用的自定义类型导致栈溢出
}

const maxGenDepth = 20

func (g *argGen) build(ix provider.Instruction) (provider.Args, []*testAccount, error) {
	args := provider.Args{}
	signers := []*testAccount{}
	for _, a := range ix.Args {
		val, signer, err := g.genArg(a.Name, a.Role, a.Type)
		if err != nil {
			return nil, nil, err
		}
		if signer != nil {
			signers = append(signers, signer)
		}
		args[a.Name] = val
	}

	// signer_lookups 中的 key（如 owner/freezer/admin）也是链端要求签名的账户。
	// 为它们生成独立的测试账户并加入签名者集合，避免报 "地址未参与当前交易签名"。
	// 注意：lookup 是从链上账户数据解析（随机地址解析出的可能是空地址），
	// 这里为每个 lookup key 提供一个测试签名，尽量覆盖签名需求。
	lookupKeys := make([]string, 0, len(ix.SignerLookups))
	for key := range ix.SignerLookups {
		lookupKeys = append(lookupKeys, key)
	}
	sort.Strings(lookupKeys)
	for _, key := range lookupKeys {
		if _, ok := g.accs[key]; ok {
			continue
		}
		acc, err := newTestAccount()
		if err != nil {
			return nil, nil, err
		}
		g.accs[key] = acc
		signers = append(signers, acc)
	}

	return args, signers, nil
}

// genArg 生成单个参数值。若该参数是 signer 角色，返回对应的签名账户。
func (g *argGen) genArg(name, role, typeStr string) (any, *testAccount, error) {
	// signer 角色：生成（或复用）一个账户，并记录为签名者
	if role == "signer" || role == "any_signer" {
		if acc, ok := g.accs[name]; ok {
			return acc.addr, acc, nil
		}
		acc, err := newTestAccount()
		if err != nil {
			return nil, nil, err
		}
		g.accs[name] = acc
		return acc.addr, acc, nil
	}

	// input 角色：地址类型用主测试账户，其余按类型生成
	if typeStr == "Address" || typeStr == "Signer" || typeStr == "AnySigner" {
		acc, err := newTestAccount()
		if err != nil {
			return nil, nil, err
		}
		return acc.addr, nil, nil
	}

	// String 参数根据参数名生成更合理的值，避免链端业务校验失败
	if typeStr == "String" || typeStr == "string" {
		return stringArgValue(name), nil, nil
	}

	v, err := g.genTypeValue(typeStr)
	return v, nil, err
}

// stringArgValue 根据参数名生成一个能通过链端基本校验的字符串值。
func stringArgValue(name string) string {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "endpoint"), strings.Contains(n, "uri"),
		strings.Contains(n, "icon"), strings.Contains(n, "url"),
		strings.Contains(n, "base_uri"), strings.Contains(n, "network_address"),
		strings.Contains(n, "avatar"), strings.Contains(n, "http"):
		return "https://example.com/milon-test"
	case strings.Contains(n, "credential"), strings.Contains(n, "schema"),
		strings.Contains(n, "credential_id"), strings.Contains(n, "vc"):
		return "AFJ8w5zXQ1mN2cV9bY6rT0pE4sK7uD3" // 合法 Base58
	case strings.Contains(n, "order"):
		return "order-test-0001"
	case strings.Contains(n, "label"), strings.Contains(n, "alias"), strings.Contains(n, "name"):
		return "test-label"
	default:
		return "test-string"
	}
}

// genTypeValue 为给定 IDL 类型生成一个合法的默认值（无 signer 语义）。
func (g *argGen) genTypeValue(typeStr string) (any, error) {
	g.depth++
	defer func() { g.depth-- }()
	if g.depth > maxGenDepth {
		// 递归过深：极可能是循环引用的自定义类型，返回空值避免栈溢出
		return map[string]any{}, nil
	}
	switch typeStr {
	case "u8":
		return uint8(1), nil
	case "u16":
		return uint16(1), nil
	case "u32":
		return uint32(1), nil
	case "u64", "Amount", "Epoch":
		return uint64(1000), nil
	case "i8":
		return int8(1), nil
	case "i16":
		return int16(1), nil
	case "i32":
		return int32(1), nil
	case "i64":
		return int64(1), nil
	case "u128":
		return uint64(1), nil // asBigInt 接受 uint64 等
	case "bool", "boolean":
		return true, nil
	case "String", "string":
		return "test-string", nil
	case "bytes":
		return []byte{1, 2, 3, 4, 5}, nil
	case "B96":
		return make([]byte, 12), nil
	case "B144":
		return make([]byte, 18), nil
	case "B160":
		return make([]byte, 20), nil
	case "B256":
		return make([]byte, 32), nil
	case "PublicKey":
		acc, err := newTestAccount()
		if err != nil {
			return nil, err
		}
		return acc.pk, nil
	case "Bitmap64":
		return uint64(0), nil
	case "Address", "Signer", "AnySigner":
		acc, err := newTestAccount()
		if err != nil {
			return nil, err
		}
		return acc.addr, nil
	}

	// 包裹类型
	if inner, ok := parseWrapped(typeStr, "vec"); ok {
		// vec<X>: 生成含一个元素的数组
		item, err := g.genTypeValue(inner)
		if err != nil {
			return nil, err
		}
		return []any{item}, nil
	}
	if _, ok := parseWrapped(typeStr, "option"); ok {
		// option<X>: 返回 nil 表示 None
		return nil, nil
	}
	if _, _, ok, err := parseMap(typeStr); err != nil {
		return nil, err
	} else if ok {
		// map<K,V>: 空 map
		return map[string]any{}, nil
	}
	if tps, ok, err := parseTuple(typeStr); err != nil {
		return nil, err
	} else if ok {
		// tuple<T1,T2,...>: 为每个元素生成一个值
		items := make([]any, 0, len(tps))
		for _, tp := range tps {
			item, err := g.genTypeValue(strings.TrimSpace(tp))
			if err != nil {
				return nil, err
			}
			items = append(items, item)
		}
		return items, nil
	}

	// 自定义 IDL 类型（struct/enum/builtin）
	if idlType, ok := g.pd.IDLTypeByName[typeStr]; ok {
		return g.genCustomType(idlType)
	}

	return nil, fmt.Errorf("unsupported type: %s", typeStr)
}

// genCustomType 为自定义 struct/enum/unit 生成默认值。
func (g *argGen) genCustomType(t provider.IDLType) (any, error) {
	switch t.Kind {
	case "struct":
		obj := map[string]any{}
		for _, f := range t.Fields {
			v, _, err := g.genArg(f.Name, "input", f.Type)
			if err != nil {
				return nil, err
			}
			obj[f.Name] = v
		}
		return obj, nil
	case "enum":
		if len(t.Variants) > 0 {
			// 第一个 unit variant
			v := t.Variants[0]
			if len(v.Fields) == 0 {
				return map[string]any{v.Name: nil}, nil
			}
			// 带字段的 variant：生成默认字段
			inner := map[string]any{}
			for _, f := range v.Fields {
				val, _, err := g.genArg(f.Name, "input", f.Type)
				if err != nil {
					return nil, err
				}
				inner[f.Name] = val
			}
			return map[string]any{v.Name: inner}, nil
		}
		return map[string]any{}, nil
	case "unit":
		return nil, nil
	case "builtin":
		// 直接按 builtin 类型生成，避免对 Address 等再递归回 genTypeValue
		return g.genTypeValue(t.Name)
	default:
		return map[string]any{}, nil
	}
}

// ---------- 简单的包裹类型解析（与 provider 对齐） ----------

func parseWrapped(typeStr, wrapper string) (string, bool) {
	prefix := wrapper + "<"
	if strings.HasPrefix(typeStr, prefix) && strings.HasSuffix(typeStr, ">") {
		return typeStr[len(prefix) : len(typeStr)-1], true
	}
	return "", false
}

func parseMap(typeStr string) (string, string, bool, error) {
	if !strings.HasPrefix(typeStr, "map<") || !strings.HasSuffix(typeStr, ">") {
		return "", "", false, nil
	}
	inner := typeStr[4 : len(typeStr)-1]
	idx := indexTopLevelComma(inner)
	if idx < 0 {
		return "", "", false, fmt.Errorf("invalid map type: %s", typeStr)
	}
	return inner[:idx], inner[idx+1:], true, nil
}

func parseTuple(typeStr string) ([]string, bool, error) {
	if !strings.HasPrefix(typeStr, "tuple<") || !strings.HasSuffix(typeStr, ">") {
		return nil, false, nil
	}
	inner := typeStr[6 : len(typeStr)-1]
	if inner == "" {
		return nil, true, nil
	}
	parts := splitTopLevel(inner)
	return parts, true, nil
}

func indexTopLevelComma(s string) int {
	depth := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '<', '[':
			depth++
		case '>', ']':
			depth--
		case ',':
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

func splitTopLevel(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '<', '[':
			depth++
		case '>', ']':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, s[start:i])
				start = i + 1
			}
		}
	}
	parts = append(parts, s[start:])
	return parts
}
