package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// ==================== resolveGitCommit ====================

// TestResolveGitCommitPrefersVCS VCS stamping 优先于 ldflags 兜底。
func TestResolveGitCommitPrefersVCS(t *testing.T) {
	got := resolveGitCommit("0123456789abcdef", "ffffffffffffffff")
	if got != "0123456789abcdef" {
		t.Errorf("resolveGitCommit = %q, want vcs revision", got)
	}
}

// TestResolveGitCommitFallsBackToLdflags VCS stamping 缺失时用 ldflags 注入值。
func TestResolveGitCommitFallsBackToLdflags(t *testing.T) {
	got := resolveGitCommit("", "aaaaaaaaaaaaaaaa")
	if got != "aaaaaaaaaaaaaaaa" {
		t.Errorf("resolveGitCommit = %q, want ldflags value", got)
	}
}

// TestResolveGitCommitUnknown 两处都缺时返回 unknown（.git 缺失/非 git 构建）。
func TestResolveGitCommitUnknown(t *testing.T) {
	if got := resolveGitCommit("", ""); got != "unknown" {
		t.Errorf("resolveGitCommit = %q, want unknown", got)
	}
}

// TestResolveGitCommitTrims 空白输入视为缺失。
func TestResolveGitCommitTrims(t *testing.T) {
	if got := resolveGitCommit("  ", " \t "); got != "unknown" {
		t.Errorf("resolveGitCommit = %q, want unknown(空白视为缺失)", got)
	}
}

// ==================== resolveGitDirty ====================

// TestResolveGitDirty 空白视为 false。
func TestResolveGitDirty(t *testing.T) {
	if resolveGitDirty("true") != true {
		t.Error(`resolveGitDirty("true") = false, want true`)
	}
	if resolveGitDirty("false") != false {
		t.Error(`resolveGitDirty("false") = true, want false`)
	}
	if resolveGitDirty("") != false {
		t.Error(`resolveGitDirty("") = true, want false`)
	}
	if resolveGitDirty("yes") != false {
		t.Error(`resolveGitDirty("yes") = true, want false(仅认 "true")`)
	}
}

// ==================== resolveRepoURL ====================

// TestResolveRepoURLEnvOverrides env 优先于 ldflags/默认值。
func TestResolveRepoURLEnvOverrides(t *testing.T) {
	got := resolveRepoURL("https://gitlab.com/x/y", "https://github.com/a/b")
	if got != "https://gitlab.com/x/y" {
		t.Errorf("resolveRepoURL = %q, want env 值", got)
	}
}

// TestResolveRepoURLFallsBack env 为空时用 ldflags/默认值。
func TestResolveRepoURLFallsBack(t *testing.T) {
	got := resolveRepoURL("", "https://github.com/a/b")
	if got != "https://github.com/a/b" {
		t.Errorf("resolveRepoURL = %q, want ldflags 值", got)
	}
}

// TestResolveRepoURLDefault 全空时落到内置默认（本仓库 git remote）。
func TestResolveRepoURLDefault(t *testing.T) {
	if !strings.HasPrefix(buildRepoURL, "https://github.com/") {
		t.Errorf("buildRepoURL = %q, want 以 https://github.com/ 开头", buildRepoURL)
	}
	got := resolveRepoURL("", buildRepoURL)
	if got != buildRepoURL {
		t.Errorf("resolveRepoURL = %q, want %q", got, buildRepoURL)
	}
}

// ==================== fileSHA256 ====================

// TestFileSHA256MatchesKnownHash 对已知内容校验哈希正确性。
func TestFileSHA256MatchesKnownHash(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bin")
	content := []byte("milon-reproducible-build")
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	want := hex.EncodeToString(sum[:])
	got, err := fileSHA256(p)
	if err != nil {
		t.Fatalf("fileSHA256: %v", err)
	}
	if got != want {
		t.Errorf("fileSHA256 = %q, want %q", got, want)
	}
}

// TestFileSHA256MissingFile 文件不存在时报错而非返回假哈希。
func TestFileSHA256MissingFile(t *testing.T) {
	if _, err := fileSHA256(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("期望对缺失文件返回 error")
	}
}

// ==================== GET /api/version ====================

// TestVersionEndpoint 端点契约：200、字段齐全、格式合法。
func TestVersionEndpoint(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewSystemHandler(nil)
	r := gin.New()
	r.GET("/api/version", h.Version)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			GitCommit    string `json:"gitCommit"`
			GitDirty     bool   `json:"gitDirty"`
			BuildDate    string `json:"buildDate"`
			GoVersion    string `json:"goVersion"`
			Goos         string `json:"goos"`
			Goarch       string `json:"goarch"`
			BinarySHA256 string `json:"binarySHA256"`
			RepoURL      string `json:"repoURL"`
			BuildCommand string `json:"buildCommand"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v, body = %s", err, w.Body.String())
	}
	if !resp.Success {
		t.Fatalf("success = false, body = %s", w.Body.String())
	}

	d := resp.Data
	if d.GoVersion == "" {
		t.Error("goVersion 为空")
	}
	if d.Goos == "" || d.Goarch == "" {
		t.Error("goos/goarch 为空")
	}
	if !strings.HasPrefix(d.RepoURL, "https://") {
		t.Errorf("repoURL = %q, want https:// 开头", d.RepoURL)
	}
	if !strings.Contains(d.BuildCommand, "-trimpath") {
		t.Errorf("buildCommand = %q, 应包含 -trimpath(可复现构建)", d.BuildCommand)
	}
	// 锁定 cgo 必需（blst BLS 依赖）：关掉 cgo 会构建失败，禁止再回退成 0
	if !strings.Contains(d.BuildCommand, "CGO_ENABLED=1") {
		t.Errorf("buildCommand = %q, 应包含 CGO_ENABLED=1(blst 需要 cgo)", d.BuildCommand)
	}
	if !isHex64OrUnknown(d.BinarySHA256) {
		t.Errorf("binarySHA256 = %q, want 64 位 hex 或 unknown", d.BinarySHA256)
	}
	// gitCommit 同样：40 位 hex 或显式 unknown
	if !isHexOrUnknown(d.GitCommit, 40) {
		t.Errorf("gitCommit = %q, want 40 位 hex 或 unknown", d.GitCommit)
	}
}

func isHex64OrUnknown(s string) bool { return isHexOrUnknown(s, 64) }

func isHexOrUnknown(s string, n int) bool {
	if s == "unknown" {
		return true
	}
	if len(s) != n {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}
