package mcpserver

// resources.go：MCP resources 摆货（审计整改第 3 刀）。
// 审计指控"resources 能力空转——接口在、内容是空的"。本文件把两类真实
// 数据源挂上 URI：
//   - 本地资源（不发网络请求）：milon://tools 工具清单、milon://guide/{topic}
//     手册页——AI 可拉取式获取，不占工具调用帧
//   - 回环资源（复用 Executor 走 REST）：milon://network、milon://idl/apps、
//     milon://idl/apps/{app}——后者是 idl_metadata 200KB 上下文炸弹的解法：
//     资源是按需拉取的，agent 按名取单个 app 的方法明细，而不是整包元数据
//     灌进对话。

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// RegisterResources 注册全部资源与资源模板。静态资源在前、模板在后；
// 模板的 URI 变量从 req.Params.URI 前缀剥离（SDK 不代解析模板变量）。
func RegisterResources(srv *mcp.Server, exec *Executor) {
	// ---- 静态资源 ----

	// milon://tools：工具清单（name/description/group/method/path）。
	// 与前端「MCP 接入」页同源（ToolInventory），agent 可静态拉全量地图。
	srv.AddResource(&mcp.Resource{
		URI:         "milon://tools",
		Name:        "tools",
		Title:       "MCP 工具清单",
		Description: "全部 MCP 工具的清单（名称/描述/分组/HTTP 方法/REST 路径），与 guide overview 互补的机器可读版",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		b, err := json.MarshalIndent(ToolInventory(), "", "  ")
		if err != nil {
			return nil, err
		}
		return textResource("milon://tools", "application/json", string(b)), nil
	})

	// milon://network：当前默认网络（回环 GET /api/network/current）。
	srv.AddResource(&mcp.Resource{
		URI:         "milon://network",
		Name:        "network",
		Title:       "当前网络",
		Description: "服务端当前默认网络（devNet/localNet）；请求级网络由 X-Milon-Network 头覆盖",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return readRESTResource(ctx, exec, RESTMapping{Method: "GET", PathTemplate: "/api/network/current"}, "milon://network", nil)
	})

	// milon://idl/apps：IDL app 轻量清单（回环 GET /api/idl/apps）。
	srv.AddResource(&mcp.Resource{
		URI:         "milon://idl/apps",
		Name:        "idl-apps",
		Title:       "IDL app 清单",
		Description: "全部 IDL app 的轻量清单（名称/简介/方法数）——取单 app 明细用 milon://idl/apps/{app}",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return readRESTResource(ctx, exec, RESTMapping{Method: "GET", PathTemplate: "/api/idl/apps"}, "milon://idl/apps", nil)
	})

	// ---- 资源模板 ----

	// milon://idl/apps/{app}：单个 app 的方法明细（回环 GET /api/idl/apps/{app}/methods）。
	// idl_metadata 工具全量约 200KB 易撑爆上下文；资源是拉取式——按 app 名
	// 精确取一份，agent 上下文只进需要的部分。
	srv.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "milon://idl/apps/{app}",
		Name:        "idl-app-methods",
		Title:       "IDL app 方法明细",
		Description: "单个 IDL app 的全量方法明细（参数/返回值/signer 角色/错误码），等价 idl_methods 工具；如 milon://idl/apps/token",
		MIMEType:    "application/json",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri := req.Params.URI
		app := strings.TrimPrefix(uri, "milon://idl/apps/")
		if app == "" || app == uri || strings.ContainsAny(app, "/?#") {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		// executor 按 PathParams 名取参：appName 从模板变量注入
		args, err := json.Marshal(map[string]string{"appName": app})
		if err != nil {
			return nil, err
		}
		return readRESTResource(ctx, exec, RESTMapping{
			Method:       "GET",
			PathTemplate: "/api/idl/apps/{appName}/methods",
			PathParams:   []string{"appName"},
		}, uri, args)
	})

	// milon://guide/{topic}：手册页（本地，六个主题）。
	srv.AddResourceTemplate(&mcp.ResourceTemplate{
		URITemplate: "milon://guide/{topic}",
		Name:        "guide-page",
		Title:       "快速手册页",
		Description: "guide 六主题页（overview/chain/workflows/limits/performance/keys），与 guide 工具同源",
		MIMEType:    "text/markdown",
	}, func(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		uri := req.Params.URI
		topic := strings.TrimPrefix(uri, "milon://guide/")
		page, ok := guidePage(topic, "")
		if topic == "" || topic == uri || !ok {
			return nil, mcp.ResourceNotFoundError(uri)
		}
		return textResource(uri, "text/markdown", page), nil
	})
}

// textResource 构造单条文本资源结果。
func textResource(uri, mimeType, text string) *mcp.ReadResourceResult {
	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{URI: uri, MIMEType: mimeType, Text: text}},
	}
}

// readRESTResource 走回环 REST 取资源正文（JSON 原文透传）；args 为路径/
// query 参数的 JSON 对象（无参传 nil）。REST 失败按错误返回：资源读取没有
// "业务信封"层，错误即协议错误。
func readRESTResource(ctx context.Context, exec *Executor, m RESTMapping, uri string, args json.RawMessage) (*mcp.ReadResourceResult, error) {
	out, err := exec.Call(ctx, m, args)
	if err != nil {
		return nil, err
	}
	if out.IsError {
		return nil, fmt.Errorf("REST %s %s 不可用: %s", m.Method, m.PathTemplate, out.Body)
	}
	return textResource(uri, "application/json", string(out.Body)), nil
}
