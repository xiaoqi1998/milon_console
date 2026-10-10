package types

// 2026-10 AI 易用性修复（Step B）：signatureMode 解析宽容化。
// AI 真实使用中的三类高频失败此前都会得到晦涩报错：
//  1. 把对象二次序列化成字符串再传（"{\"type\":\"pubkey\",...}"）；
//  2. 省略 type 字段只给 publicKey（pubkey 语义不言自明）；
//  3. 完全不传/传空（此前报 "unexpected end of JSON input"）。
// 修复后：1/2 直接解析成功；3 与其余解析失败统一附正确形态示例，AI 可自纠。

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/milon-labs/milon-go-sdk/lib"
)

const testPubKey = "0x02f175c4673255dc8e674d70c3d6d45550ccebafb745fbd386fa52ce6f74bb2ef6"

func mustParse(t *testing.T, raw string) lib.AccountSignatureMode {
	t.Helper()
	mode, err := ParseSignatureModeFromJSON(json.RawMessage(raw))
	if err != nil {
		t.Fatalf("parse %s: %v", raw, err)
	}
	return mode
}

// TestParseStringifiedSignatureMode：对象被序列化成字符串再传，应剥壳解析。
func TestParseStringifiedSignatureMode(t *testing.T) {
	mode := mustParse(t, `"{\"type\":\"pubkey\",\"publicKey\":\"`+testPubKey+`\"}"`)
	if _, ok := mode.(lib.PubKeySignatureMode); !ok {
		t.Fatalf("want PubKeySignatureMode, got %T", mode)
	}
}

// TestParseSignatureModeDefaultsToPubkey：缺省 type 视为 pubkey。
func TestParseSignatureModeDefaultsToPubkey(t *testing.T) {
	mode := mustParse(t, `{"publicKey":"`+testPubKey+`"}`)
	if _, ok := mode.(lib.PubKeySignatureMode); !ok {
		t.Fatalf("want PubKeySignatureMode, got %T", mode)
	}
	// 显式空串 type 与缺省同义
	mode = mustParse(t, `{"type":"","publicKey":"`+testPubKey+`"}`)
	if _, ok := mode.(lib.PubKeySignatureMode); !ok {
		t.Fatalf("want PubKeySignatureMode, got %T", mode)
	}
}

// TestParseSignatureModeErrorHasExample：解析失败必须附正确形态示例。
func TestParseSignatureModeErrorHasExample(t *testing.T) {
	cases := []string{
		``,                                    // 完全为空（此前报 unexpected end of JSON input）
		`{"type":"weird","publicKey":"x"}`,    // 不支持的 type
		`{"type":"multisig","index":0}`,       // 缺 publicKey
		`{"type":"multisig","publicKey":"x"}`, // multisig 缺 index
	}
	for _, raw := range cases {
		_, err := ParseSignatureModeFromJSON(json.RawMessage(raw))
		if err == nil {
			t.Errorf("%q 应解析失败", raw)
			continue
		}
		if !strings.Contains(err.Error(), `"type":"pubkey"`) || !strings.Contains(err.Error(), `"type":"multisig"`) {
			t.Errorf("%q 的报错未附示例: %v", raw, err)
		}
	}
}
