# milon Python SDK 全量移植实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 把 `gosdk-develop/`（103 文件 / 5.6 万行）逐文件对译为 `pysdk-develop/src/milon_sdk/`（包 `milon_sdk`），含全部 Go 测试移植与 idlgen 生成器移植，最终全量 pytest 绿。

**Architecture:** 按包依赖拓扑分 8 批推进（postcard/types 已完成）：crypto → api → provider → lib → helper+根目录 → gen(生成器) → 总验收。每任务 TDD：先移植 Go 测试（红）→ 对译实现（绿）→ 提交。移植规范主体是 `pysdk-develop/CONVENTIONS.md`。

**Tech Stack:** Python ≥3.10；blake3 / coincurve / cryptography / py_ecc / base58 / requests / grpcio / pytest。解释器：`/d/miniconda/python.exe`。

**Spec:** `docs/superpowers/specs/2026-10-08-pysdk-port-design.md`

## Global Constraints（全任务隐含遵守）

- 命名映射、错误处理、位宽校验、Option/序列映射、Blake3 域哈希、测试规则，全部按 `pysdk-develop/CONVENTIONS.md` 执行（类型 PascalCase；函数/字段 snake_case；`NewXxx`→`new_xxx`；常量 UPPER_SNAKE；Go `(v, err)` → raise）。
- Go 测试向量（hex、期望字节、期望错误消息）**逐字保留**；`assert.Equal(a,b)` → `assert b == a`；`assert.Error` → `pytest.raises`。
- 每个 Go 源文件对应 Python 文件 1:1；函数顺序与 Go 原文件一致；Go 注释保留为 docstring/注释。
- 提交信息用中文、 conventional 前缀（`feat(pysdk): ...` / `test(pysdk): ...`）；每个任务至少一个提交。
- 所有 pytest 命令在 `pysdk-develop/` 目录下执行：`/d/miniconda/python.exe -m pytest tests/... -v`。
- Git Bash 临时文件传给 Windows Python 前先 `cygpath -m` 转路径。
- 依赖已装于 `/d/miniconda`（pyproject 已声明）；如缺包 `pip install` 到该解释器。

---

### Task 0: 工程底座与存量测试（批0）

**Files:**
- Create: `pysdk-develop/tests/conftest.py`
- Create: `pysdk-develop/tests/__init__.py`（空）
- Create: `pysdk-develop/tests/postcard/__init__.py`、`pysdk-develop/tests/postcard/test_postcard.py`
- Create: `pysdk-develop/tests/types/__init__.py`、`pysdk-develop/tests/types/test_bitbap.py`
- Modify: `pysdk-develop/.gitignore`（新建，含 `__pycache__/`、`*.pyc`、`.pytest_cache/`）

**Interfaces:**
- Consumes: 已完成的 `milon_sdk.postcard`（`Serializer/Deserializer/serialize_postcard/deserialize_postcard` 等）、`milon_sdk.types.bitbap`。
- Produces: `tests/conftest.py` 把 `src/` 注入 `sys.path`（后续所有测试依赖）：

```python
import sys
from pathlib import Path

SRC = Path(__file__).resolve().parents[1] / "src"
if str(SRC) not in sys.path:
    sys.path.insert(0, str(SRC))
```

- [ ] **Step 1: 写 conftest.py、目录骨架与 .gitignore**
- [ ] **Step 2: 移植 `gosdk-develop/postcard/postcard_test.go`（274 行，4 个测试）为 `tests/postcard/test_postcard.py`**

测试函数映射：`TestSerializerDeserializerRoundTrip`→`test_serializer_deserializer_round_trip`；`TestPersonMatchesTypeScriptFixture`→`test_person_matches_type_script_fixture`（TS fixture 字节逐字保留）；`TestVarUint64RoundTrip`→`test_var_uint64_round_trip`；`TestErrorPaths`→`test_error_paths`；`TestPeekRemainingOffset`→`test_peek_remaining_offset`。Go 侧自定义 `Person` 结构（含 MarshalPostcard/UnmarshalPostcard）一并移植为测试内 dataclass。

- [ ] **Step 3: 跑测试验证** —— `/d/miniconda/python.exe -m pytest tests/postcard/ -v`，预期全 PASS。失败则修 `src/milon_sdk/postcard/` 实现（存量代码 bug 就地修复并注明）。
- [ ] **Step 4: 移植 `gosdk-develop/types/bitmap_test.go`（192 行，7 个测试）为 `tests/types/test_bitbap.py`**（SetAndClear/CountOnes/LowestVacantIndex/IsSubsetOf/IterSetBits/StringAndGoStringAndFormat/MarshalPostcard）
- [ ] **Step 5: 跑绿后提交** —— `git add pysdk-develop && git commit -m "test(pysdk): 补齐 postcard 与 bitbap 存量测试，锁住已译实现"`

---

### Task 1: crypto 包（批1，7 个子任务）

依赖：仅 postcard（已完成）。Go 源：`gosdk-develop/crypto/*.go`；目标：`src/milon_sdk/crypto/`。
通用 TDD 循环（每个子任务相同，下面只列文件与要点）：移植对应 `crypto/test/*_test.go` → 跑红 → 对译实现 → 跑绿 → 提交 `feat(pysdk): crypto/<文件>`。

- [ ] **1.1 `hash_domain.py` + `error.py`**（46+9 行）
  - `error.py`：Go 导出错误变量 → 异常类。先读 `crypto/error.go` 确认名字（如 `ErrInvalidSecretKey` → `InvalidSecretKeyError`），在 `crypto/__init__.py` re-export。
  - `hash_domain.py`：常量 `MILON_ROOT_DOMAIN_CONTEXT = "Milon-blake3"` 等 9 个域字符串逐字复制；`hasher(domain)` = `blake3(out_length=32).update(MILON_ROOT_DOMAIN.encode()).update(domain)`；`hash32(domain, *parts) -> bytes(32)`。
  - 测试：`test_hash_domain.py`（`TestHash32DeterministicAndDomainSeparated`、`TestHash32PartsEquivalentToConcat`）。
- [ ] **1.2 `_fndsa` 纯 Python 移植**（无对应 Go 测试文件；正确性由 1.3 的 fn_dsa512 测试背书）
  - 源：`/c/Users/xiaoqi/go/pkg/mod/github.com/pornin/go-fn-dsa@v0.2.0`（只取 logn=9 分支）。目标 `src/milon_sdk/crypto/_fndsa/`（`__init__.py` + 按源文件拆分模块）。
  - 接口契约（CONVENTIONS.md 已定）：`LOG_N=9; SIGN_KEY_LEN=1281; VRFY_KEY_LEN=897; SIG_LEN=666`；`keygen(log_n=9, seed=None) -> (sign_key, vrfy_key)`；`sign(sign_key, ctx, hash_id, msg, seed=None) -> bytes`；`verify(vrfy_key, ctx, hash_id, msg, sig) -> bool`。与 Go `fndsa.KeyGen/Sign/Verify(logn=9, ctx=b"", hash_id=0)` 行为一致。
  - 优先从该 module cache 里找 Go 侧测试向量（`*_test.go`、fips203/采样器向量）逐个对拍；纯 Python 性能不足时只保证正确性，不优化。
- [ ] **1.3 `fn_dsa512.py`**（103 行）：`SecretKeyBytesFnDsa512`/`PublicKeyBytesFnDsa512`/`SignatureBytesFnDsa512` 定长 bytes 类型 + `keygen_512() / new_sign_key_512_from_bytes(raw) / new_vrfy_key_512_from_bytes(raw) / sign_512(signKey, msg) / verify_512(vrfyKey, sig, msg)`。测试：`test_fn_dsa512.py`（7 个：TestKeygen512…TestNewVrfyKey512FromBytesWrongLen）。
- [ ] **1.4 `publickey.py`**（287 行）：`PublicKeyType(IntEnum)`（ED25519/SECP256K1/BLS12381/FN_DSA512，值按 Go iota 顺序）、`PublicKey` 类（`bytes` 字段 + `pk_type`）、`new_public_key_from_bytes/from_string_relaxed/from_ed25519_native/...`、`to_secp256k1/to_ed25519/to_bls12381/to_fn_dsa512`、`marshal_postcard/unmarshal_postcard`、`to_json_value/from_json_value`。BLS 公钥 = G1 压缩 48B（CONVENTIONS 的 py_ecc + ZCash 压缩细则）。测试：`test_publickey.py`（11 个测试）。
- [ ] **1.5 `signature.py`**（392 行）：`SignatureType(IntEnum)`、`Signature` 类、`new_signature_from_bytes/from_string_relaxed`、`as_bytes/to_hex/to_base58`、`to_*` 变体转换、`verify(msg, pub_key)`（四曲线分发）、`verify_batch(sigs, msgs, pub_keys)`（逐条验 + 长度校验，Go 并发语义等价于串行）、postcard/JSON 往返。测试：`test_signature.py`（14 个测试，含 TestVerifyBatchEmpty/LengthMismatch/Three/All）。
- [ ] **1.6 `secretkey.py`**（579 行）：`SecretKeyType(IntEnum)`、`SecretKeyer` 协议（Python `Protocol` 或 ABC）、`ClassicalSecretKey`（32 字节种子、四曲线派生：默认 secp256k1，见记忆 milon-server-handler-pitfalls）、`FnDsa512SecretKey`、`new_classical_secret_key/new_pure_classical_secret_key`、`sign_for(public_key, msg)`、`sign_secp256k1/sign_ed25519/sign_bls12381`、`zeroize()`。secp256k1 签名 65B `R||S||V`、V+27 由 coincurve `sign_recoverable` 手动处理。测试：`test_secretkey.py`（12 个测试）。
- [ ] **1.7 `address.py`**（158 行）：`Address` 类（32B）、`address_from_public_key(pub)`（域 `MILON_PK_ADDRESS_DOMAIN_CONTEXT`）、`from_bytes/from_string_relaxed/to_hex/to_base58/__str__`、postcard/JSON 往返。测试：`test_address.py`（5 个测试）。
- [ ] **1.8 收尾**：`crypto/__init__.py` re-export 全部导出符号（对照 Go 包导出清单逐一核对）；跑 `/d/miniconda/python.exe -m pytest tests/ -v` 全绿；提交。

---

### Task 2: api 包（批2，5 个子任务）

Go 源：`gosdk-develop/api/`；目标：`src/milon_sdk/api/`。均为 postcard 序列化结构体，模式统一：dataclass + `marshal_postcard(serializer)` / `unmarshal_postcard(deserializer)` + JSON。

- [ ] **2.1 `base.py`**（450 行，`base_test.go`）：请求基类与公共类型（先读源确认导出符号），测试 `tests/api/test_base.py`。
- [ ] **2.2 视图请求小件**：`block.py`(108)、`chain_head.py`(61)、`get_resource.py`(37)、`get_access_value.py`(77)、`list_resource_path.py`(46)、`batch_get_resource_path_by_hash.py`(47)、`account_view.py`(53)。测试合并放各自 `tests/api/test_<name>.py`（对应 7 个 Go 测试文件的用例；`TestSimulateReceipt_*`、`TestTxHistory_*` 中的 `WithRealProvider` 用例**本批跳过**，批3 回填）。
- [ ] **2.3 `events_by_tx_hash.py`**（159 行）+ `test_events_by_tx_hash.py`（4 个测试）。
- [ ] **2.4 `tx_history.py`**（310）+ `get_tx_history_proof.py`（86）+ `test_tx_history.py`（`TestTxHistory_WithError`、`TestTxHistory_DeserializeErrors`；`WithRealProvider` 留批3）。
- [ ] **2.5 `simulate_receipt.py`**（243）+ `test_simulate_receipt.py`（`WithError`、`DeserializeErrors`；`WithRealProvider` 留批3）。
- [ ] **2.6 收尾**：`api/__init__.py` re-export；全量 pytest 绿；提交。

---

### Task 3: provider 包（批3，4 个子任务）

- [ ] **3.1 `types.py`**（114 行）+ 拷贝 `gosdk-develop/provider/IDL/*.json` → `src/milon_sdk/provider/IDL/`（含 `index.json`，pyproject 已配置 package-data）。
- [ ] **3.2 `idl_type_resolver.py`**（111 行）+ `test_idl_type_resolver.py`（8 个测试：DecodeResource/Errors/BuiltinTypeFallback/ResourceWinsOverType/MultipleProviders/CollisionFirstWins/DecodeEvent/DecodeEvent_Errors）。
- [ ] **3.3 `registry.py`**（451 行）+ `test_registry.py`（15 个测试：NewIDLRegistry_Duplicates…FormatDecodedEvent）。
- [ ] **3.4 `provider.py`**（1585 行）+ `test_provider.py`（15 个测试：ProviderDemoEncodeAndDecode…FormatDecodedEvent 中归属 provider_test.go 的用例：TestProviderDemoEncodeAndDecode、TestProviderTokenEncodeAndDecode、TestDecode_Errors、TestDecodeViewDatas、TestDecodeViewDatas_ResultBranches、TestProviderReportsErrors、TestSerializeValue_*、TestDeserializeValue_*、TestSerializeEnumVariant、TestDecodeDataByIDLTypeName、TestDecodeViewVarint_Errors）。
- [ ] **3.5 回填批2 遗留**：启用 `TestSimulateReceipt_WithRealProvider`、`TestTxHistory_WithRealProvider` 的移植并跑绿；`provider/__init__.py` re-export；全量 pytest 绿；提交。

---

### Task 4: lib 包（批4，3 个子任务）

- [ ] **4.1 `rpc_request.py`**(103) + `rpc_response.py`(137) + 对应 2 个测试文件（`TestNewSubmitTransaction`、`TestSubmitTransaction_MarshalPostcard`、`TestSubmitTransaction_DeserializeErrors`、`TestRpcResponse_*`）。
- [ ] **4.2 `account_signature.py`**(391) + `account_signature_build.py`(184) + 2 个测试文件（AuthVoteIxes/VoteBatchHash/AuthIx/AuthIxes/AuthPayer/Unsigned/Sign/SignSkipPubKey/CollectIxHashes 等全部用例）。
- [ ] **4.3 `transaction.py`**(306) + `transaction_builder.py`(309) + 2 个测试文件（TxHash/AddSignature/IxHashes/IxHashFromWire/ValidateWire(With)/SerializeRoundTrip/UnifiedPayer*/SplitPayer/Simulate*/SignWith* 全部用例）。
- [ ] **4.4 收尾**：`lib/__init__.py` re-export；全量 pytest 绿；提交。

---

### Task 5: helper + 根目录（批5，2 个子任务）

- [ ] **5.1 helper 6 文件**：`check_simulate_success.py`(21)、`check_tx_success.py`(18)、`events_by_tx_hash.py`(34)、`get_account.py`(21)、`list_signers.py`(19)、`tx_history.py`(228)。无独立 Go 测试 → 每文件写最小冒烟测试（导入 + 典型输入路径），放 `tests/helper/`。
- [ ] **5.2 根目录 7 文件** → `src/milon_sdk/` 顶层模块：`network.py`(23)（`Network` dataclass + `LOCAL_NET`/`DEV_NET` 常量）、`client.py`(300)（`Client`、`new_client(config, *options)`、`with_client_poll_period/with_client_poll_timeout` 等选项函数；`RequestOption/WaitOption` 可调用对象）、`rpc_client_v1.py`(768)、`block_stream.py`(265)、`framed_history.py`(340)、`print_framed_history.py`(132)、`resolve_resource_paths.py`(49)。Go `RpcClientImpl` 接口 → `typing.Protocol`。无 Go 测试 → 冒烟测试：`new_client(DEV_NET)` 可构造、network 常量与 Go 逐字段相等。`milon_sdk/__init__.py` 补齐 Go 包 `milon` 的导出（对照 Go 侧导出符号清单）。
- [ ] **5.3 提交**（每子任务一提交）。

---

### Task 6: gen 包与 idlgen 生成器（批6）

- [ ] **6.1 `src/milon_sdk/gen/gen.py`**（对译 `gen.go` 45 行）：`Binder = Callable[[Provider], None]`、`register_app(app_name, binder)`、`bind_all(providers: dict[str, Provider])`，模块级 `_binders: dict` + `threading.RLock`；错误消息逐字（`"gen: provider for IDL app %q not loaded"` → f-string 等价文本）。
- [ ] **6.2 `pysdk-develop/tools/idlgen.py` 生成器**（对照 `gosdk-develop/tools/idlgen/main.go` 移植，Go 侧约 600 行）：
  - CLI：`python tools/idlgen.py --index src/milon_sdk/provider/IDL/index.json --out src/milon_sdk/gen/idl_gen.py`。
  - 输出头：`# Code generated by milon idlgen. DO NOT EDIT.` + `# Source: provider/IDL/index.json (regenerate with: python tools/idlgen.py)`。
  - 生成三件套：`DEFAULT_IDLS`（全 IDL 内联，dict 列表字面量，键名对齐 Go `provider.IDL` JSON tag）；每 app 一个类（如 `TokenApp`）+ 包级实例 `TOKEN`，instruction 成员为 builder 对象：`.args(...) -> self` 链式 + `.encode() -> list`（转 provider wire 格式）；自定义类型 dataclass + `decode_view(view_result.http_response_body)`。
  - `gen/__init__.py`：re-export `DEFAULT_IDLS`、各 app 实例、`register_app/bind_all`；生成文件末尾 `init` 时机对齐 Go（import 时 `register_app`）。
- [ ] **6.3 对账脚本 `pysdk-develop/tools/check_gen_parity.py`**：解析 Go `idl_gen.go` 与 Python `idl_gen.py`，断言：app 名集合相等、每 app instruction 名集合相等、每 instruction 判别器（Discriminator）数值映射相等、自定义类型名集合相等。输出差异清单，退出码非 0 表示不对账。
- [ ] **6.4 幂等检查**：重跑 idlgen → `git diff --exit-code src/milon_sdk/gen/idl_gen.py` 为空。
- [ ] **6.5 冒烟测试** `tests/gen/test_idl_gen.py`：`import milon_sdk.gen as gen`；`gen.TOKEN` 可用（或以 index.json 实际 app 为准）；随机挑 3 个 instruction 与 Go 侧 encode 结果对拍（可先手跑 Go example 取向量写死进测试）。提交。

---

### Task 7: 总验收（批7）

- [ ] **7.1** 全量 `/d/miniconda/python.exe -m pytest tests/ -v` 全绿；统计测试数 vs Go 侧测试函数总数（grep `^func Test`），差异需逐条说明（如性能型/竞态型用例不适用）。
- [ ] **7.2** 生成器幂等 + 对账脚本通过。
- [ ] **7.3**（可选）devNet 冒烟：`new_client(DEV_NET)` + `get_account` 查余额（遵循记忆：水龙头 24h 冷却勿重试、BalanceOf 新账户 512 属已知）。
- [ ] **7.4** 收尾提交 + 向用户汇报各批交付清单与测试计数。

---

## Self-Review 结论

- 覆盖检查：spec 8 批 ↔ Task 0–7 一一对应；idlgen 三件套、对账、幂等均有任务。✓
- 类型一致性：`new_client(config, *options)`、`hash32(domain, *parts)`、`register_app/bind_all` 等跨任务签名已固定。✓
- 无占位符：每任务有确切文件映射、测试函数清单与命令。✓
- 已知风险标记：`WithRealProvider` 两用例批3 回填；`_fndsa` 向量从 go module cache 提取。✓
