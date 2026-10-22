# pysdk 对照 gosdk 增量同步 + 开源发布设计

- 日期：2026-10-22
- 状态：已获用户批准（两阶段：主仓移植 → 独立开源仓）
- 源：`gosdk-develop/`，基线 `17ae931`（2026-10-09）→ `HEAD`（49 文件 / +16662 −5716）
- 目标：`pysdk-develop/src/milon_sdk/`（发布名 `milon-python-sdk`），最终开源到 GitHub
- 移植规范：沿用 `pysdk-develop/CONVENTIONS.md`，本文只列增量与验收

## 1. 变更清单（`git diff 17ae931..HEAD -- gosdk-develop/` 逐文件分类）

### A. IDL / gen —— 机械同步，无需手写代码

| Go 侧 | 变化 | Python 侧动作 |
|---|---|---|
| `provider/IDL/validator.idl.json` | **新增**（app_id 7，validator 应用） | 直接拷入 |
| `provider/IDL/index.json` | +validator(7)、**−lucky_box(9)**、**demo 255 → 254**、错误码 +64..68（ContextNotBound / AddressNotSignatured / DirectPubkeySignerContextNotBound / DirectPubkeySignerMismatch / AppCallerRequired） | 直接拷入（`gen.default_idls()` 按 index 加载，lucky_box 自动退出注册面） |
| `provider/IDL/{account,demo,dex,identity,keyless,sftoken,social,staking,system,token}.idl.json` | 全量重生成（staking −23/+28 指令改 Pool 模型、token +Vesting、account 多签地址注册、system +ValidatorId 内建类型） | 直接拷入 |
| `gen/idl_gen.go` | 14626 行重生成，**全部 typeTag 变化** | 无需移植：Python gen 运行时从同一份 IDL JSON 构建，只更新对账基准 |
| `tools/idlgen/main.go` | 生成器自身小改 | Go-only，不移植 |

基线核验：17ae931 时 pysdk 与 gosdk 的 12 个 IDL 文件 md5 **逐字节相同**，故"直接拷贝"是安全动作。

### B. ApiFailure 错误协议 —— 需移植

| Go 侧 | 变化 | Python 侧动作 |
|---|---|---|
| `api/apiFailure.go` | **新增 92 行**：`ApiFailure{Message, Code Option<u16>, Data Option<Vec<u8>>}` + postcard 编解码 + `String()`（code 十进制、data 十六进制） | 新建 `api/api_failure.py` |
| `lib/rpcResponse.go` | **删除** `RpcResponseError`，`RpcResponse.Error` 改用 `*api.ApiFailure` | `lib/rpc_response.py` 删本地类，改引 `api.ApiFailure` |
| `provider/provider.go` | `decodeTxFailurePayload` → 包级 `decodeApiFailure`：布局由 `code+message+data` 改为 **`message+Option<code>+Option<data>`**；新增 `decodeViewString` / `decodeViewOptionFlag` | `provider/provider.py::_decode_tx_failure_payload` 同步重写 |
| `api/test/{base,simulateReceipt,txHistory}_test.go`、`lib/test/rpcResponse_test.go` | 改名/字段波及 | 同步更新 `tests/api/*`、`tests/lib/*` |

注：`api.TxFailurePayload`（simulate receipt 内的失败载荷）**未变**，与 `ApiFailure` 是两个类型，不可合并。

### C. Batch 方法类型 —— 需移植

| Go 侧 | 变化 | Python 侧动作 |
|---|---|---|
| `lib/rpcRequest.go` | 新增 `BatchSubmitTx=6`、`BatchSimulateTx=11`、`BatchView=16`（6/11 仅占位，暂无 client 方法） | `MethodType` + 模块级别名同步 |
| `rpcClientV1.go` | **`View()` 请求方法号 15 → 16**（响应体布局不变） | `rpc_client_v1.py::view` 改用 `METHOD_TYPE_BATCH_VIEW` |
| `blockStream.go`（+77）、`framedHistory.go`、`printFramedHistory.go` | 批交易解码、新增 `ErrTxDecode` | **不移植**：pysdk 本就未移植 blockStream（grpc 区块流），列入 README「未支持」清单 |

### D. crypto 地址保留区 + keyless —— 需移植

| Go 侧 | 变化 | Python 侧动作 |
|---|---|---|
| `crypto/hash_domain.go` | 新增 4 个域：`milon.address.pk.retry.v1`、`milon.address.keyless.v1`、`milon.seed-address.v3`、`milon.seed-address.v2` | 同步常量 + `*_DOMAIN_BYTES` |
| `crypto/address.go` | `ContractAppID=0xFF`；`NewAddressFromPublicKey` 改**重试循环**（候选命中保留区则用 retry 域 + 域 + pk + BE u64 计数器从 1 重哈希，counter 耗尽 panic）；新增 `NewAddressFromAppSeed`（`[appID｜seed u64 BE｜hash[0..11]]`）、`NewAddressFromAppSeed32`、`IsAppReserved`、`AppSeed`；`NewAddressFromRelaxed` 解析顺序改为 hex 形状先 hex 后 base58、其余先 base58 后 hex；`isHexString` | `crypto/address.py` 同结构重写 |
| `crypto/publickey.go` | 变体新增 `PublicKeyTypeKeylessDerived=4`（32 字节，只由 `NewKeylessDerivedPublicKey` 显式构造，文本/字节解析永不产出；postcard 期望长度 32）；`IsKeylessDerived()` | `crypto/publickey.py` 同步 |
| `crypto/test/address_test.go`（+207）、`publickey_test.go`（+95） | 保留区/retry/keyless 全量新测试 | 逐例译为 `tests/crypto/test_address.py`、`test_publickey.py` |

### E. 注释与文档-only，无需移植

`network.go`（GrpcAddr 注释翻译）、`resolveResourcePaths.go`（注释翻译）、`README.md`、`.gitignore`、`example/**`（Go 示例，含新增 identity_demo 约 1200 行；pysdk 示例面维持 `e2e_token_demo` + `sig_modes_demo`）。

## 2. 影响与风险

- **breaking**：地址派生规则变化 + typeTag 全变。`0.1.0` 从未发布，无兼容负担；版本直接跳 `0.2.0`。
- **devNet 依赖**：`View()` 改走 method 16，需 devNet RPC 支持；冒烟若遇链侧问题按 `docs/PLAYBOOK_milon_devnet_操作经验_20261021.md` 排障，不阻塞移植正确性判定。
- **既有断言漂移**：typeTag 与 demo app_id(255→254) 相关的硬编码断言需随新 IDL 更新，更新必须逐项核对而非批量改数值。
- **遗留项如实标注**：`_fndsa`（19 skip）与 blockStream 不在开源前补齐，README 写清「未支持」。

## 3. 两阶段执行

阶段一（主仓 `pysdk-develop`，TDD，每步独立 commit）：
1. 本清单入档（本文件）
2. IDL 全量同步 + `tools/check_gen_parity.py` 对账基准更新 → PASS
3. 译 Go 新测试为红（apiFailure / 保留区地址 / keyless 域）
4. 移植 `api/api_failure.py` + lib Batch* + rpcClientV1 BatchView → 绿
5. 移植 crypto 保留区 retry 域 + keyless 域 → 绿
6. provider `decodeApiFailure` + network/既有硬编码断言更新 → 172+ 全绿
7. devNet 冒烟：`e2e_token_demo` + `sig_modes_demo` 真实回归

阶段二（独立仓 `xiaoqi1998/milon-python-sdk`，public + MIT）：
从移植后的 pysdk-develop 拷贝、**全新 git 历史**（主仓 commit message 含内部服务器细节，不带入）、
LICENSE / README.md（英文主）+ README.zh-CN.md / docs/PORTING.md（由 CONVENTIONS.md 改造）/
GitHub Actions pytest 矩阵；examples 去本机绝对路径；版本 0.2.0；敏感信息终扫；
`tools/check_gen_parity.py` 保留。推公开仓前需用户确认。
