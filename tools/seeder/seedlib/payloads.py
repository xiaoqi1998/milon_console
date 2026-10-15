# -*- coding: utf-8 -*-
"""链上 payload 构造器:字段名与 IDL(/api/write)与编排端点严格对齐。

纯函数,无 IO——字段形状由 tests/test_payloads.py 钉死。
"""
MIL_TOKEN_ADDRESS = "M11on1111111111111111111111"

EXPECTED_CODES = {
    "insufficient": 513,        # token.InsufficientBalance
    "insufficient_approval": 514,
    "vc_required": 521,         # token.VcRequired(合规代币拦截)
    "vc_policy_required": 522,
}


def expected_code(kind: str) -> int:
    return EXPECTED_CODES[kind]


def codes_in(err: str) -> tuple:
    """从错误文本提取链端错误码(int 元组)。"""
    import re
    return tuple(int(c) for c in re.findall(r"\b(5\d\d|10\d\d|12\d\d)\b", err or ""))


def _metadata(spec) -> dict:
    return {"name": spec.name, "symbol": spec.symbol, "decimals": spec.decimals,
            "icon": spec.icon_url, "uri": spec.uri}


# ---------- token app ----------

def token_create_args(token_addr: str, owner_addr: str, spec) -> dict:
    return {"token": token_addr, "owner": owner_addr, "metadata": _metadata(spec)}


def token_mint_batch_args(token_addr: str, addrs, amounts) -> dict:
    return {"token": token_addr, "to": list(addrs), "amount": [int(a) for a in amounts]}


def token_transfer_args(token_addr: str, from_addr: str, to_addr: str, amount: int) -> dict:
    return {"from": from_addr, "token": token_addr, "to": to_addr, "amount": int(amount)}


def token_freeze_args(token_addr: str, holder: str, amount: int) -> dict:
    return {"token": token_addr, "holder": holder, "amount": int(amount)}


# ---------- sftoken(经 /api/tool/sft-flow 编排) ----------

def sft_flow_bootstrap(sft_addr: str, sft_priv: str, owner: dict, sftspec) -> dict:
    return {
        "ownerAddress": owner["address"], "ownerPrivateKey": owner["privateKey"],
        "sft": {"address": sft_addr, "privateKey": sft_priv},
        "sftMetadata": {"name": sftspec.name, "symbol": sftspec.symbol,
                        "coverUrl": sftspec.cover_url, "metadata": sftspec.metadata_url,
                        "attribute": f"collection={sftspec.key}"},
        "royaltyBps": sftspec.royalty_bps,
    }


def sft_flow_scenario(sft_addr: str, owner: dict, sc, users: dict) -> dict:
    """users: 名称(含 'owner')→ 地址。"""
    dists = []
    for d in sc.dists:
        item = {"to": users[d.to], "amount": d.amount}
        if d.override:
            item["metadata"] = dict(d.override)
        dists.append(item)
    return {"ownerAddress": owner["address"], "ownerPrivateKey": owner["privateKey"],
            "sft": {"address": sft_addr},
            "slot": {"isTransferable": sc.transferable, "metadata": dict(sc.slot_meta)},
            "distributions": dists}


# ---------- identity VC(经 /api/tool/vc-flow 编排) ----------

def vc_flow_payload(issuer: dict, user: dict, vi) -> dict:
    return {
        "issuerPrivateKey": issuer["privateKey"], "issuerAddress": issuer["address"],
        "userPrivateKey": user["privateKey"], "userAddress": user["address"],
        "credentialPrefix": vi.schema_prefix, "credentialCount": vi.schema_count,
        "issuerDid": {"alias": issuer["alias"], "suffix": issuer["alias_suffix"],
                      "avatarUri": issuer["avatar_url"]},
        "userDid": {"alias": user["alias"], "suffix": user["alias_suffix"],
                    "avatarUri": user["avatar_url"]},
    }


# ---------- 合规代币(token.CreateWithCompliance 系列) ----------

def compliance_create_args(token_addr: str, owner_addr: str, spec, credential_id: str) -> dict:
    return {"token": token_addr, "owner": owner_addr,
            "metadata": _metadata(spec), "credential_id": credential_id}


def compliance_mode_args(token_addr: str, mode: str) -> dict:
    return {"token": token_addr, "mode": mode}


def compliance_add_args(token_addr: str, credential_ids) -> dict:
    return {"token": token_addr, "credential_ids": list(credential_ids)}
