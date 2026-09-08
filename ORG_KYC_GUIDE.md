# 前端如何通过 IDL 创建「组织 KYC」使用说明

> 适用对象：接入 `milon-api-server` 的前端业务开发
> 关联 IDL：`identity`（app_id = 4）
> 配套接口：`/api/idl/metadata`（发现）、`/api/simulate`（模拟）、`/api/write`（落链）、`/api/read`（查询）

---

## 1. IDL 里的「组织 KYC」是什么

在 `identity` 应用里，“一个组织的 KYC”由三部分组成：

| 概念 | 对应 IDL 类型 / 方法 | 说明 |
| --- | --- | --- |
| 组织身份 | `DidSubjectType::Organization` | DID 主体的类型，建 DID 时指定 |
| 组织能力声明 | `identity.RegisterOrganization` | 声明该组织扮演的角色（如 `KycProvider`）与接受的凭证 schema |
| KYC 凭证 | `identity.DiscloseVcAttestation` | 由 KYC 提供方签发、组织（subject）披露上链的可验证凭证（VC） |

相关枚举（见 `identity.idl.json` 的 `types` 段）：

- `EntityRole`：`VcIssuer` / `KycProvider`
- `DidSubjectType`：`Personal` / `Organization`
- `OrganizationStatus`：`Active` / `Deactivated`

**结论**：前端“创建一个组织的 KYC”，本质是「先给组织建 DID（类型=Organization）→ 声明组织能力（含 KycProvider 角色）→ 由 KYC 提供方签发凭证、组织披露上链」三步（外加查询校验）。如果业务本身是要「注册一个做 KYC 的组织」，则只需前两步；如果要「给某个组织做 KYC 认证」，则三步都要。

---

## 2. 调用链路（顺序不能乱）

```
[1] identity.Create              # 建组织 DID（subject_type = Organization）
        │  必须先成功，否则下一步报 1053 OrganizationDidRequired
        ▼
[2] identity.RegisterOrganization# 声明角色 roles=["KycProvider"] + credential_schemas
        │
        ▼
[3] identity.DiscloseVcAttestation  # 组织作为 subject，披露 KYC 提供方签发的凭证
        │
        ▼
[4] 查询/校验 view 方法            # HasValidVcFromIssuer / VcAttestationCore / VcAttestationLifecycle
```

> 约束：`subject`（`Signer`）必须作为签名者。本服务的 `/api/write` 用 `unified_payer_all` 模式时，付款方 `payerAddress` 同时签 gas(bit63) 与指令(bit0)，因此 **`args.subject` 必须等于 `payerAddress`**。后端不会自动回填 signer 参数，前端务必在 `args` 里带上 `subject` 地址。

---

## 3. 每一步的前端请求示例

所有 `entry` 方法都走 `POST /api/write`，`kind=view` 走 `POST /api/read`。下面给出可直接复制的 JSON。

### 步骤 1：创建组织 DID —— `identity.Create`

```json
POST /api/write
{
  "appName": "identity",
  "methodName": "Create",
  "paymentMode": "unified_payer_all",
  "payerPrivateKey": "<组织私钥 hex/base58>",
  "payerAddress": "<组织地址 base58>",
  "signatureMode": { "type": "pubkey", "publicKey": "<组织公钥 hex/base58>" },
  "args": {
    "subject": "<组织地址 base58>",
    "doc": {
      "subject_type": "Organization",
      "keys": [
        { "public_key": "<组织公钥 hex/base58>", "label": "primary" }
      ],
      "services": [
        { "label": "official", "service_endpoint": "https://org.example.com" }
      ],
      "avatar_uri": "https://org.example.com/logo.png"
    }
  }
}
```

参数编码要点：
- `subject`：`Signer` 类型，传 base58 或 hex（20 字节）地址字符串。
- `doc`：`DidDocumentInput` 结构体，按字段名传对象。
- `subject_type`：枚举，直接传变体名字符串 `"Organization"`（大小写不敏感）。
- `keys[].public_key`：公钥，hex 或 base58 均可（`NewPublicKeyFromStringRelaxed`）。
- `label` / `avatar_uri`：普通字符串；`label` 可为 `null`（对应 `option<String>`）。

### 步骤 2：注册组织能力 —— `identity.RegisterOrganization`

```json
POST /api/write
{
  "appName": "identity",
  "methodName": "RegisterOrganization",
  "paymentMode": "unified_payer_all",
  "payerPrivateKey": "<组织私钥>",
  "payerAddress": "<组织地址 base58>",
  "signatureMode": { "type": "pubkey", "publicKey": "<组织公钥>" },
  "args": {
    "subject": "<组织地址 base58>",
    "roles": ["KycProvider"],
    "credential_schemas": ["kyc_basic_v1", "kyc_enterprise_v1"]
  }
}
```

参数编码要点：
- `roles`：`vec<EntityRole>`，传字符串数组，如 `["KycProvider"]` 或 `["VcIssuer","KycProvider"]`。变体名大小写不敏感。
- `credential_schemas`：`vec<String>`，字符串数组，建议与步骤 3 披露的 `credential_schema` 保持一致。
- 约束：角色不可重复（否则 1056）、角色数量非法（1055）、schema 不可重复（1059）；若声明 `KycProvider`/`VcIssuer` 角色则**必须**带 `credential_schemas`（否则 1060）。

### 步骤 3：披露 KYC 凭证 —— `identity.DiscloseVcAttestation`

这一步由**被认证的组织（subject）**发起，`issuer` 是已注册 `KycProvider` 的一方（仅提供地址与签名，不当签名者）。

```json
POST /api/write
{
  "appName": "identity",
  "methodName": "DiscloseVcAttestation",
  "paymentMode": "unified_payer_all",
  "payerPrivateKey": "<被认证组织私钥>",
  "payerAddress": "<被认证组织地址 base58>",
  "signatureMode": { "type": "pubkey", "publicKey": "<被认证组织公钥>" },
  "args": {
    "subject": "<被认证组织地址 base58>",
    "issuer": "<KYC 提供方地址 base58>",
    "issuer_key_id": 0,
    "credential_schema": "kyc_basic_v1",
    "credential_hash": "0x<64位hex的32字节哈希>",
    "valid_until_ms": 1767273600000,
    "issuer_signature": "0x<hex签名>"
  }
}
```

参数编码要点：
- `issuer`：`Address` 输入参数（**不是 signer**），只传地址。
- `issuer_key_id`：`u8` 数字。
- `credential_schema`：字符串，需与 `RegisterOrganization` 声明的 schema 之一对应。
- `credential_hash`：`B256`，32 字节，传 `0x` 前缀或纯 hex 字符串。
- `valid_until_ms`：`option<u64>`，传 `null`（永久有效）或毫秒时间戳数字。
- `issuer_signature`：`Signature`，hex 字符串，由 KYC 提供方用其第 `issuer_key_id` 把密钥对凭证内容签名得到（链上会校验 1069 `InvalidVcIssuerSignature`）。

### 步骤 4：查询 / 校验（view）

```json
POST /api/read
{
  "appName": "identity",
  "methodName": "HasValidVcFromIssuer",
  "args": {
    "subject": "<被认证组织地址 base58>",
    "issuer": "<KYC 提供方地址 base58>",
    "credential_schema": "kyc_basic_v1",
    "now_ms": 1756273600000
  }
}
```
返回 `{"value": true}` 表示当前有效。其它可用 view：`VcAttestationCore`、`VcAttestationLifecycle`、`OrganizationCapabilities`、`OrganizationStatus`。

---

## 4. 复杂参数编码速查（基于 `provider.Encode` 源码）

| IDL 类型 | 前端 JSON 写法 | 示例 |
| --- | --- | --- |
| `enum`（如 `DidSubjectType`/`EntityRole`） | 变体名字符串（大小写不敏感） | `"Organization"` / `"KycProvider"` |
| `struct`（如 `DidDocumentInput`） | 按字段名对象 | `{"subject_type":"Organization","keys":[...],"services":[...],"avatar_uri":"..."}` |
| `vec<T>` | 数组 | `["KycProvider"]` |
| `option<u64>` | `null` 或数字 | `null` / `1767273600000` |
| `Address` / `Signer` | base58 或 hex(20字节) 字符串 | `"1A1z..."` |
| `PublicKey` | hex 或 base58 字符串 | `"04a1b2..."` |
| `B256` | 32 字节 hex（可带 `0x`） | `"0x..."` |
| `Signature` | hex 字符串 | `"0x..."` |
| `u8`/`u64` | 数字 | `0` / `1756273600000` |
| `String` | 字符串 | `"kyc_basic_v1"` |

> 枚举对象写法也支持：`{"variant":"Organization"}` 或 `{"Organization": {}}`，但最简洁就是直接传字符串。

---

## 5. 前端集成建议

1. **元数据驱动**：先 `GET /api/idl/metadata`，按 `name=identity` 筛出方法，根据 `instructions[].args[].type` 动态渲染参数表单。新增/修改 IDL 方法无需改前后端代码。
2. **先模拟后落链**：敏感操作先用 `POST /api/simulate` 跑一遍，确认无 `1053/1032/1060/1069` 等错误再调 `/api/write`。
3. **signer 必须自洽**：所有 `role=signer` 的参数（如 `subject`）都要在 `args` 里显式传地址，且与 `payerAddress` 一致（统一支付模式）。
4. **错误码兜底**：前端按 `data.code` 展示对应提示，常见见下表。

---

## 6. 常见错误码（组织 KYC 相关）

| code | name | 含义 / 处理 |
| --- | --- | --- |
| 1053 | OrganizationDidRequired | 注册组织前必须先建 `subject_type=Organization` 的 DID（先跑步骤 1） |
| 1032 | OrganizationAlreadyExists | 该 DID 已注册组织，勿重复调用 |
| 1055 | InvalidOrganizationRoleCount | 角色数量非法 |
| 1056 | DuplicateOrganizationRole | 角色不可重复 |
| 1059 | DuplicateCredentialSchema | schema 不可重复 |
| 1060 | IssuerCredentialSchemaRequired | 声明 `KycProvider`/`VcIssuer` 必须带 `credential_schemas` |
| 1069 | InvalidVcIssuerSignature | `issuer_signature` 校验失败，检查签发方与 `issuer_key_id` |
| 1071 | VcAttestationAlreadyExists | 同一不可变内容的凭证已存在 |
| 1025 | DidNotFound | subject 的 DID 不存在，确认步骤 1 已成功 |

完整错误码见 `identity.idl.json` 的 `errors` 段与 `API.md`「错误码」章节。
