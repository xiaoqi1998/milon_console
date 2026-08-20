# Python SDK 与 Go SDK（gosdk）差距评审报告

- 评审日期：2026-08-19
- 对象：`python/milon_sdk`（Python SDK）↔ `gosdk-develop`（Go SDK，~30k 行，不含生成代码与测试）
- 方法：按包逐项对比公开 API 面（client/rpcClientV1/network/crypto/lib/api/provider/types/postcard/gen/helper/tools），
  并核对行为差异与字节级对齐（`parity/check_parity.py` 对 Go 黄金向量，20 项全过；82 个 pytest 全过）。
- 结论：**核心面（Client/RPC/交易/签名/Postcard/IDL 绑定）已对齐并真机联调通过**；
  存在 1 个模块级功能性 bug、2 个高风险跨语言字节对齐风险（BLS/FN-DSA-512）、若干次要 API 缺口。

## 修复记录（2026-08-19 同日）

| 项 | 状态 | 说明 |
|---|---|---|
| P0-1 helper 属性名 | ✅ 已修 | `rpc.py` 全部结果类补 snake_case 属性别名（`body_tx_history` 等，`@property`），helper 4 个函数恢复可用；新增 4 个单测 |
| P1-2 ListResourcePathInfo | ✅ 已修 | 补 `ListResourcePathInfo` + `unmarshal_list_resource_path_list_from_raw_list`（对齐 Go 公开类型；`GetResourcePathByHash` 本身与 Go 一致为裸字符串） |
| P1-4 BLS 字节对齐 | ✅ 已修（重大发现） | **Go SDK 调 `blst.Sign(sk,msg,nil)` 实际使用「空 DST」**（非评估文档假设的 NUL_；已用 blst.HashToG2 对比空/NUL/POP 实证）。`_bls.py` py_ecc 回退路径改用 `hash_to_G2(msg, b"", sha256)`，**签名 96B 与 Go blst 逐字节一致**、可互验；新增 `test_bls12381_byte_parity_with_go` |
| P1-5 FN-DSA-512 | ⚠️ 环境受限 | `tectonic-bedrock-python` 0.1.0 存在但需 Rust 工具链（rustup bootstrap 在本机失败），暂不可装；维持"缺库即存根"，需在具备 Rust 的环境安装后补跨语言向量 |
| P2-3 IDLRegistry | ✅ 已修 | 补 `decode_instructions` / `decode_event_data_by_tag` / `format_decoded_instruction` / `format_decoded_event`（含 `_format_value`），镜像 Go 输出格式；新增 3 个单测 |

修复后回归：**91 passed**（原 82 + 9 新增），`parity/check_parity.py` PARITY: PASS，`example/quickstart.py` 真机 DevNet 全链路通过。

---


## 一、已对齐并验证通过的核心面 ✅

| 领域 | 状态 | 说明 |
|---|---|---|
| Client / RPC 方法 | ✅ 一一对应 | Go `Client` 24 个方法（ClaimFaucet/CreateAccount/BalanceOf/ListAccountSigners/AccountSignerBit/GetChainHead/SubmitTx/SubmitTxWithSponsorIxes/SimulateTx/View/GetAccount/EventsByTxHash/GetBlockByHeight/GetTxByHash/GetTxHistoryProof/GetResource/GetResourcePathByHash/BatchGetResourcePathByHash/GetAccessValue/WaitForTransaction + 4 个构造/选项）全部有 Python 对应，签名等价 |
| Functional Options | ✅ | WithContext/WithRequestID/WithWait*/WithClientPoll* 全部实现 |
| MethodType 枚举 | ✅ 数值一致 | 1/5/10/15/20/25/50/55/60/150/155/160/165，与 Go `rpcRequest.go` 逐值核对 |
| 双线 RPC 协议 | ✅ | JSON 线 + Postcard 线，Content-Type 一致；真机 DevNet 联调通过（balance_of 返回真实服务端响应） |
| 交易构建/签名/提交 | ✅ | TransactionBuilder 全部链式方法（WithPayer/WithStamp/AddPayerSig/AddIxAndPayerSig/AddIxesSig/ApplySlots/SignWith/Simulate*、ResetSigs/Build）；`claim_faucet` 真机提交+轮询+解码 TxHistory 全链路成功 |
| Postcard 协议 | ✅ | varint/u8-u128/i8-i64/bool/bytes/str/option/seq/enum 与 Go 逐字段对齐；`PARITY: PASS`（20 个黄金向量） |
| IDL 动态绑定 | ✅ | 9 个 IDL + index.json 与 Go `provider/IDL/` **文件集完全一致**；`gen.Token/Account/System` 动态绑定可用 |
| Bitmap64 | ✅ | raw/is_empty/test/set/clear/count_ones/lowest_vacant_index/is_subset_of/iter_set_bits/low_bits_mask/marshal 全有 |
| 地址派生 | ✅ | BLAKE3(ROOT ‖ "milon.address.pk.v1" ‖ pk)[:20]，与 Go 一致（黄金向量校验） |
| 签名（Ed25519/Secp256k1） | ✅ | ed25519 用 stdlib；secp256k1 用 coincurve，可恢复 V=(sig[64]&1)+27，compact→DER 验签；跨语言对齐 |
| 交易/账户签名 | ✅ | AccountSignature/AccountSignatureBuilder/SigningSlot/Signer/Unsigned/CollectIxHashes 全有 |
| provider 层 | ✅ | Provider 的 Encode/Decode/DecodeViewDatas/DecodeViewData/DecodeDataByIDLTypeName、IDLTypeResolver 全有 |
| helper（部分） | ⚠️ | `check_simulate_success` 等 4 个函数**损坏**（见 P0-1） |

---

## 二、功能性缺口（会导致运行失败或功能缺失）

### 🔴 P0-1 `helper` 模块整体损坏（AttributeError）
- 现象：`helper.py` 的 `check_tx_success` / `check_simulate_success` / `get_account` / `events_by_tx_hash`
  访问 `result.body_tx_history` / `result.body_simulate_receipt` / `result.body_account_view`，
  但 `rpc.py` 结果类只暴露 **PascalCase** 属性（`BodyTxHistory` / `BodySimulateReceipt` / `BodyAccountView`），
  无蛇形别名（已实测 `hasattr(GetTxByHashResult(None,None), 'body_tx_history') == False`）。
- 影响：任何调用 helper 的代码都会抛 `AttributeError`。
- 根因：结果类与 helper 命名约定不一致；helper 无单测（82 个测试未覆盖）。
- 修复建议：在 `rpc.py` 结果类上加 `@property` 蛇形别名，或改 helper 访问 PascalCase 属性；并补 4 个单测。

### 🟠 P1-2 `get_resource_path_by_hash` 缺类型化解析
- Go 有 `api.ListResourcePathInfo` + `UnmarshalListResourcePathListFromRawList`（`[[rsHash数组, path], ...]` → 类型化列表）。
- Python `rpc.get_resource_path_by_hash` 直接 `json.loads` 返回**裸 JSON**，无 `ListResourcePathInfo` 类型与解析函数。
- 影响：路径查询结果无类型、无校验；`batch_get_resource_path_by_hash` 有 `BatchGetResourcePathInfo`（该侧已类型化）。
- 修复建议：补 `ListResourcePathInfo` + `unmarshal_list_resource_path_list_from_raw_list`（对齐 Go），
  并让 `get_resource_path_by_hash` 走类型化解析。

### 🟡 P2-3 `IDLRegistry` 缺 4 个方法
- 缺失：`decode_instructions`（批量）、`decode_event_data_by_tag`、`format_decoded_instruction`、`format_decoded_event`。
- Go `registry.go` 均有；Python 只有 `decode_instruction` / `decode_view_datas` 等。
- 影响：事件数据按 tag 解码与格式化展示能力缺失（辅助/调试用途）。

---

## 三、跨语言字节对齐高风险（未实证，需 Phase-0 spike）

### 🟠 P1-4 BLS12-381：当前回退 py_ecc，与 Go blst 不字节兼容
- Go 用 `supranational/blst`（CGO，DST = `BLS_SIG_BLS12381G2_XMD:SHA-256_SSWU_RO_NUL_`）。
- Python `crypto/_bls.py` 优先 `blst`，回退 `py_ecc`（G2Basic，DST = `..._POP_`）。
- 现状：本环境 `blst` **未安装**，走 py_ecc → **roundtrip 自洽可过，但与 Go 互验会失败**。
- 修复建议：`pip install blst` 并补一条跨语言 BLS 黄金向量（Go 生成 → Python 验）。

### 🟠 P1-5 FN-DSA-512（后量子）：当前不可用
- Go 用 `go-fn-dsa v0.2.0` 完整实现（KeyGen512/Sign512/Verify512）。
- Python `crypto/fn_dsa512.py` 依赖 `tectonic-bedrock-python`，**未安装** → `FnDsa512SecretKey` 调用即报错。
- 影响：后量子签名在 Python 侧目前是"能导入、不能算"。
- 修复建议：安装 bedrock 并做参数化跨语言 cross-check（评估文档已标记此为最高风险项）。

---

## 四、次要 API 缺口（不影响主链路）

| 项 | Go | Python | 备注 |
|---|---|---|---|
| Signature 类型访问器 | `ToSecp256k1/ToEd25519/ToBLS12381/ToFnDsa512` | ❌ 缺失 | 仅 `verify/verify_batch/as_bytes` |
| PublicKey 原生互转 | `ToSecp256k1/ToEd25519/ToBLS12381`、`From*Native` | ❌ 缺失 | 与底层库互操作场景 |
| JSON 编解码 | `Address/PublicKey/Signature.MarshalJSON/UnmarshalJSON` | ❌ 缺失 | 序列化到 JSON 场景 |
| AccountSignature | `AuthMessageForTx` | ❌ 缺失 | 有 `auth_message`（IxHashItem 版） |
| helper | `DisplayTxHistory/DisplayEventsByTxHash/DisplayGetAccount/DisplayAccountGetListSigners` | ❌ 缺失 | 展示辅助（cosmetic） |
| postcard 模块级函数 | `SerializeSeq/DeserializeSeq/SerializeOption/DeserializeOption/DeserializeValue/DeserializePostcardWithResolver` | 部分 | Python 以 Serializer/Deserializer **方法**实现（功能等价，函数面不同） |
| gen | `RegisterApp(name, binder)` 自定义应用注册 | ❌ 缺失 | `bind_all` 只处理已加载 IDL |
| Bitmap64 | `GoString` | ❌ 缺失 | 调试展示（cosmetic） |

---

## 五、设计差异（非缺口，仅说明）

- **错误处理**：Go 返回 `(T, error)`；Python 抛异常（RuntimeError/ValueError/MilonCryptoError）。语义等价，风格不同。
- **命名**：Go 全 PascalCase（`ToBytes`/`BalanceOf`）；Python 提供 snake_case（`to_bytes`/`balance_of`）为主、Go 风格别名（`gen.Token.BalanceOf`/`default_bindings`）为辅的双 API。
- **额外增强**：Python 独有 `gen.default_bindings()` Pythonic 容器、`Address.from_relaxed`、`load_providers` 等，属增量非缺口。

---

## 六、测试覆盖盲区（建议补）

82 个测试集中在：crypto（密钥/签名/地址）、postcard roundtrip、IDL 编解码、交易构建、Go 黄金向量 parity。
**未覆盖**（仅真机冒烟，无单测）：
- helper 全部函数（P0-1 由此漏网）
- RPC 响应解码：`ChainHead` / `Block` / `EventsByTxHash` / `GetResource` / `GetTxHistoryProof` / `GetAccessValue` / `BatchGetResourcePathByHash` / `GetResourcePathByHash`
- BLS 与 FN-DSA-512 的跨语言向量（P1-4/P1-5）
- `AccountSignatureBuilder.SignMultisigKey` / `SimulateSignMultisigKey`（Go 有多签路径）

---

## 七、优先级建议

1. **立即**：修 P0-1 helper 属性名（1 处，补 4 个测试）。
2. **尽快**：补 P1-2 `ListResourcePathInfo`；安装 `blst` 并加 BLS 跨语言向量；确认 FN-DSA-512 bedrock 方案。
3. **按需**：P2-3 IDLRegistry 方法；第四节各次要缺口（接入外部系统时再补）。
4. **长期**：为所有 RPC 响应解码补 golden 测试（用 Go `golden_gen.go` 扩展向量即可低成本覆盖）。

---

*验证基线：`pytest -q` → 82 passed；`parity/check_parity.py` → PARITY: PASS；`example/quickstart.py` 真机 DevNet 全链路通过。*
