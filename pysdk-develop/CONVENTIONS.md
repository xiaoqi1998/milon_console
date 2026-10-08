# Python SDK 转换约定（所有模块必须遵守）

本约定用于将 `gosdk-develop/`（Go）逐文件、逐函数地翻译为
`pysdk-develop/src/milon_sdk/`（Python）。目标：**原文翻译、功能一致、测试完整**。

## 目录映射

| Go 文件 | Python 文件 |
|---|---|
| `gosdk-develop/fooBar.go` | `src/milon_sdk/foo_bar.py` |
| `gosdk-develop/api/base.go` | `src/milon_sdk/api/base.py` |
| `gosdk-develop/api/test/base_test.go` | `tests/api/test_base.py` |
| `gosdk-develop/crypto/secretkey.go` | `src/milon_sdk/crypto/secretkey.py` |
| 其余目录 1:1 对应 | |

`example/` 与 `tools/` 下的可执行程序另行处理，不在常规包映射内。

## 命名规则（必须统一，跨模块引用才不会漂移）

- **类型/类**：保留 Go 类型名（PascalCase）：`Transaction`、`Bitmap64`、`Serializer`、
  `PublicKey`、`FramedHistory`、`IDLRegistry` 等。
- **构造函数**：Go `NewXxx(...)` → 模块级函数 `new_xxx(...)`，
  例如 `NewClient` → `milon_sdk.client.new_client`；
  `NewTransactionBuilder` → `lib.transaction_builder.new_transaction_builder`。
- **方法/函数**：snake_case：`SerializeU32` → `serialize_u32`，
  `MarshalPostcard` → `marshal_postcard`，`ValidateWireWith` → `validate_wire_with`。
- **字段**：snake_case：`TxHash` → `tx_hash`；字节数组字段用 `bytes`
  （如 `PublicKey.Bytes` → `pk.bytes`）。
- **常量**：UPPER_SNAKE：`PublicKeyEd25519Size` → `PUBLIC_KEY_ED25519_SIZE`；
  `MilonTxHashDomainContext` → `MILON_TX_HASH_DOMAIN_CONTEXT`。
- **枚举**：Go `type XxxType uint8` + iota → `enum.IntEnum`（类名 `XxxType`），
  成员 UPPER_SNAKE：`PublicKeyTypeEd25519` → `PublicKeyType.ED25519`；
  同时导出模块级别名 `PUBLIC_KEY_TYPE_ED25519`。
- **包级变量**（如 `LocalNet`）：`milon_sdk.network.LOCAL_NET` / `DEV_NET`。

## 行为规则（保证功能一致）

1. **错误处理**：Go 返回 `(value, error)` → Python 直接 `raise`。
   - Go `errors.New("xxx")` / `fmt.Errorf` → `ValueError`/自定义异常。
   - Go 导出错误变量 `ErrInvalidSecretKey` → 异常类 `InvalidSecretKeyError`。
   - `fmt.Errorf("...: %w", err)` → `raise ... from err`，错误消息文本逐字保留。
2. **整数**：Go 有严格位宽；Python 序列化函数必须显式校验范围：
   - `serialize_u8(v)`: `0 <= v <= 0xFF`；`serialize_u16/u32/u64` 同理。
   - `serialize_i8..i64` 接受有符号数，越界报错；编码按补码 `v & mask`。
   - `u128` 用 `int`，范围 `[0, 2^128-1]`。
3. **Option**：Go `*T`（nil 表示无值）→ `Optional[T] = None`。
4. **序列/映射**：Go `[]T` / `map[K]V` → `list[T]` / `dict[K, V]`。
5. **JSON**：Go `MarshalJSON/UnmarshalJSON` → `to_json_value()` / `from_json_value()`，
   并提供模块级 `dumps_default` 供 `json.dumps(obj, default=...)` 使用。
6. **字符串表示**：Go `String()` → `__str__`；Go `GoString()` → `__repr__`。
7. **Blake3 域哈希**：`Hasher(domain)` =
   `blake3(out_length=32).update(MILON_ROOT_DOMAIN).update(domain)`。
8. **随机数**：Go `crypto/rand` → Python `os.urandom`。
9. **并发**：Go `sync.Mutex` 全局注册表 → Python `threading.RLock`。
10. **注释**：Go 注释（中英文）原样保留为 docstring/注释，函数顺序与
    Go 原文件一致。

## 密码学库对应关系

| Go 依赖 | 用途 | Python 对应 |
|---|---|---|
| `lukechampine.com/blake3` | BLAKE3 | `blake3` 包 |
| `decred/.../secp256k1` + `go-ethereum/crypto` | secp256k1 签名(65B R||S||V, V+27) | `coincurve`（`sign_recoverable`，手动 V+27） |
| `golang.org/x/crypto/ed25519` | Ed25519 | `cryptography` 的 `Ed25519PrivateKey` |
| `blst`（KeyGen=EIP-2333; G1 公钥/G2 签名; DST=nil） | BLS12-381 | `py_ecc`（G2Basic 域分隔）+ 自实现 EIP-2333 KeyGen + ZCash 压缩点序列化（48B/96B，flags 与 blst 一致） |
| `pornin/go-fn-dsa` | FN-DSA-512 | 纯 Python 移植 `milon_sdk.crypto._fndsa`（源：本机 Go module cache 的 go-fn-dsa@v0.2.0） |
| `btcsuite/btcutil/base58` | Base58 | `base58` 包 |

BLS 细节：DST = `BLS_SIG_BLS12381G2_XMD:SHA-256_SSWAP_RO_NUL_`（blst 传 nil DST
时的默认值，即 basic/min-sig 方案）；公钥 = G1 生成元×标量，压缩 48 字节；
签名 = G2 hash-to-curve(msg)×标量，压缩 96 字节。

### FN-DSA 移植接口契约（`milon_sdk/crypto/_fndsa/__init__.py`）

```python
LOG_N = 9
SIGN_KEY_LEN = 1281
VRFY_KEY_LEN = 897
SIG_LEN = 666

def keygen(log_n: int = LOG_N, seed: bytes | None = None) -> tuple[bytes, bytes]:
    """生成 (sign_key, vrfy_key)；seed 提供时为确定性密钥生成。"""

def sign(sign_key: bytes, ctx: bytes, hash_id: int, msg: bytes,
         seed: bytes | None = None) -> bytes:
    """签名，返回 666 字节签名。"""

def verify(vrfy_key: bytes, ctx: bytes, hash_id: int, msg: bytes, sig: bytes) -> bool:
```

与 Go `fndsa.KeyGen/Sign/Verify(logn=9, ctx=b"", hash_id=0)` 行为一致。

## 测试规则

- Go `xxx_test.go` → `tests/<pkg>/test_<file>.py`（目录对应 Go 包）。
- `testify/assert` → 直接 `assert` + `pytest.raises`；
  `assert.Equal(a, b)` → `assert b == a`；`assert.Error` → `pytest.raises`。
- 测试函数名 `TestFoo` → `test_foo`，测试向量（hex 等）原样保留。
- 每个被转换的 Go 源文件对应的测试必须存在并全部通过。

## 包导出

- `milon_sdk/__init__.py`：对应 Go 包 `milon` 的导出（`new_client` 等）。
- 各子包 `__init__.py`：re-export 对应 Go 包的全部导出符号。
