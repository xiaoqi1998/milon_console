# Milon Go SDK → Python 迁移可行性评估与实施计划

> 评估对象：`D:\pprojiect\milon-api-server\gosdk-develop`（Milon 区块链 Go SDK）
> 评估日期：2026-08-18
> 结论先读：**技术上可行，但有一个关键门控风险（FnDSA-512 后量子签名的跨语言字节级互操作），必须在动工前用可行性 spike 验证**。

---

## 1. 执行摘要

现有 SDK 是一个约 **3.98 万行 Go 代码** 的区块链客户端，涵盖合约 IDL 编解码、交易构建与签名（4 种密码学算法）、RPC 通信（双线协议：JSON 信封 + Postcard 二进制）、事件/资源解析。架构清晰、模块边界明确，非常适合迁移。

**可行性结论：可行。** 主要工作量集中在三块必须字节级精确复刻的部分：
1. 自定义 **Postcard 二进制协议**（变长整数编码 + type_tag 动态解析）；
2. **4 种签名算法**（Ed25519 / Secp256k1 / BLS12-381 / FnDSA-512）及其地址派生哈希；
3. **IDL 编解码 + 动态 `gen` 绑定接口**。

**最关键的兼容性风险是 FnDSA-512**：它是基于 Falcon 的 NIST 后量子草案（FIPS 206 尚未定稿），Go 侧 `go-fn-dsa` 明确声明“不保证向后兼容、密钥与签名可能被后续版本拒绝”。Python 侧虽有候选库 `tectonic-bedrock-python`（`bedrock.FalconScheme.dsa_512()`），但其参数化（原始消息 / 空域 / `raw` 预哈希）能否与 Go `v0.2.0` 逐字节对齐，**必须通过实证 cross-check 确认**，否则链上验证会失败。

**工作量估算**：1 名资深工程师约 **19 人周（~4.5 个月）**；2 人并行可压缩至 **10–12 周**。

---

## 2. 现有 SDK 分析

### 2.1 编程语言与运行时
- 单一 Go module：`github.com/milon-labs/milon-go-sdk`，Go 1.25.9。
- 总 `.go` 代码 **39,787 行**，其中 `gen/idl_gen.go` 约 20,359 行为 **`tools/idlgen` 机器生成**（由 IDL JSON 驱动），手写代码约 **19,428 行**。
- 无操作系统相关代码；**唯一平台耦合是 BLS 库 `supranational/blst` 使用 CGO**（C/ASM），因此 Go SDK 构建需 C 工具链。Python 侧应优先选用同样基于 blst 的 Python 绑定以保证字节一致。

### 2.2 架构分层
```
client.go         Client 门面 + RequestOption/WaitOption 函数式选项 + 方法委托
network.go        Network 配置（LocalNet / DevNet 预设，含 RpcUrl/ChainId）
rpcClientV1.go    RPC 实现：JSON 信封 / Postcard 二进制双线、IDL 加载、13 个 RPC 方法、WaitForTransaction 轮询
api/              RPC 响应结构体 + 各自的 Postcard Marshal/Unmarshal
crypto/          4 种签名算法、地址派生（BLAKE3）、哈希域常量
lib/             交易构建与签名（TransactionBuilder、AccountSignature、4 种付款模式、AuthBit 64 位位图、校验）
postcard/        自定义二进制协议（变长整数、option/seq、TypeResolver 注入）
provider/        IDL 编解码（Provider/registry/idlTypeResolver）、事件/资源/视图解码
gen/             IDL 生成的类型安全绑定（gen.Token.BalanceOf.Args(...).Encode()），随 NewClient 绑定
helper/          工具函数（CheckTxSuccess、GetAccount 展示等）
tools/           http.go（连接池 + 5xx 重试）、idlgen（IDL→Go 代码生成器）
types/           Bitmap64 64 位位图
```

### 2.3 核心功能模块
| 模块 | 职责 | 关键文件 |
|------|------|---------|
| 客户端门面 | 统一封装所有 RPC，类型安全选项 | `client.go` |
| 网络配置 | 链预设 / 自定义网络 | `network.go` |
| RPC 层 | 13 个 RPC 方法、双线协议、轮询等待 | `rpcClientV1.go` |
| 密码学 | 4 算法签名/验签、地址派生 | `crypto/*.go` |
| 交易 | 构建、4 种付款模式、AuthBit、ValidateWire | `lib/*.go` |
| 序列化 | 自定义 Postcard 二进制协议 | `postcard/*.go` |
| IDL | 指令/视图/事件编解码、类型解析 | `provider/*.go` |
| 生成绑定 | 类型安全的合约调用入口 | `gen/idl_gen.go` |
| 协议类型 | TxHash/RsHash/BlobHash 定长类型、各响应体 | `api/*.go` |

### 2.4 对外接口（需保证一致）
**公共 Client API**（`milon.NewClient(...)` 返回 `*Client`）：
- 账户：`ClaimFaucet / CreateAccount / BalanceOf / ListAccountSigners / AccountSignerBit`
- 交易：`SubmitTx / SubmitTxWithSponsorIxes / SimulateTx / WaitForTransaction`
- 链：`GetChainHead / GetBlockByHeight / GetTxByHash / GetTxHistoryProof`
- 资源：`GetResource / GetResourcePathByHash / BatchGetResourcePathByHash / GetAccessValue`
- 合约：`View`
- 元数据：`GetAllPd / GetProviderManager`
- 选项：`WithContext / WithRequestID / WithWaitContext / WithWaitPollPeriod / WithWaitPollTimeout / WithClientPollPeriod / WithClientPollTimeout`

**合约调用入口（IDL 驱动）**：
```go
wire, _ := gen.Token.BalanceOf.Args(token, account).Encode()   // 编码
balance, _ := gen.Token.BalanceOf.DecodeView(viewResult.HTTPResponseBody)  // 解码
```
若要“接口一致”，Python 侧必须提供等价调用形态（推荐**运行时动态绑定**，见 §6.3）。

**RPC 方法（MethodType 1–13）**：ChainHead / SubmitTx / SimulateTx / View / GetResource / GetBlockByHeight / GetTxByHash / GetAccount / EventsByTxHash / GetResourcePathByHash / GetAccessValue / BatchGetResourcePathByHash（+ GetTxHistoryProof）。注意：**双线协议**——部分方法体以 JSON 十进制整数数组发送（`encodeJsonRPCRequest`），Submit/Simulate/View-body 用 Postcard 二进制；`GetResourcePathByHash` 直接以 `rsHash[:]` 字节为体。

---

## 3. 依赖项与平台相关代码

### 3.1 第三方库（go.mod 直接依赖）
| 库 | 用途 | Python 对应 | 风险 |
|----|------|------------|------|
| `lukechampine.com/blake3` | BLAKE3 哈希（地址/交易/授权哈希） | `blake3` (PyPI) | 低，官方 Rust 绑定 |
| `golang.org/x/crypto` / `hdevalence/ed25519consensus` | Ed25519 签名 / 批量验签 | `cryptography` | 低 |
| `decred/dcrd/dcrec/secp256k1/v4` + `ethereum/go-ethereum/crypto` | Secp256k1 ECDSA（以太坊可恢复签名） | `coincurve` / `eth_keys` | 中（V=27/28 恢复字节） |
| `supranational/blst`（**CGO**） | BLS12-381 签名（G2，96B） | `blst`(Python 绑定) 或 `py_ecc` | 中（密码套件匹配） |
| `cloudflare/circl/sign/bls` | BLS 公钥解析（G1） | 同上 | 中 |
| `pornin/go-fn-dsa` | **FnDSA-512 后量子签名（666B/1281B）** | `tectonic-bedrock-python` | **高（草案、跨语言对齐未证）** |
| `btcsuite/btcutil/base58` | Base58 编解码（地址/公钥/签名序列化） | `base58` (PyPI) | 低 |
| `stretchr/testify` | 测试 | `pytest` | 低 |

### 3.2 平台相关代码
- **CGO**：仅 `blst` 使用（C/ASM）。迁移到 Python 时，若选 `blst` 的 Python wheel（预编译），则无需本地 C 工具链；若选 `py_ecc`（纯 Python/C 扩展），需确认其 BLS 与 Go `blst` 同密码套件。
- 无其他 OS/架构分支；并发仅用于 `WaitForTransaction` 轮询（`time.Ticker`）与 `sync.Pool` 缓冲复用，可直译为 Python 线程/asyncio。
- 并发原语：`sync.RWMutex` 保护全局 `ChainId`、`sync.Pool` 复用 JSON 缓冲——Python 侧可用模块级全局 + `threading.Lock`，缓冲复用非必需（GC 足够）。

---

## 4. 需重写的逻辑 / 类型映射 / 兼容性问题

### 4.1 密码学（最关键）
**地址派生**：`Address = BLAKE3(MILON_ROOT || "milon.address.pk.v1" || pk.Bytes)[:20]`，其中 `MILON_ROOT = "Milon-blake3"`。域字节与截断必须精确复刻。

**交易/授权哈希**（端序极易出错）：
- `TxHash = BLAKE3(MILON_ROOT || "milon.tx.v1" || chainId(BE u64) || stamp(BE u64) || [payer] || ixHashes)`
- `IxHash = BLAKE3(MILON_ROOT || "milon.ix.v1" || chainId(BE u64) || wire)`
- `AuthMessage = BLAKE3(MILON_ROOT || "milon.tx.auth.v1" || chainId(BE u64) || owner || authBit(LE u64) || txHash || ixHashes)`
- 注意：**哈希输入中的整数用 BigEndian，而线上 Postcard 的 u64 用变长（LE 7-bit 组）**——两者不可混淆。

**4 种签名算法差异与风险**：
| 算法 | 线上长度 | Go 实现 | Python 候选 | 兼容性风险 |
|------|---------|---------|------------|-----------|
| Ed25519 | 64B | `x/crypto/ed25519`（标准 RFC8032） | `cryptography.Ed25519` | 低（标准） |
| Secp256k1 | 65B | go-ethereum `crypto.Sign`（V=27/28） | `coincurve.sign_recoverable` + V+27 | 中（恢复字节 V 处理） |
| BLS12-381 | 96B | `blst.P2Affine.Sign(sk,msg,nil)` G2Basic | `blst`(Python) / `py_ecc.G2Basic` | 中（DST/密码套件须匹配） |

> **2026-08-19 更新**：BLS 字节兼容已实证解决——Go SDK 传 DST=nil，blst 实际使用**空 DST**（非 NUL_/POP_，已用 `blst.HashToG2` 对比实证）。Python 侧用 `py_ecc.bls.hash_to_curve.hash_to_G2(msg, b"", sha256)` 复刻，签名 96B 与 Go 逐字节一致且可互验（见 `python/milon_sdk/crypto/_bls.py` 与 `tests/test_crypto.py::test_bls12381_byte_parity_with_go`）。
| **FnDSA-512** | 666B | `go-fn-dsa v0.2.0`（原始消息、空域、raw） | `tectonic-bedrock-python.dsa_512()` | **高（草案、跨实现未验证）** |

**Secp256k1 细节**：`SignFor` 中若 `msg` 为 32 字节则直接作为哈希，否则先 `BLAKE3(msg)`；验签 `VerifySignature(pub, hash, sig[:64])`。Python 必须保留“32B 直通 / 否则 BLAKE3”的分支。

**FnDSA-512 特别风险**：
- Go `go-fn-dsa` 自述：**非最终 FN-DSA 标准，不保证向后兼容**。
- 调用参数：`fndsa.Sign(rand.Reader, sk, []byte{}, 0, msg)` → `ctx=空`、`id=0(HASH_ID_RAW)` 即**原始消息**签名。
- Go 侧 `FnDsa512Public()` 未实现（TODO：无法从签名密钥推导验证密钥），因此公钥需预生成传入。Python 侧对称。
- 必须验证 `bedrock-python` 的 `dsa_512()` 使用相同的 Falcon 变体、原始消息、空域，才能与 Go 及链上互验。

### 4.2 Postcard 二进制协议（必须字节级精确）
自定义变长整数：`SerializeU16/U32/U64` 均走 `serializeVarUint64`（**LE 7-bit 组**，continuation bit 在高位）；`U128` 走 `serializeVarUintBig`。固定字节、`option(bool+value)`、`seq(len varint + 元素)` 均有特定布局。`TypeResolver` 在反序列化时按 `type_tag` 动态分发到已加载 IDL 解码 `DecodeResource/DecodeEvent`。

Python 侧需用 `bytes` + 游标偏移复刻整个 Serializer/Deserializer。**建议优先于任何业务代码实现，并用 Go `postcard_test.go` 的黄金向量守护。**

### 4.3 IDL 编解码与 `gen` 绑定
- `encodeInstruction`：`app_id(u8) + discriminator(u16 LE) + args`（按 IDL 顺序、按类型序列化）。
- 支持 `vec<T> / option<T> / map<K,V> / tuple<...> / struct / enum / 基础类型`，且 `enum` 变体前有 `variant_index(varint)`。
- 视图解码 `DecodeViewDatas`：外裹 `Vec<Result<T>>`，变体 0=Ok(长度前缀+值)、1=Err(`TxFailurePayload{code u16, msg, data}`)；失败需转为异常。
- `gen` 绑定：`gen.Token.BalanceOf.Args(...).Encode()` 与 `DecodeView(body)` 由 IDL 生成。Python 侧**推荐运行时动态绑定**（加载 IDL JSON → 用 `type()`/`SimpleNamespace` 动态构造同名可调用对象），免去移植代码生成器，且天然与 IDL 同步。

### 4.4 Go → Python 类型映射差异
| Go | Python | 注意 |
|----|--------|------|
| `[N]byte` 定长数组 | `bytes`（运行时长度校验） | 失去编译期保证，需显式 assert len |
| `uint64/int64` | `int`（任意精度） | u128 用 `int` 并校验范围；负数禁止 |
| `map[any]any` | `dict` | key 可为任意可哈希类型 |
| `enum` 变体 | `dict{"variant", "fields"}` 或 dataclass | 保持可预测结构 |
| `Bitmap64 uint64` | `int` + 辅助类 | 提供 Test/Set/IterSetBits |
| 接口 `SecretKeyer`/`AccountSignatureMode`/`Marshaler` | 鸭子类型 / `typing.Protocol` / ABC | |
| 泛型 `SerializeSeq[T]` | 泛型函数 / 类型注解 | |
| `(v, err)` 多返回值 | **异常**（推荐） | 保证接口等价前提下选异常 |
| `sync.Pool`/`RWMutex` | 不需要 / `threading.Lock` | 性能非热路径，可简化 |

---

## 5. 工作量、风险与资源评估

### 5.1 代码量基准
| 部分 | Go LOC（约） | Python 估算 LOC | 说明 |
|------|-------------|----------------|------|
| postcard | 766 | ~900 | 字节级复刻，机械但严格 |
| crypto | 2,883 | ~2,200 | 含 4 算法适配 + 地址/哈希 |
| api 类型 + 编解码 | 3,779 | ~2,800 | 各响应体 Marshal/Unmarshal |
| provider/IDL codec | 3,316 | ~2,800 | 编解码 + registry + resolver |
| lib 交易构建/签名 | 3,976 | ~3,000 | 4 付款模式 + 多签 + 校验 |
| gen 绑定 | 20,359（生成） | ~0（运行时动态） | 改为运行时绑定，几乎零手写 |
| rpcClient + client | 397（client）+ rpc 体 | ~1,400 | 双线 RPC + 选项 + 轮询 |
| helper + 示例 + 打包 | 259+1,842 | ~1,200 | 含示例与 pytest |
| 测试 + 跨语言 parity | （testify） | ~3,000 | **最关键，门控** |
| **合计** | **~39.8k** | **~17–19k** | 生成代码转为运行时 |

### 5.2 工作量（1 名资深工程师，熟悉密码学）
| 阶段 | 内容 | 人周 |
|------|------|------|
| 0. 可行性 spike | 密码学 parity harness，验证 4 算法（尤其 FnDSA/BLS）与 Go/链上互验 | 1.5 |
| 1. 核心协议 | postcard + crypto + api 类型 | 4 |
| 2. IDL 体系 | provider + resolver + 动态 gen 绑定 | 4 |
| 3. 交易 | 构建/签名/校验/4 付款模式/多签 | 3 |
| 4. RPC 客户端 | 双线 RPC + Client 门面 + 选项 + WaitForTransaction | 2 |
| 5. 封装 | helper + 示例 + 打包（PyPI/内源） | 1.5 |
| 6. 加固 | 跨实现测试 + fuzz + 文档 + 切换 | 2.5 |
| **总计** | | **~18.5 周** |

2 人并行（密码学 spike 与协议层可部分重叠）可压至 **10–12 周**。

### 5.3 风险矩阵
| 风险 | 等级 | 影响 | 缓解 |
|------|------|------|------|
| FnDSA-512 跨语言字节对齐（草案） | **致命** | 签名链上拒收 | Phase 0 spike 实证；失败则降级（见 §7） |
| Postcard/crypto 字节级不一致 | 高 | 交易无法上链/解析错乱 | 黄金向量守护 + 双向 round-trip |
| BLS12-381 密码套件/DST 不匹配 | 中 | BLS 签名验签失败 | 锁定 blst G2Basic 同 DST，cross-check |
| Secp256k1 恢复字节 V 处理 | 中 | 可恢复签名格式不符 | 严格复刻 V=27/28 规则 |
| 接口漂移（gen 风格丢失） | 中 | 调用方需改写 | 运行时动态绑定，保持 `sdk.token.X.Args(...).Encode()` |
| 性能/并发（非热路径） | 低 | 吞吐下降 | SDK 非高频，Python 足够 |

### 5.4 所需资源
- **人力**：1–2 名 Python 工程师 + 0.2 名密码学顾问（评审 FnDSA/BLS）。
- **Python 依赖**：`blake3`、`base58`、`coincurve`（或 `eth_keys`）、`blst`（或 `py_ecc`）、`tectonic-bedrock-python`（FnDSA）、`httpx`（异步 HTTP，含重试）/`requests`、`pytest`。
- **环境**：Python ≥3.10、venv、CI（GitHub Actions + pytest）。
- **验证基座**：现有 Go SDK + DevNet RPC（`149.104.26.82:6280/milon/v1`）用于 parity 与链上集成测试。

---

## 6. 分阶段迁移计划

### 6.1 总体策略
**自底向上 + 双向 parity 守护**：先复刻最底层（Postcard、哈希、密码学原语），逐级向上，每层都用“Go 产出 → Python 消费 / Python 产出 → Go 验证”的黄金向量锁死字节级一致性，最后做 RPC 与门面。

### 6.2 阶段划分与优先级
| 阶段 | 优先级 | 目标 | 退出标准 |
|------|--------|------|---------|
| **P0 可行性 spike** | P0（门控） | 证明 4 算法（尤其 FnDSA-512、BLS）Python 签名能被 Go `Signature.Verify` 与 DevNet 接受 | 4 算法 cross-check 全绿；否则触发 §7 降级决策 |
| **P1 Postcard + crypto + api 类型** | P0 | 字节级序列化/反序列化、地址派生、哈希域 | Postcard 全原语 round-trip + Go 黄金向量 100% 通过 |
| **P2 IDL 体系 + 动态 gen 绑定** | P1 | 指令/视图/事件编解码、type_tag resolver、`sdk.token.X.Args(...).Encode()` | 全部内置 IDL 的 encode/decode 与 Go 一致 |
| **P3 交易构建/签名/校验** | P1 | 4 付款模式、多签、Simulate/Sign、ValidateWire、TxHash/AuthMessage | 与 Go 构造的 tx 字节、哈希、签名完全一致 |
| **P4 RPC 客户端 + Client 门面** | P2 | 双线 RPC、13 方法、选项、WaitForTransaction(轮询+5xx 重试) | 每个 RPC 方法返回结构与 Go 等价 |
| **P5 封装与示例** | P2 | helper、示例、打包 | 7 个示例均有 Python 等价，产生相同链上效果 |
| **P6 加固与切换** | P3 | 跨实现测试、fuzz、文档、发布 | 黄金测试集 100% 通过，可灰度替代 |

### 6.3 关键设计决策
1. **动态 `gen` 绑定**：不移植 `idlgen`，而是在 `NewClient` 时加载 IDL JSON，用元编程动态生成 `token.BalanceOf.Args(...).Encode() / DecodeView(...)` 形态。好处：零生成代码、与 IDL 自动同步、接口一致。
2. **先写 Postcard**：协议是地基，必须用 Go `postcard_test.go` 的向量逐条锁定。
3. **密码学适配层**：把 4 个算法封装为统一 `sign(msg)->bytes`、`verify(msg,sig,pk)->bool` 接口，便于替换与单测。
4. **端序纪律**：BLAKE3 输入里的整数用 BigEndian；线上 Postcard 整数用变长（LE 7-bit）。代码里加注释与单测防混。
5. **异常 vs 返回码**：采用 Python 异常（保留错误语义），但对外错误类型命名尽量对齐 Go（如 `TxFailurePayload`）。

### 6.4 测试策略
- **单元测试**：每个 Postcard 原语 round-trip；每个密码学算法 sign/verify；IDL 各类型 encode/decode。
- **跨实现 parity（最重要）**：
  - Go 随机构造交易/签名 → Python 反序列化并验证；
  - Python 签名 → Go `Signature.Verify` 验证 + DevNet 上链验证。
- **链上集成**：用 Python SDK 在 DevNet 跑 faucet/account/create/transfer/view，与 Go SDK 同输入结果比对。
- **黄金向量**：将已知输入/输出（哈希、序列化 tx、签名、解码视图/事件）快照入仓库，CI 强制比对。
- **属性测试**：fuzz 序列化↔反序列化 round-trip。
- **CI**：`pytest` + 对 DevNet 的 parity（可设为 nightly 门控，避免 flaky）。

### 6.5 验证标准（功能等价 + 接口一致）
1. 对每个公共 `Client` 方法：相同输入 → 相同**线上字节**与相同输出（对比 Go 黄金向量）。
2. Python 生成的签名可被 Go `Signature.Verify` 及链上验证通过。
3. 13 个 RPC 方法返回等价结构体（字段、类型、嵌套一致）。
4. 提供 IDL 驱动的 `sdk.token.BalanceOf.Args(...).Encode()` 等价入口。
5. 7 个示例均有 Python 版本且产生相同链上效果。
6. 黄金测试集（tx 哈希、签名、解码视图/事件）**100% parity**。

---

## 7. 关键决策建议（给决策者）

1. **动工前必须先做 P0 spike（1.5 周）**，且仅当 FnDSA-512 与 BLS 的 Python 签名能被 Go/链上接受时才启动全量迁移。这是唯一可能“叫停”的硬风险。
2. **若 FnDSA-512 跨语言对齐失败**，有两个务实出路：
   - (a) Python SDK 先支持 Ed25519/Secp256k1/BLS，**FnDSA-512 仅做验签、暂不做签名**（或等 `go-fn-dsa` 随 FIPS 206 定稿、Python 库同步）；
   - (b) 用 Go 侧导出的预生成 FnDSA 密钥对文件供 Python 加载（避免在线派生）。
3. **若 Python SDK 仅用于只读分析/工具链**（不频繁上链），FnDSA 签名路径优先级可大幅降低，P1–P4 仍可独立交付。
4. **优先选 `blst` 的 Python 绑定**而非 `py_ecc`，以最大化与 Go `supranational/blst` 的字节一致性。
5. 采用**运行时动态 `gen` 绑定**，不要移植 IDL 代码生成器——更少代码、自动与 IDL 同步、接口天然一致。

---

## 附录 A：需重点复刻的常量（摘自源码）
- 哈希域：`Milon-blake3`(ROOT)、`milon.ix.v1`、`milon.tx.v1`、`milon.tx.auth.v1`、`milon.address.pk.v1`
- AuthBit 布局：`bit0–61`=指令授权、`bit62`=保留、`bit63`=gas(payer)
- 定长类型：`Address=20B`、`TxHash=32B`、`RsHash=18B`、`BlobHash=32B`、`TxId=12B`
- 签名长度：Ed25519=64、Secp256k1=65、BLS=96、**FnDSA=666**
- 公钥长度：Secp256k1=33、Ed25519=32、BLS=48、FnDSA=897
- MIL 代币地址常量：`M11on1111111111111111111111`

## 附录 B：建议 Python 包结构
```
milon_sdk/
  client.py          # Client 门面 + 选项
  network.py         # Network 配置
  crypto/
    hashes.py        # BLAKE3 域哈希
    keys.py          # 4 算法适配 + 地址派生
  postcard/          # serializer/deserializer + resolver
  provider/          # IDL 编解码 + registry
  lib/               # 交易构建/签名/校验
  api/               # 响应类型 + 编解码
  rpc.py             # 双线 RPC + WaitForTransaction
  gen.py             # 运行时动态绑定
  examples/          # 7 个示例等价物
tests/               # parity + 黄金向量
```
