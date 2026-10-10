package handler

// 2026-10 AI 提速（Step 4）：idl_methods 附 exampleArgs 调用模板。
// AI 此前要按参数表自行拼 args 对象；模板可直接填空，配合缺参一次性报出，
// 调用合约从"试错 N 轮"压缩到一轮。

import (
	"reflect"
	"testing"
)

// TestExampleArgsTemplate：token.Transfer 的 exampleArgs 必须覆盖全部参数、
// 类型合理（数值 0、地址占位、vec 为数组）。
func TestExampleArgsTemplate(t *testing.T) {
	pd := tokenProvider(t)
	meta := buildAppMeta("token", pd)
	var transfer *idlInstructionMeta
	for i := range meta.Instructions {
		if meta.Instructions[i].Name == "Transfer" {
			transfer = &meta.Instructions[i]
			break
		}
	}
	if transfer == nil {
		t.Fatal("token.Transfer 不在 IDL 中")
	}
	if transfer.ExampleArgs == nil {
		t.Fatalf("Transfer 应带 exampleArgs: %+v", transfer)
	}
	for _, a := range transfer.Args {
		if _, ok := transfer.ExampleArgs[a.Name]; !ok {
			t.Errorf("exampleArgs 缺参数 %s: %v", a.Name, transfer.ExampleArgs)
		}
	}
	// 数值参数给 0，地址参数给占位符
	if v, ok := transfer.ExampleArgs["amount"].(float64); !ok || v != 0 {
		t.Errorf("amount 应为数值 0, got %v (%T)", transfer.ExampleArgs["amount"], transfer.ExampleArgs["amount"])
	}
	addr, ok := transfer.ExampleArgs["to"].(string)
	if !ok || addr == "" {
		t.Errorf("to 应为地址占位字符串, got %v", transfer.ExampleArgs["to"])
	}
}

// TestExampleArgsTypeMapping：类型映射核心规则。
func TestExampleArgsTypeMapping(t *testing.T) {
	cases := []struct {
		typ  string
		want any
	}{
		{"u64", float64(0)},
		{"u8", float64(0)},
		{"i64", float64(0)},
		{"bool", false},
		{"String", "<字符串>"},
		{"Address", "<base58地址>"},
		{"vec<u64>", []any{float64(0)}},
		{"option<u64>", nil},
		{"map<String, u64>", map[string]any{"<key>": float64(0)}},
	}
	for _, c := range cases {
		if got := exampleValueForType(c.typ); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %#v want %#v", c.typ, got, c.want)
		}
	}
}
