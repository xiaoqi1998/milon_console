# 跨 IDL 类型名冲突 bug（NFT 合约 500 错误）

## 一、错误现象

请求 `GET /api/transactions/5S9Aqgdv9C8vkA9d2rw3NL5X8yNSnipqa7yD6Vx8xJ9F` 返回 500：

```
failed to get transaction: failed to deserialize TxHistory:
  failed to deserialize Receipt:
    failed to deserialize Access records:
      failed to deserialize FirstSnapshot:
        failed to read Inline value (type_tag=16229100535211548774):
          TypeTagWithDataResolver.DecodeResource failed (type_tag=16229100535211548774):
            deserialize struct/enum MintConfig failed:
              deserialize struct field metadata (Metadata) failed:
                deserialize struct field icon (String) failed: invalid UTF-8 string
```

## 二、根本原因

这是**跨 IDL 类型名冲突 bug**，不是反序列化功能缺失。

错误链 `MintConfig → metadata (Metadata) → icon (String) invalid UTF-8` 暴露了问题：存在两个同名 `Metadata` 结构体：

| 来源 IDL | typeTag | 字段 |
|---|---|---|
| **nft.idl.json** | 5542048412392312772 | name, symbol, uri, external_url, attributes, properties |
| **token.idl.json** | 13005725662941815531 | name, symbol, decimals, **icon** |

错误信息里出现的 `icon` 字段是 token 的 Metadata，但实际交易是 NFT 的 `MintConfig`（其 `metadata` 字段应该是 nft 的 Metadata）。

## 三、代码缺陷位置

`idlTypeResolver.go:170-175` 的 `deserializeField` 在解析自定义类型时按**名称**查找，且遍历所有 Provider，第一个匹配就返回：

```go
for _, pd := range r.Providers {
    if idlType, ok := pd.GetIDLTypeByName(typeName); ok {  // 按名称查找!
        _, err := r.deserializeStructEnum(d, idlType)
        return err
    }
}
```

Go map 遍历顺序是随机的。当 token Provider 先被遍历时，`Metadata` 这个名字命中 token 的定义（带 `icon`），而不是 nft 的定义。结果用 token 的字段顺序去解码 nft 的字节流：

- name + symbol 读完后
- token 期望 `decimals(u8)`，但实际字节是 nft 的 `uri` 长度前缀
- 错位导致后续 `icon(String)` 读到无效 UTF-8

对比 `provider.go:577` 里的 `deserializeValue` 用的是 `p.IDLTypeByName[idlTypeName]`（当前 provider 作用域内查找），就没这个问题。

## 四、上下文丢失说明

`DecodeResource` 和 `DecodeEvent` 入口已经通过 typeTag 确定了正确的 Provider，但这个上下文在递归调用 `deserializeField` 时丢失了，导致递归回退到「按名字全局查找」的错误路径。

## 五、修复方案

给 resolver 加个 `currentProvider` 字段，进入 struct/enum 解码前设置，`deserializeField` 优先用它。

最简改法：

```go
type IDLTypeResolver struct {
    Providers       map[string]*Provider
    currentProvider *Provider  // 新增：递归上下文
}

// deserializeStructEnum 入口处设置 currentProvider
// deserializeField 查找自定义类型时优先用 currentProvider
```

## 六、影响范围

改完会影响 NFT 的 `MintConfig`、`Metadata`、`CollectionMetadata` 等所有跨合约同名类型的解码，也会顺带修复所有类似冲突（比如 token / nft 都有 `Metadata`、`Royalty` 等可能同名结构）。
