# milon-api-server 请求级网络参数设计（方案 A）

- 日期：2026-10-09
- 状态：已与用户对齐，待实施
- 类型：架构级（改变所有链上端点的网络解析方式）

## 背景与动机

当前网络选择是**服务端进程级全局状态**：`client.NetworkManager` 持有单一 `currentNetwork` 字段，`POST /api/network/switch` 一改全员生效；handler 层 35 处 `nm.GetCurrent()`、MCP 回环、Web 前端顶栏选择器全部跟随这个全局。devNet 单人多机使用的当下没有问题，但主网/测试网上线后：A 切主网会让 B 正在 devNet 发的交易发错网络。

**关键决策记录**（与用户逐项确认）：

1. 方向：**方案 A——请求级网络参数**（否决了部署隔离过渡方案 B「每 API key 绑网络」的多租户方案，后者等账号体系再议）。
2. 传参形式：HTTP 头 `X-Milon-Network`（否决 query 参数与每网路由前缀）。
3. `network_switch` **保留**，语义改为「设置服务端默认网络」——只影响未携带网络头的请求，现有脚本/调用方零破坏。
4. MCP 侧每个工具获得可选 `network` 参数（schema 层面），executor 回环时转为请求头。
5. 前端顶栏选择器改为本地偏好（localStorage），不再调用 `network_switch`。

## 目标

- 多用户共享同一 server 实例时，各自请求可指定不同网络（`X-Milon-Network: devNet` / `localNet`，未来 `mainNet`），互不干扰。
- 未携带网络头的请求行为与现状完全一致（走服务端默认网络）。
- MCP 的 57 个工具均支持可选 `network` 参数，AI 代理可按需指定。
- 消除「切网络影响所有人」的全局竞态，水平扩展不再被网络状态阻塞。

## 非目标

- 不做多租户/账号体系/API key 绑定。
- 不做动态注册网络的管理 API（网络集合仍由代码内置 + `MILON_RPC_URL` 覆盖 devNet 端点；mainNet 等 gosdk 提供常量时加一行即可）。
- 不移除 `network_switch` / `network_current` / `network_list` 端点及其 MCP 工具。
- 不改任何端点的 body/path/query 契约（网络只走头）。

## 架构

```
REST 客户端                    MCP 客户端（AI 代理）           Web 前端
  │ X-Milon-Network: devNet      │ 工具参数 network=devNet       │ localStorage 偏好
  ▼                              ▼ (schema 可选字段)             ▼
gin /api 组 ── ResolveNetwork 中间件        mcpserver.registerTool[networkScoped[In]]
  │ 读头 → nm.ClientFor(name)                │ 抽出 network 键 → 回环请求加头
  │ c.Set 注入 client                         ▼
  ▼                              executor.Call ──POST──▶ gin /api（同进程，带头）
middleware.ClientFrom(c)                     （进入左侧同一中间件路径）
  ▼
35 处 handler 调用点（替换原 nm.GetCurrent()）
```

缺省路径：请求不带头 → `ClientFor("")` → 服务端默认网络（启动配置或 `network_switch` 所设）。

## 组件设计

### 1. client 层（`client/network_manager.go`）

新增：

```go
// ClientFor 返回指定网络的客户端；name 为空时回落到默认网络（currentNetwork，
// 即启动配置或 network_switch 所设），未知网络名返回错误。client 按网缓存复用。
func (nm *NetworkManager) ClientFor(name string) (*milon.Client, milon.Network, error)
```

- 内部复用现有 `getOrCreateClient`（已有缓存与 panic-recover 逻辑）。
- `Switch` / `GetCurrent` / `ListNetworks` 原样保留；文档语义更新为「默认网络」。

### 2. middleware 层（新文件 `middleware/network.go`）

```go
func ResolveNetwork(nm *client.NetworkManager) gin.HandlerFunc  // 挂 /api 组
func ClientFrom(c *gin.Context) *milon.Client                    // handler 侧取 client
```

- 中间件逻辑：读 `X-Milon-Network` 头 → `ClientFor` → `c.Set("milon_network_client", mc)` + `c.Set("milon_network_name", name)`；空头直接注入默认网络 client（不报错）。
- 未知网络名：`c.AbortWithStatusJSON(400, types.ErrorResponse(types.ERR_INVALID_PARAMETER, "unknown network: xxx", nil))`。
- `/api/network/list|current|switch` 三个端点无需 client，但中间件对它们无害（不带头走默认解析，不触发副作用），统一挂载不做豁免，换取简单。
- import 方向：`handler → middleware → client`，无环。

### 3. handler 层（18 个文件，机械替换）

35 处 `mc, _ := h.nm.GetCurrent()` → `mc := middleware.ClientFrom(c)`。个别需要网络名的（如日志/回显）用 `middleware.NetworkNameFrom(c)`。`handler/network.go` 的 `GetCurrentNetwork` 改为展示**默认网络**（数据源 `nm.GetCurrent()` 不变，仅文案与文档更新）。

### 4. MCP 层（`mcpserver/tools.go` + `executor.go`）

- `registerTool[In any]` 的 AddTool 输入类型包一层：

```go
type networkScoped[In any] struct {
    Network string `json:"network,omitempty" jsonschema:"可选：目标网络（devNet/localNet…），缺省用服务端默认网络"`
    In                                  // 嵌入，schema 字段被 inline 展开（前提见风险）
}
```

- 工具回调内：args 整体 marshal → unmarshal 成 `map[string]any` 抽出 `network` 键 → 余下重新 marshal 为原 argsJSON → `executor.Call` 签名增加 `network string`，回环 HTTP 请求带 `X-Milon-Network` 头。
- `network_switch` 工具的 jsonschema 描述同步改为「设置服务端默认网络（仅影响未携带 network 的请求）」。
- 工具清单导出（`ToolInventory`）不受影响：name/desc/REST 映射不变。

### 5. 前端（`static/js/app.js` + `saved-instructions.js`）

- 顶栏选择器：加载逻辑不变；回显顺序 = `localStorage['milonNetwork']` > 服务端默认（`/api/network/current`）；`change` 事件写 localStorage + toast「后续请求将携带 X-Milon-Network: xxx」，**不再调用** `/api/network/switch`。
- 统一发送路径（Playground 的 fetch 封装、saved-instructions 的请求函数）统一附加 `X-Milon-Network` 头（偏好为空则不带头，走服务端默认）。
- 静态资源版本号 bump（`?v=20261009` → `?v=20261010`），防 webview 旧缓存。

### 6. 文档

- `API.md`：「通用说明」新增「网络选择」小节（头用法、缺省行为、`network_switch` 新语义、示例 curl）；`network/switch` 条目说明更新；端点示例里可带 `-H 'X-Milon-Network: devNet'` 演示。
- `README.md`：MCP 章节与快速开始各补一句。
- `static/index.html` MCP 页工具清单区描述提一句 network 参数。

## 错误处理

| 场景 | 行为 |
|---|---|
| 头缺失 | 走服务端默认网络（与现状行为一致） |
| 头为已知网络名 | 按该网路由，client 懒创建并缓存 |
| 头为未知网络名 | 400 `ERR_INVALID_PARAMETER`，message 指明未知名 |
| 该网 client 创建失败 | 500（沿用 createClient 的 panic-recover 错误路径） |
| MCP 工具 network 参数未知 | 回环打到 REST 后被中间件 400，工具结果 `isError=true`（沿用现有错误透传） |

## 测试策略（TDD）

1. **client**：`ClientFor` 三态（空名回落默认 / 合法名返回对应网 client / 未知名报错）+ 同名两次调用返回同一实例（缓存）。
2. **middleware**：httptest 三态（带头路由正确 / 不带走默认 / 坏头 400）。
3. **网络路由 E2E（核心证明）**：两个 httptest fake RPC 后端分别充当 localNet 与 devNet（`MILON_RPC_URL` 指向其一），同一 REST 端点分别带不同头请求，断言请求落到对应 fake 后端。
4. **MCP schema**：tools/list 断言全部 57 个工具含非必填 `network` 字段且描述正确（同时锁死泛型嵌入的 schema 展开行为）。
5. **MCP 回环**：fake REST 后端断言带 `network` 参数的工具调用转成了 `X-Milon-Network` 头、且 body 中 `network` 键已剥离。
6. **前端**：浏览器实测——切换顶栏偏好后，Playground 请求带对应头（Network 面板/服务端日志确认）；刷新页面偏好保持。

## 风险与对策

- **go-sdk jsonschema 对泛型嵌入字段的展开**（方案 3 的前提）：实施第一步先写 schema 形状测试；若 SDK 不 inline 嵌入字段导致 schema 呈现嵌套对象，则退路为给 57 个 args struct 逐一加嵌入（机械但可行），或 wrapper 改为显式字段复制。
- **executor 剥键重编**：REST handler 以 map 语义读 body，字段顺序无关；`signatureMode` 等 `any` 透传字段经 marshal/unmarshal 往返后 JSON 值不变。用既有 `TestSchemaRequiredMatchesREST` 回归。
- **遗漏 fetch 点**：前端凡发 `/api/*` 的代码分散在 app.js 与 saved-instructions.js，实施时 grep `fetch(` 全量清点，统一封装处补头。
- **兼容性**：不带头的请求与现状逐字节一致；`network_switch` 端点保留；旧脚本零影响。

## 验收清单

- [ ] 两用户（两头）同 server 分别路由到不同网络（E2E 测试通过）
- [ ] 不带头的请求行为与现状一致（全部既有测试保持绿，无回归）
- [ ] tools/list 57 工具均含可选 `network`
- [ ] MCP 工具带 network 调用，REST 侧收到正确头
- [ ] 前端偏好切换 + 刷新保持 + 请求带头（浏览器实测）
- [ ] API.md / README / MCP 页面文档同步
- [ ] `go test ./...` 全绿
