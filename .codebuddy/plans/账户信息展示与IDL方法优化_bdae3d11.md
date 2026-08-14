---
name: 账户信息展示与IDL方法优化
overview: 前端账户管理弹窗增加私钥/公钥的展开查看（默认遮蔽、可点击眼睛图标显示、支持复制），并利用 IDL 完整元数据（signer_lookups/types/constants/errors）对 6 个模块的所有方法做针对性优化，让 IDL 方法调用更智能好用。
design:
  architecture:
    framework: html
  styleKeywords:
    - 深色主题
    - 科技风
    - 可折叠详情
    - 等宽密钥展示
    - 微交互动效
  fontSystem:
    fontFamily: PingFang SC
    heading:
      size: 14px
      weight: 600
    subheading:
      size: 12px
      weight: 600
    body:
      size: 12px
      weight: 400
  colorSystem:
    primary:
      - "#7c5cff"
      - "#22d3ee"
    background:
      - "#0a0e1a"
      - "#1a1f2e"
    text:
      - "#e5e7eb"
      - "#94a3b8"
    functional:
      - "#22d3ee"
      - "#f87171"
      - "#fbbf24"
todos:
  - id: backend-idl-metadata
    content: 扩展 handler/idl_handler.go 的元数据结构与 buildAppMeta，新增返回 types、constants、errors、signerLookups 字段
    status: completed
  - id: account-key-display
    content: 在 static/js/app.js 的 renderAccountModal 中实现账户行展开详情，展示私钥/公钥并支持遮蔽切换与复制
    status: completed
  - id: account-key-style
    content: 在 static/css/style.css 新增账户详情、眼睛切换、复制按钮等样式
    status: completed
    dependencies:
      - account-key-display
  - id: idl-type-form
    content: 改造 buildIDLArgInput 与状态缓存，支持枚举下拉、struct 字段级子表单、类型提示
    status: completed
    dependencies:
      - backend-idl-metadata
  - id: idl-signer-constants
    content: 在支付配置与请求组装中应用 signerLookups 自动推导签名角色、constants 填充默认值
    status: completed
    dependencies:
      - backend-idl-metadata
  - id: idl-example-data
    content: 核对 6 个 idl.json，修正 IDL_EXAMPLE_ARGS 与 IDL_EXAMPLE_PAYMENT 映射表并补充 IDL 表单样式
    status: completed
---

## 产品概述

对 Milon 区块链调试控制台做两项前端可用性增强：(1) 账户管理弹窗中可视化展示已保存账户的私钥与公钥信息；(2) 针对 IDL 中全部 6 个 app（token/account/demo/identity/nft/staking）的每个方法进行针对性优化，提升调用便捷度。

## 核心功能

### 账户管理密钥展示

- 账户列表中每个账户行新增「展开/折叠」能力，展开后显示该账户的完整私钥与公钥，并各附一个「复制」按钮。
- 私钥默认遮蔽显示（如 `••••••••`），点击「眼睛」图标可在遮蔽与明文之间切换；公钥默认明文展示。
- 展开/折叠、眼睛切换、复制均为纯前端交互，数据来源于已保存在 localStorage 的账户信息，不涉及后端改动。

### IDL 方法针对性优化

- 后端 `/api/idl/metadata` 接口补充返回 `signerLookups`、`types`（struct 字段 + enum 变体）、`constants`、`errors` 四类元数据，供前端做智能表单渲染与提示，保持向后兼容。
- 前端 IDL 参数表单按类型智能渲染：枚举类型（enum）渲染为下拉选择框；结构体类型（struct）渲染为字段级子表单；标量保持输入框；vec/map/tuple 保持 JSON 文本域并附带类型提示。
- entry 方法根据 `signerLookups` 自动确定签名者角色与对应参数，替代手写的 `payerRole` 硬编码，减少配置错误。
- 参数默认值优先从 `constants`（如 `MIL_TOKEN_ADDRESS`）读取，替代硬编码示例地址。
- 核对并修正 6 个模块的 `IDL_EXAMPLE_ARGS` 与 `IDL_EXAMPLE_PAYMENT` 映射表，使示例参数与实际 IDL 定义一致。
- 响应错误时，可依据后端 `errors` 元数据提示中文错误说明。

## 技术栈

- 后端：Go + Gin（现有），复用 `github.com/milon-labs/milon-go-sdk/provider` 中的 `IDL`/`Instruction`/`IDLType`/`Constant`/`ErrorDef` 等结构。
- 前端：原生 JavaScript（无框架），DOM 通过现有 `el()` helper 构建，样式写在 `static/css/style.css`。
- 数据持久化：账户信息存 localStorage（现有），IDL 元数据由后端动态下发。

## 实现方案

### 后端 IDL 元数据扩展（handler/idl_handler.go）

扩展 `idlAppMeta` 与 `idlInstructionMeta` 结构体，新增字段并从 `pd.IDL` 读取：

- `idlAppMeta` 新增 `Types []idlTypeMeta`、`Constants []idlConstantMeta`、`Errors []idlErrorMeta`。
- `idlInstructionMeta` 新增 `SignerLookups map[string]idlSignerLookupMeta`。
- 新增 `idlTypeMeta`（Name/Kind/Fields/Variants）、`idlStructFieldMeta`（Name/Type）、`idlEnumVariantMeta`（Name/Kind/Fields）、`idlConstantMeta`（Name/Type/Value）、`idlErrorMeta`（Code/Name/Message）、`idlSignerLookupMeta`（Path.Arg/Path.Type/Res）。
- `buildAppMeta` 中同步构建 `Types`/`Constants`/`Errors`，指令循环中构建 `SignerLookups`。
- 保持原有字段不变，新增字段用 `omitempty` 控制空值，确保向后兼容。这是纯增量改动，不改变现有前端消费的字段。

### 前端账户密钥展示（static/js/app.js + style.css）

- `renderAccountModal()` 中账户行结构改造：保留 `label`/`address` 主行，新增一个「展开」切换按钮与一个可折叠的详情子面板。
- 详情面板含两行：私钥（默认遮蔽，眼睛图标切换明文，复制按钮）、公钥（明文，复制按钮）。
- 使用 `state` 或行级闭包变量记录每行的展开/遮蔽状态；切换时仅重渲染当前行详情，避免整表重渲染（性能友好）。
- 复用现有 `copyToClipboard`、`showToast` 工具函数。
- CSS 新增 `.account-detail`、`.account-key-row`、`.account-key-label`、`.account-key-value`、`.account-eye-btn`、`.account-copy-btn` 等样式，与现有深色主题一致。

### 前端 IDL 表单交互优化（static/js/app.js）

- `loadIDLMetadata` 后，将每个 app 的 `types` 建立为 `Map<name, typeDef>` 缓存（存于 `state.idlTypes`），供 `buildIDLArgInput` 查询。
- `buildIDLArgInput` 改造：
- 若 `arg.type` 命中 enum 定义（`Kind=="enum"`），渲染 `select`，options 为各 variant 名。
- 若命中 struct 定义（`Kind=="struct"`），渲染字段级子表单（每个字段按自身类型递归渲染，支持嵌套）。
- 标量类型保持现有 input；`vec<>`/`map<>`/`tuple<>`/`option<>` 保持 JSON textarea，但用 `placeholder` 提示元素类型。
- `buildIDLPaymentSection`/`renderIDLPaymentFields`：优先用 `ix.signerLookups` 推导 payer 角色与对应参数名，替代 `IDL_EXAMPLE_PAYMENT` 中手写的 `payerRole`（保留 `IDL_EXAMPLE_PAYMENT` 作为兜底/特殊场景覆盖）。
- `buildIDLRequest`：地址/signer 类参数默认值优先取 `constants`（如 token 地址）与当前活跃账户地址，再回退示例值。
- 核对 `gosdk-develop/provider/IDL/*.idl.json`（6 个模块），修正 `IDL_EXAMPLE_ARGS` 与 `IDL_EXAMPLE_PAYMENT` 中与实际 IDL 不一致的字段（如 signer 角色、`_tag` 字段、枚举参数值等）。

### 性能与可靠性

- 前端渲染均为一次性 DOM 构建 + 局部更新，避免反复全量重绘；账户详情切换仅操作目标行。
- IDL `types` 缓存为内存 Map，避免每次渲染重复线性查找（O(1) 查询）。
- 后端元数据一次性构建，无额外 I/O 热路径。

## 目录结构

```
milon_console/
├── handler/
│   └── idl_handler.go        # [MODIFY] 扩展元数据结构与 buildAppMeta，新增 types/constants/errors/signerLookups
├── static/
│   ├── js/
│   │   └── app.js            # [MODIFY] 账户展开详情+密钥展示；IDL 类型缓存、枚举下拉、struct 子表单、signerLookups/constants 应用、示例数据修正
│   └── css/
│       └── style.css         # [MODIFY] 新增账户详情/眼睛切换/复制按钮、enum select、struct 子表单样式
```

（`main.go`、`gosdk-develop`、`API.md` 无需改动；API.md 如需要可后续单独补充说明，非本次必需。）

## 关键数据结构（后端新增）

```
type idlTypeMeta struct {
    Name     string               `json:"name"`
    Kind     string               `json:"kind"` // struct | enum | builtin | tuple | unit
    Fields   []idlStructFieldMeta `json:"fields,omitempty"`
    Variants []idlEnumVariantMeta `json:"variants,omitempty"`
}
type idlStructFieldMeta struct {
    Name string `json:"name"`
    Type string `json:"type"`
}
type idlEnumVariantMeta struct {
    Name   string               `json:"name"`
    Kind   string               `json:"kind"`
    Fields []idlStructFieldMeta `json:"fields,omitempty"`
}
type idlSignerLookupMeta struct {
    Arg  string `json:"arg"`
    Type string `json:"type"`
    Res  uint8  `json:"res"`
}
```

本次为现有控制台的局部 UI 增强，沿用当前深色科技风主题。账户管理弹窗内新增可展开的账户详情区域，密钥信息用等宽字体呈现，私钥默认遮蔽并用「眼睛」图标切换明文，配复制按钮；IDL 表单新增枚举下拉、结构体字段级分组输入，整体保持与现有卡片、渐变、圆角、霓虹强调色一致，避免引入新的设计语言。