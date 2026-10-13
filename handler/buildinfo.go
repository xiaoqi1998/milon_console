package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
)

// 构建指纹（spec: 2026-10-13 open-source verify）：
// 服务端自报 git commit / 构建时间 / 二进制 SHA256 / 开源仓库地址，
// 页面展示并提供「用户本地构建 → 比对哈希」的核验入口。
// 语义边界：自报指纹能发现「部署了未开源/被改动的构建」，
// 但防不住运营者连自报逻辑一起篡改——完整核验以用户本地构建为准。

// ldflags 注入的构建元数据兜底（Go VCS stamping 缺失时生效，
// 如 .git 不在构建上下文或 unsafe directory）：
//
//	go build -trimpath -ldflags="-s -w \
//	  -X milon-api-server/handler.buildGitCommit=$(git rev-parse HEAD) \
//	  -X milon-api-server/handler.buildBuildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var (
	buildGitCommit = ""
	buildBuildDate = ""
	buildRepoURL   = "https://github.com/xiaoqi1998/milon_console"
)

// repoURLEnv 覆盖 buildRepoURL 的环境变量名。
const repoURLEnv = "OPEN_SOURCE_REPO_URL"

// reproducibleBuildCommand 是页面展示的可复现构建命令：
// -trimpath 消除本机路径差异，CGO_ENABLED=0 排除 C 工具链差异，
// 同 commit + 同 Go 版本 + 同 GOOS/GOARCH 下哈希一致。
const reproducibleBuildCommand = `CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o milon-api-server .`

// BuildInfo 是 /api/version 返回的构建指纹快照。
type BuildInfo struct {
	GitCommit    string `json:"gitCommit"`
	GitDirty     bool   `json:"gitDirty"`
	BuildDate    string `json:"buildDate"`
	GoVersion    string `json:"goVersion"`
	Goos         string `json:"goos"`
	Goarch       string `json:"goarch"`
	BinarySHA256 string `json:"binarySHA256"`
	RepoURL      string `json:"repoURL"`
	BuildCommand string `json:"buildCommand"`
}

var (
	buildInfoOnce sync.Once
	buildInfoVal  *BuildInfo
)

// GetBuildInfo 返回进程级构建指纹（首次调用时对自身二进制计算 SHA256 并缓存）。
func GetBuildInfo() BuildInfo {
	buildInfoOnce.Do(func() {
		commit, dirty := vcsStamping()
		exe, err := os.Executable()
		binSHA := "unknown"
		if err == nil {
			if sum, err := fileSHA256(exe); err == nil {
				binSHA = sum
			}
		}
		buildInfoVal = &BuildInfo{
			GitCommit:    resolveGitCommit(commit, buildGitCommit),
			GitDirty:     resolveGitDirty(dirty),
			BuildDate:    orUnknown(strings.TrimSpace(buildBuildDate)),
			GoVersion:    runtime.Version(),
			Goos:         runtime.GOOS,
			Goarch:       runtime.GOARCH,
			BinarySHA256: binSHA,
			RepoURL:      resolveRepoURL(os.Getenv(repoURLEnv), buildRepoURL),
			BuildCommand: reproducibleBuildCommand,
		}
	})
	return *buildInfoVal
}

// vcsStamping 读 Go 构建时自动嵌入的 VCS 信息（vcs.revision/vcs.modified）。
func vcsStamping() (revision, modified string) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "", ""
	}
	for _, kv := range info.Settings {
		switch kv.Key {
		case "vcs.revision":
			revision = kv.Value
		case "vcs.modified":
			modified = kv.Value
		}
	}
	return revision, modified
}

// resolveGitCommit VCS stamping 优先，ldflags 兜底，均缺则 unknown。
func resolveGitCommit(vcsRevision, ldflagsCommit string) string {
	if s := strings.TrimSpace(vcsRevision); s != "" {
		return s
	}
	if s := strings.TrimSpace(ldflagsCommit); s != "" {
		return s
	}
	return "unknown"
}

// resolveGitDirty 仅认 VCS stamping 的 "true"。
func resolveGitDirty(vcsModified string) bool {
	return strings.TrimSpace(vcsModified) == "true"
}

// resolveRepoURL env 优先，ldflags/默认值兜底；空白视为未设置。
func resolveRepoURL(envVal, fallbackVal string) string {
	if s := strings.TrimSpace(envVal); s != "" {
		return s
	}
	if s := strings.TrimSpace(fallbackVal); s != "" {
		return s
	}
	return "unknown"
}

// fileSHA256 计算文件内容的 SHA256 hex。
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func orUnknown(s string) string {
	if s == "" {
		return "unknown"
	}
	return s
}
