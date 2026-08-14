---
name: 账户管理支持JSON粘贴导入
overview: 在账户管理弹窗中新增「粘贴导入」功能：粘贴 JSON 数组文本（每项含 address + privateKey），解析后批量导入账户，label 自动生成、publicKey 留空，同地址覆盖旧账户，并展示导入结果统计。
todos:
  - id: import-functions
    content: 在 static/js/app.js 新增 importAccounts 及 label 自动生成辅助函数
    status: completed
  - id: import-ui
    content: 在 renderAccountModal 中新增「批量导入」粘贴区与导入按钮，接入 importAccounts
    status: completed
    dependencies:
      - import-functions
  - id: import-style
    content: 在 static/css/style.css 补充批量导入区样式
    status: completed
    dependencies:
      - import-ui
---

## 产品概述

为 Milon 调试控制台的账户管理弹窗新增「粘贴导入」能力：用户可粘贴一个 JSON 数组文本，批量导入多个账户到本地账户库。

## 核心功能

- 在账户管理弹窗中新增「粘贴导入」入口，提供一个 JSON 文本输入区与「导入」按钮。
- 导入数据格式：JSON 数组，每项仅需 `address` 与 `privateKey` 两个字段。
- `label` 自动生成（如「导入账户1」「导入账户2」，序号自动递增避免与现有账户重名）。
- `publicKey` 留空（`''`）。
- 冲突处理：若导入项的 `address` 与已存在账户地址相同，则用导入数据覆盖该旧账户（保留原 id 与 createdAt，更新 label/address/privateKey，publicKey 清空）。
- 导入结果提示：明确反馈「成功导入 X 个、覆盖 Y 个、跳过 Z 个非法项」。
- 容错：JSON 解析失败、非数组、数组项字段缺失或 address/privateKey 为空时，给出明确提示并跳过非法项。

## 技术栈

- 前端：原生 JavaScript（无框架），DOM 通过现有 `el()` helper 构建。
- 样式：`static/css/style.css`，沿用深色主题，复用现有 CSS 变量（`--bg-card`/`--border-color`/`--accent-cyan` 等）与 `.account-form`/`.param-row`/`.param-input` 样式。
- 数据持久化：localStorage（key `milon_accounts`），复用现有 `saveAccounts()`。

## 实现方案

### 实现思路

纯前端改动，不涉及后端与 API。在 `renderAccountModal` 的「添加账户」表单区之前新增一个「批量导入」区域（含 textarea + 导入按钮），并新增 `importAccounts(jsonText)` 函数完成解析、校验、覆盖/新增、持久化、重渲染与结果提示。

### 关键设计决策

1. **覆盖语义保留 id 与 createdAt**：同地址覆盖时保留原账户的 `id`（避免破坏 `state.currentAccountId` 引用）与 `createdAt`，仅更新 `label`/`address`/`privateKey`/`publicKey`，保证当前活跃账户引用稳定。
2. **label 自动递增生成**：计算现有账户中 `导入账户{N}` 的最大序号，从 `max+1` 起递增生成，避免重名。
3. **批量导入函数返回统计结果**：`importAccounts` 返回 `{added, updated, skipped}`，由调用方 `showToast` 汇总展示，职责清晰、可测试。
4. **校验严格但跳过而非中断**：单项非法（缺 address/privateKey、类型非字符串、去空白后为空）时计数为 skipped 并继续，不因单条坏数据中断整批导入。

### 实现要点

#### 新增函数 `importAccounts(jsonText)`

```
function importAccounts(jsonText) { ... 返回 {added, updated, skipped} }
```

逻辑步骤：

1. `JSON.parse` 解析文本；失败则 `showToast` 报错并返回 null。
2. 校验结果为数组；非数组报错返回 null。
3. 遍历每项：

- `address = String(item.address || '').trim()`，`privateKey = String(item.privateKey || '').replace(/\s/g,'')`。
- 二者任一为空则 `skipped++` 继续。
- 查找 `state.accounts` 中 `address` 相同的账户：
    - 存在：更新该账户 `label`（自动生成）、`privateKey`、`address`、`publicKey=''`，保留 `id`/`createdAt`，`updated++`。
    - 不存在：`state.accounts.push({id: genAccountId(), label: 自动生成, address, privateKey, publicKey:'', createdAt: Date.now()})`，`added++`。

4. 返回统计结果。

#### 新增辅助函数 `nextImportLabel()`（可选内联）

计算 `state.accounts` 中 label 形如 `导入账户(\d+)` 的最大序号，返回下一个 label。可在循环内用一个局部 `labelSeq` 计数器简化：先算初始 `base = maxSeq+1`，每次生成后 `labelSeq++`。

#### 修改 `renderAccountModal`

在「添加账户」表单（`var form = el('div', {class:'account-form'})`）之前插入「批量导入」区域：

- 标题「批量导入」。
- `textarea`（class 复用 `body-editor` 或 `param-input`，placeholder 示例 JSON 数组）。
- 「导入」按钮，点击后读取 textarea 值 → 调用 `importAccounts` → 成功后清空 textarea、`saveAccounts()`（在 importAccounts 内完成）、`renderAccountModal()`、`showToast` 汇总。

#### CSS 补充

新增少量样式（如 `.account-import-section`、`.account-import-actions`），或直接复用 `.account-form`/`.account-form-title`/`.account-form-actions`/`.param-row` 等既有样式，最小化新增代码。

### 性能与可靠性

- 导入规模为本地账户（通常几十到几百条），线性遍历即可，无性能瓶颈。
- 覆盖查找用 `state.accounts.find`（O(n)），对小规模账户列表足够；若需优化可建立 address→index Map，但非必要。
- 全部逻辑为同步、无 I/O 热路径；仅导入完成后调用一次 `saveAccounts()` 持久化。

## 目录结构

```
milon_console/
├── static/
│   ├── js/
│   │   └── app.js      # [MODIFY] renderAccountModal 新增批量导入区；新增 importAccounts 及 label 生成辅助函数
│   └── css/
│       └── style.css   # [MODIFY] 新增批量导入区样式（可复用既有类，按需少量新增）
```

（后端、`main.go`、`API.md`、`handler` 均无需改动。）