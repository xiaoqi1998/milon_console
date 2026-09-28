package handler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// IDL JSON 目录与运行时 provider 加载的是同一份文件。
const idlTestDir = "../gosdk-develop/provider/IDL"

type idlTestApp struct {
	Metadata struct {
		AppID uint8  `json:"app_id"`
		Name  string `json:"name"`
	} `json:"metadata"`
	Instructions []struct {
		Name string `json:"name"`
		Kind string `json:"kind"`
		Args []struct {
			Name string `json:"name"`
			Type string `json:"type"`
			Role string `json:"role"`
		} `json:"args"`
	} `json:"instructions"`
}

func loadIDLTestApps(t *testing.T) map[string]*idlTestApp {
	t.Helper()
	entries, err := os.ReadDir(idlTestDir)
	if err != nil {
		t.Fatalf("读取 IDL 目录失败: %v", err)
	}
	apps := map[string]*idlTestApp{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".idl.json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(idlTestDir, name))
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", name, err)
		}
		var app idlTestApp
		if err := json.Unmarshal(raw, &app); err != nil {
			t.Fatalf("解析 %s 失败: %v", name, err)
		}
		apps[strings.TrimSuffix(name, ".idl.json")] = &app
	}
	return apps
}

// TestIDLDocsCoverAllInstructions 保证每个 IDL 方法都有中文说明、每个参数都有中文参数说明，
// 且文档映射里没有指向已不存在的方法/参数的失效键（gosdk 同步后跑这里即可定位要增删的条目）。
func TestIDLDocsCoverAllInstructions(t *testing.T) {
	apps := loadIDLTestApps(t)
	wantMethods := map[string]bool{}
	wantArgs := map[string]bool{}
	totalIx, totalArgs := 0, 0
	for appName, app := range apps {
		for _, ix := range app.Instructions {
			totalIx++
			mk := appName + "." + ix.Name
			wantMethods[mk] = true
			if d := idlMethodDocs[mk]; strings.TrimSpace(d) == "" {
				t.Errorf("缺少方法文档: %s", mk)
			}
			for _, a := range ix.Args {
				totalArgs++
				ak := mk + ":" + a.Name
				wantArgs[ak] = true
				if d := idlArgDocs[ak]; strings.TrimSpace(d) == "" {
					t.Errorf("缺少参数文档: %s", ak)
				}
			}
		}
	}

	var staleMethods, staleArgs []string
	for k := range idlMethodDocs {
		if !wantMethods[k] {
			staleMethods = append(staleMethods, k)
		}
	}
	for k := range idlArgDocs {
		if !wantArgs[k] {
			staleArgs = append(staleArgs, k)
		}
	}
	if len(staleMethods) > 0 {
		sort.Strings(staleMethods)
		t.Errorf("文档映射存在失效方法键（IDL 已不含这些方法）: %s", strings.Join(staleMethods, ", "))
	}
	if len(staleArgs) > 0 {
		sort.Strings(staleArgs)
		t.Errorf("文档映射存在失效参数键: %s", strings.Join(staleArgs, ", "))
	}

	if totalIx != len(idlMethodDocs) {
		t.Errorf("方法文档条数 %d != IDL 指令数 %d（可能有重名键被覆盖）", len(idlMethodDocs), totalIx)
	}
	if totalArgs != len(idlArgDocs) {
		t.Errorf("参数文档条数 %d != IDL 参数总数 %d（可能有重名键被覆盖）", len(idlArgDocs), totalArgs)
	}
}

// TestDumpIDLDocs 平时不执行：设置环境变量 MILON_DUMP_IDL_DOCS=<输出路径> 时，
// 把方法/参数文档映射导出为 JSON，供脚本重生成 IDL_FUNCTIONS.md 的说明列。
func TestDumpIDLDocs(t *testing.T) {
	out := os.Getenv("MILON_DUMP_IDL_DOCS")
	if out == "" {
		t.Skip("设置 MILON_DUMP_IDL_DOCS 后才导出")
	}
	payload := map[string]any{
		"methods": idlMethodDocs,
		"args":    idlArgDocs,
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if err := os.WriteFile(out, raw, 0644); err != nil {
		t.Fatalf("写出失败: %v", err)
	}
	t.Logf("已导出文档映射到 %s", out)
}
