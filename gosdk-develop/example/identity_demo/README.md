# identity_demo —— Milon 链上身份三合约（did / entity_registry / compliance）

本目录演示 identity 应用（`app_id = 4`）的三个子合约，并给出可直接运行的 Go SDK 示例：

| 文件 | 内容 |
| --- | --- |
| [did_demo.go](./did_demo.go) | 个人 DID 全生命周期（create / key / service / avatar / alias / deactivate） |
| [registry_demo.go](./registry_demo.go) | 机构入驻开放目录、更新能力声明、永久退出 |
| [compliance_demo.go](./compliance_demo.go) | 最小链上 VC 结果：disclose / remove / revoke 与有效性查询 |
| [main.go](./main.go) | 编排：4 个测试账户 + 通用 helper（submit / view / 失败断言） |

运行（需要能访问 DevNet）：

```bash
cd example/identity_demo
go run .
# 或
go test -run Test_Main -v
```

---

## 1. 三个合约分别在做什么

identity 应用把"链上身份"拆成三个职责单一的模块：

| 合约 | 一句话职责 | 回答的问题 | 信任模型 |
| --- | --- | --- | --- |
| `did` | 身份锚点：每个地址可注册一份可拆分存储的 DID 文档 | 我是谁？ | 地址自控（`controller = subject`） |
| `entity_registry` | 开放机构目录：机构用 Organization DID 自声明角色与可签发凭证类型 | 我自称能做什么？ | 自声明，链上不审核 |
| `compliance` | 最小链上 VC（Verifiable Credential）结果：只存凭证摘要与生命周期状态 | 别人替我背书过什么？ | issuer 链下签发 + 链上验签结果 |

**关键设计思想：链上永远不背书现实资质。** 链能证明的只是"某个有活跃 DID 的地址，用它的 DID key 对这份凭证摘要签过名"；这个地址现实中是不是真的 KYC 机构，由链下世界判断（registry 只是把它公开声明出来，方便 dApp 枚举和筛选）。

三个合约的协作关系：

```
   ┌────────────────────────────────────────────────┐
   │                  1. did                        │
   │   地址注册 DID（Personal / Organization）        │
   │   管理 key / service / alias / avatar          │
   └───────┬────────────────────────────┬───────────┘
           │（机构 DID，可选前置）        │（个人 DID，disclose 必须前置）
           ▼                            ▼
   ┌──────────────────────────┐  ┌─────────────────────────────────┐
   │   2. entity_registry     │  │        3. compliance            │
   │  机构自声明 VcIssuer /    │  │  issuer 链下对凭证摘要签名        │
   │  KycProvider + 可签发     │  │  subject 上链 disclose 结果      │
   │  schema 清单             │  │  issuer 可 revoke（终态）        │
   │  credential_id 确定性索引 │  │  subject 可 remove（仅摘展示）    │
   └──────────────────────────┘  └─────────────────────────────────┘
```

三条最容易踩的规则（"没看懂"通常卡在这三点）：

1. **`disclose` 只要求 issuer 有活跃 DID，不要求它在 registry 注册。** registry 注册是"公开资质声明"，不是 VC 的前置条件。
2. **`remove_vc_disclosure` 只影响"展示索引"。** 移除后 `has_valid_vc_from_issuer` 仍然为 `true`——底层 attestation 没有被删除。
3. **只有 issuer 能 `revoke`（终态，不可复活）；subject 只能 `remove`。** remove 是"我不想展示"，revoke 是"我（签发方）宣布作废"。

---

## 2. did 合约（MIP-22 v1）

### 2.1 数据模型：一个 DID 拆成多份资源

DID 由地址本身定义（`did:milon` 地址型 DID），`controller` 就是 subject 自己。整份文档拆成独立资源存储，局部更新（如换一个 key）不需要重写全文：

| 资源 | 内容 | 对应 view |
| --- | --- | --- |
| `DidCore` | `subject`（类型 + 地址）、`controller` | `core` |
| `DidKeyIndex` | `initialized`、已占用的 key id 列表 | `key_index` |
| `DidKey` | 单个 key：`id`、`public_key`、`label` | `keys` / `key(id)` |
| `DidServiceIndex` | `initialized`、已占用的 service id 列表 | `service_index` |
| `DidService` | 单个 service：`id`、`label`、`service_endpoint` | `services` / `service(id)` |
| `DidAlias` | `initialized`、当前 `DidName`（可为空） | `alias` |
| `DidAvatar` | `initialized`、`avatar_uri` | `avatar` |
| `DidUpdatedAt` | 最近一次写操作的时间戳（ms） | `updated_at` |
| `DidDeactivate` | 停用标志（不可逆） | `deactivated` |
| `NameBinding` | 名称哈希 → (name, subject)，全局唯一 | `name_binding(name)` |

`document` view 会把上面这些资源合并成一份完整文档返回。

### 2.2 DID 名称（alias-suffix）

- 人类可读名称形如 `did:milon:alice-1001`，由 `alias` + `suffix` 组成；
- **归一化**：写入前去掉所有空白字符 + Unicode 转小写（不做同形字符 / 零宽字符归一化）；
- `alias`：归一化后 3-27 字符；`suffix`：1000-9999；
- 名称哈希：`hash32("milon.did.name.v1", alias_bytes || suffix_le(4B))` 取前 8 字节作为 `NameBinding` 资源索引；
- **全局唯一**：一个名称只能绑定一个 DID；同一 DID 可通过 `set_alias` 换名，旧绑定自动释放；
- `name_binding` view 的参数也会先归一化，所以查 `Alice-1001` 与 `alice-1001` 等价。

### 2.3 key / service / avatar 规则

**key**（上限 16 个）：

- `create` / `create_with_alias` 时必须携带 1-16 个 key，自动编号 `0..n`；
- `add_key` 自动分配**最小空闲 id**（删掉的 id 会被下一次添加复用）；
- `update_key` 保持 id 不变，替换 public_key / label；
- `remove_key` 不能删除最后一个 key（`CannotRemoveLastKey`）；
- `label`：去首尾空白后非空、≤64 字节。

**service**（上限 32 个）：

- `label`：字符集 `[a-zA-Z0-9-_.]`、≤64 字节；
- `service_endpoint`：必须是绝对 URL、scheme 只允许 `https` / `wss`、≤512 字节。

**avatar**：1-512 字节，`create` 时必填（不能为空）。

### 2.4 entry / view 一览

entry（11 个，全部要求 subject 本人签名）：

| entry | 说明 |
| --- | --- |
| `create` | 创建 DID 文档（无 alias） |
| `create_with_alias` | 创建 DID 文档并绑定名称（一步完成，避免先建后绑的空窗） |
| `add_key` | 添加 key，返回自动分配的 id |
| `update_key(id)` | 更新指定 key，id 不变 |
| `remove_key(id)` | 删除指定 key（不能删最后一个） |
| `add_service` | 添加 service，返回自动分配的 id |
| `update_service(id)` | 更新指定 service |
| `remove_service(id)` | 删除指定 service |
| `set_avatar_uri` | 更新头像 URI |
| `set_alias` | 设置/更换名称，旧绑定自动释放 |
| `deactivate` | 停用 DID（**不可逆**） |

view（13 个）：

| view | 返回 |
| --- | --- |
| `core(subject)` | subject + controller |
| `document(subject)` | 合并后的完整 DID 文档 |
| `key_index(subject)` | initialized + key id 列表 |
| `keys(subject)` | 全部 key |
| `key(subject, id)` | 单个 key |
| `service_index(subject)` | initialized + service id 列表 |
| `services(subject)` | 全部 service |
| `service(subject, id)` | 单个 service |
| `alias(subject)` | alias 状态 |
| `avatar(subject)` | avatar 状态 |
| `updated_at(subject)` | 更新时间戳 |
| `deactivated(subject)` | 是否已停用 |
| `name_binding(name)` | 名称 → subject 绑定（参数自动归一化） |

### 2.5 重要语义

- 重复 `create` → `DidAlreadyExists (1024)`；
- `deactivate` 不可逆；之后**所有写操作**（含 set_alias / add_key 等）报 `DidDeactivated (1026)`，读取不受影响；
- 针对不存在 DID 的写操作报 `DidNotFound (1025)`；但 `has_active_did` 语义下（如 issuer 检查）返回 false → 对应 `IssuerDidNotActive`。

---

## 3. entity_registry 合约

### 3.1 注册前置条件

`register_organization` 要求同时满足：

1. 有**活跃**（未停用）的 DID（否则 `DidNotFound 1025`）；
2. DID 的 `subject_type = Organization`（个人 DID 会被拒：`OrganizationDidRequired 1053`）；
3. 该地址从未注册过（重复注册 → `OrganizationAlreadyExists 1032`）。

### 3.2 角色与凭证 schema

- `roles`：1-4 个，不能重复；`EntityRole = VcIssuer | KycProvider`；
- 只要声明了 `VcIssuer` 或 `KycProvider`，**必须至少声明 1 个 credential schema**（`IssuerCredentialSchemaRequired 1060`）——避免"宣称能签发但不知道签什么"的空声明；
- credential schema：非空、≤96 字符、安全字符集 `[a-zA-Z0-9-_.:/#?=&%]`、每机构 ≤16 个、不能重复；
- `update_organization_capabilities` 为**全量覆盖**：roles 和 schemas 会一起被替换。

### 3.3 credential_id（确定性 ID）

为什么需要它：给"issuer + 凭证类型"一个稳定的引用标识，任何链下系统（dApp、钱包、扫描器）无需状态即可复算：

```
credential_id = base58( hash32("milon.identity.credential-id.v1",
                        chain_id_le(8B) || issuer.digest(20B) || schema_bytes)[..16] )
```

- 16 字节摘要做 Base58，得到短 ID；
- 客户端**不需要**自己复算：`credential_id(issuer, schema)` view 直接查；
- `credential_definition(credential_id)` view 反查 `{ issuer, credential_schema }`；
- 注册 / 更新声明时，链上会为每个 schema 写入对应的 `CredentialDefinition`（重复写入幂等）。

### 3.4 entry / view 一览

entry（3 个，均要求机构主体本人签名）：

| entry | 说明 |
| --- | --- |
| `register_organization(subject, roles, credential_schemas)` | 入驻目录并发布能力声明 |
| `update_organization_capabilities(subject, roles, credential_schemas)` | 全量更新声明（停用后调用报 1054） |
| `deactivate_organization(subject)` | 永久退出目录（**不可逆**） |

view（5 个）：

| view | 返回 |
| --- | --- |
| `credential_definition(credential_id)` | `{ issuer, credential_schema }` |
| `credential_id(issuer, schema)` | 确定性凭证 ID |
| `organization_capabilities(subject)` | `{ roles[], credential_schemas[] }` |
| `organization_status(subject)` | `Active` / `Deactivated` |
| `organization_updated_at(subject)` | 更新时间戳 |

### 3.5 重要语义

- 链上**不审核**机构资质，注册即公开声明；
- `deactivate_organization` 不可逆：之后能力更新报 `OrganizationDeactivated (1054)`，但 capabilities 数据仍可读取；
- 机构注册状态**不影响** compliance 的 disclose（见第 4 节）。

---

## 4. compliance 合约

### 4.1 链上只存"最小结果"

对每组 `(subject, schema, issuer)` 存两份资源：

| 资源 | 内容 |
| --- | --- |
| `VcAttestationCore` | `credential_schema` + `credential_hash (32B)` + `valid_until_ms (可选)` |
| `VcAttestationLifecycle` | `status (Active / Revoked)` + `updated_at_ms` |

外加两条展示索引：

| 资源 | 内容 |
| --- | --- |
| `AcceptedVcIssuerIndexMeta` + `AcceptedVcIssuers` | subject 公开的 schema → issuer 列表（含每个 issuer 的 credential_hash） |
| `DisclosedVcSchemas` | subject 公开过的 schema 列表 |

**VC 全文（姓名、生日、证件号……）永远不上链**，链上只有 32 字节摘要（`credential_hash`）。全文留在链下由双方自行保管，需要时链下出示、链上比对摘要即可。

### 4.2 disclose 的签名摘要（重点）

issuer 链下用自己 DID 文档里的 key（`issuer_key_id` 指定）对**固定 32 字节摘要**签名。SDK 必须逐字节复现（`example/identity_demo/compliance_demo.go` 中的 `vcAttestationSigningDigest` 即 Go 实现）：

```
digest = hash32( domain = "milon.identity.vc-attestation.v1",
                 chain_id (8B LE)
              || app_id = 0x04 (1B)              // identity 应用固定为 4
              || subject.digest (20B)
              || issuer.digest (20B)
              || issuer_key_id (1B)
              || len(credential_schema) (2B LE)
              || credential_schema (UTF-8)
              || credential_hash (32B)
              || expiry_tag (1B)                 // 0 = 无过期，1 = 有过期
              || expiry (8B LE)                  // 无过期时为 0
)
```

要点：

- 是对"字段拼接摘要"签名，**绝不是**对 JSON 文本签名；
- `hash32` = `Blake3("Milon-blake3" || domain || parts...)`，Go 侧对应 `crypto.Hash32`；
- Ed25519 是对该 32 字节摘要做**原始签名**（不是先哈希再签），Go 侧对应 `sk.SignEd25519(digest[:])`；
- 链上验签用 issuer DID 里 `issuer_key_id` 对应的 key。

### 4.3 disclose 的约束与幂等

- subject 必须有活跃 DID（否则 `DidNotFound 1025`）；
- issuer 必须有活跃 DID（否则 `IssuerDidNotActive 1034`）；**不要求** registry 注册；
- `valid_until_ms` 若已经是过去时间 → `VcAttestationNotCurrentlyValid 1067`；
- 签名校验：key 不存在 → `VcIssuerKeyNotFound 1071`；验签失败 → `InvalidVcIssuerSignature 1070`；
- 同一 `(subject, schema, issuer)` 已存在且内容完全一致 → **幂等成功**，且**不刷新时间戳**；
- 内容有任一不同（hash / expiry）→ `VcAttestationAlreadyExists 1072`；
- 已 revoke → `VcAttestationRevoked 1073`（不可复活）；
- 上限：每 `(subject, schema)` 最多 32 个 issuer（`TooManyAcceptedVcIssuers 1065`）；每 subject 最多 64 个 schema（`TooManyDisclosedVcSchemas 1066`）。

### 4.4 remove 与 revoke 对比（最容易混淆）

| 操作 | 签名者 | 效果 | 底层 attestation | `has_valid_vc_from_issuer` |
| --- | --- | --- | --- | --- |
| `remove_vc_disclosure` | subject | 从展示索引摘除该 issuer（schema 无 issuer 时同时摘除 schema） | **保留**（status 仍 Active） | 仍为 `true` |
| `revoke_vc_attestation` | issuer | status → `Revoked`（终态） | 保留但不可用 | `false` |

一句话：**remove 是"我不展示"，revoke 是"签发方宣布作废"。**

### 4.5 有效性判定（`has_valid_vc_from_issuer` / `disclosed_vcs.is_valid`）

```
valid = status == Active
     && (valid_until_ms == None || now <= valid_until_ms)
     && issuer 的 DID 存在且未停用
```

注意最后一条：**issuer 停了 DID，它签发的所有 VC 立即失效**。

### 4.6 entry / view 一览

entry（3 个）：

| entry | 签名者 | 说明 |
| --- | --- | --- |
| `disclose_vc_attestation(subject, issuer, issuer_key_id, schema, hash, valid_until, sig)` | subject | 公开 issuer 已链下签发的 VC 摘要 |
| `remove_vc_disclosure(subject, issuer, schema)` | subject | 从展示索引摘除（不删声明） |
| `revoke_vc_attestation(issuer, subject, schema)` | issuer | 作废其签发的 VC（终态） |

view（7 个）：

| view | 返回 |
| --- | --- |
| `vc_attestation_core(subject, schema, issuer)` | `{ schema, hash, valid_until }` |
| `vc_attestation_lifecycle(subject, schema, issuer)` | `{ status, updated_at }` |
| `accepted_vc_issuer_index_meta(subject, schema)` | 索引中的 schema 原值 |
| `accepted_vc_issuers(subject, schema)` | `[{ issuer, credential_hash }]` |
| `disclosed_vc_schemas(subject)` | `[schema...]` |
| `disclosed_vcs(subject, offset, limit)` | 分页摘要 `[{..., status, is_valid}]` |
| `has_valid_vc_from_issuer(subject, issuer, schema)` | `bool` |

### 4.7 identity 应用错误码全表（1024-1073）

| 码 | 名称 | 码 | 名称 |
| --- | --- | --- | --- |
| 1024 | DidAlreadyExists | 1049 | ServiceEndpointNotAbsolute |
| 1025 | DidNotFound | 1050 | ServiceEndpointInvalidScheme |
| 1026 | DidDeactivated | 1051 | ServiceNotFound |
| 1027 | InvalidDid | 1052 | DidServiceResourceMismatch |
| 1028 | NameAlreadyBound | 1053 | OrganizationDidRequired |
| 1029 | NameNotFound | 1054 | OrganizationDeactivated |
| 1030 | AddressNotSignatured | 1055 | InvalidOrganizationRoleCount |
| 1031 | OrganizationNotFound | 1056 | DuplicateOrganizationRole |
| 1032 | OrganizationAlreadyExists | 1057 | TooManyCredentialSchemas |
| 1033 | VcAttestationNotFound | 1058 | InvalidCredentialSchema |
| 1034 | IssuerDidNotActive | 1059 | DuplicateCredentialSchema |
| 1035 | NameInvalidLength | 1060 | IssuerCredentialSchemaRequired |
| 1036 | NameInvalidFormat | 1061 | CredentialIdCollision |
| 1037 | NameAliasInvalidLength | 1062 | InvalidCredentialId |
| 1038 | NameSuffixInvalid | 1063 | CredentialDefinitionNotFound |
| 1039 | NameHashCollision | 1064 | CredentialSchemaMismatch |
| 1040 | InvalidKeyCount | 1065 | TooManyAcceptedVcIssuers |
| 1041 | DidKeyNotFound | 1066 | TooManyDisclosedVcSchemas |
| 1042 | CannotRemoveLastKey | 1067 | VcAttestationNotCurrentlyValid |
| 1043 | DidKeyResourceMismatch | 1068 | VcAcceptanceIndexNotFound |
| 1044 | InvalidKeyLabel | 1069 | VcIssuerNotAccepted |
| 1045 | InvalidAvatarUri | 1070 | InvalidVcIssuerSignature |
| 1046 | TooManyServices | 1071 | VcIssuerKeyNotFound |
| 1047 | InvalidServiceLabel | 1072 | VcAttestationAlreadyExists |
| 1048 | ServiceEndpointInvalidLength | 1073 | VcAttestationRevoked |

链上失败时，Go SDK 的 `helper.CheckTxSuccess` 会 panic 并打印 error code；本 demo 中对"预期失败"的场景（如停用后写操作、重复 disclose）改用 `expectOnChainError` 打印错误码而不中断。

---

## 5. demo 说明

### 5.1 账户与角色

`main.go` 创建 4 个测试账户并自动 `ClaimFaucet` 领取测试 MIL：

| 账户 | 角色 |
| --- | --- |
| `alice` | 个人 DID，did_demo 全流程（最后 deactivate） |
| `kycOrg` | 机构 DID：registry 入驻 + compliance 的 issuer（全程保持 Active） |
| `tempOrg` | 机构 DID：演示 deactivate_organization 永久退出 |
| `bob` | 个人 DID：compliance 的 subject |

### 5.2 各 demo 覆盖的流程

**did_demo（alice）**：`create_with_alias` → 全套 view → `add_key` / `update_key` / `remove_key`（含 id 复用）→ `add_service` / `update_service` / `remove_service` → `set_avatar_uri` → `set_alias`（旧绑定释放）→ `deactivate`（+ 停用后写操作失败演示）。名字的 suffix（1000-9999）由账户地址派生，避免重复运行时名称已被占用。

**registry_demo（kycOrg / tempOrg）**：创建 Organization DID → `register_organization`（+ 重复注册报 1032 演示）→ `organization_capabilities` / `organization_status` / `organization_updated_at` / `credential_id` / `credential_definition` → `update_organization_capabilities`；tempOrg 演示 `deactivate_organization` 永久退出与停用后的失败（1054）。

**compliance_demo（kycOrg / bob）**：bob 建 DID → 构造 mock VC 文档并取 blake3 摘要 → **Go 侧复现签名摘要** + issuer 链下签名 → `disclose_vc_attestation` → 全套 view（含 `disclosed_vcs` 分页、`has_valid_vc_from_issuer`）→ 重复 disclose 幂等（时间戳不变）→ `remove_vc_disclosure`（展示清空但 `has_valid` 仍为 true）→ 重新 disclose → `revoke_vc_attestation`（`has_valid` 变 false）→ revoke 后 disclose 失败（1073）。

### 5.3 方法覆盖情况

- `did`：11 个 entry + 13 个 view 全覆盖；
- `entity_registry`：3 个 entry + 5 个 view 全覆盖；
- `compliance`：3 个 entry + 7 个 view 全覆盖。

---

## 6. 常见误区 FAQ

1. **为什么 `disclose` 不校验 registry 注册？**
   registry 是"公开资质声明"，不是准入名单。VC 的信任链条是"issuer DID key 的签名"，registry 只是给 dApp 提供一个可枚举、可展示的资质来源；链上不承担审核职责。

2. **`remove_vc_disclosure` 之后为什么 `has_valid_vc_from_issuer` 还是 true？**
   remove 只操作 subject 的"展示索引"；`VcAttestationCore/Lifecycle` 是 issuer 的声明，只有 issuer 能通过 revoke 改变其状态。

3. **为什么链上只存 `credential_hash`？**
   隐私与成本：VC 全文可能含个人敏感信息，上链不可撤回；32 字节摘要足以在链下出示全文时做完整性/一致性比对。

4. **为什么我传的 alias 和查出来的不一样？**
   写入前会做归一化（去空白 + 小写）。`alice-1001` 与 `Alice-1001` 是同一个名称。

5. **`add_key` 返回的 id 不一定是递增的？**
   对，分配的是**最小空闲 id**：删除 id 2 后再次添加会复用 2。id 是稳定的资源索引，不表示顺序。
