# -*- coding: utf-8 -*-
"""链上 payload 构造器:字段名与 IDL/端点严格对齐。"""
from seedlib import payloads
from seedlib.plan import build_plan

MIL = "M11on1111111111111111111111"


def _acct(name, alias=None, suffix=0):
    return {"name": name, "address": f"ADDR_{name}", "privateKey": f"PK_{name}",
            "publicKey": f"PUB_{name}", "alias": alias or f"seed-{name}",
            "alias_suffix": suffix, "avatar_url": f"https://picsum.photos/seed/ava-{name}/200/200"}


def test_token_create_args():
    p = build_plan("small", "s1")
    spec = p.tokens[0]
    a = payloads.token_create_args("TOKADDR", "OWNADDR", spec)
    assert set(a) == {"token", "owner", "metadata"}
    assert a["token"] == "TOKADDR" and a["owner"] == "OWNADDR"
    md = a["metadata"]
    assert set(md) == {"name", "symbol", "decimals", "icon", "uri"}
    assert md["symbol"] == spec.symbol and md["decimals"] == spec.decimals
    assert md["icon"] == spec.icon_url and md["uri"] == spec.uri


def test_token_mint_batch_args():
    to = ["A1", "A2", "A3"]
    amounts = [10, 20, 30]
    a = payloads.token_mint_batch_args("TOK", to, amounts)
    assert a == {"token": "TOK", "to": to, "amount": amounts}
    assert len(a["to"]) == len(a["amount"])


def test_token_transfer_and_mil_const():
    a = payloads.token_transfer_args("TOK", "FROM", "TO", 5)
    assert a == {"from": "FROM", "token": "TOK", "to": "TO", "amount": 5}
    mil = payloads.token_transfer_args(MIL, "FROM", "TO", 7)
    assert mil["token"] == MIL


def test_sft_flow_bootstrap():
    p = build_plan("small", "s1")
    s = p.sfts[0]
    a = payloads.sft_flow_bootstrap("SFTADDR", "SFTPRIV", _acct("owner"), s)
    assert a["sft"] == {"address": "SFTADDR", "privateKey": "SFTPRIV"}
    assert a["ownerAddress"] == "ADDR_owner"
    md = a["sftMetadata"]
    assert set(md) == {"name", "symbol", "coverUrl", "metadata", "attribute"}
    assert md["name"] == s.name and md["symbol"] == s.symbol
    assert md["coverUrl"] == s.cover_url
    assert a["royaltyBps"] == s.royalty_bps


def test_sft_flow_scenario_shape():
    p = build_plan("full", "s1")
    gen = {s.key: s for s in p.sfts}["genesis"]
    sc = {x.key: x for x in gen.scenarios}["burn_shell"]
    users = {u.name: f"ADDR_{u.name}" for u in p.users}
    users["owner"] = "ADDR_owner"
    a = payloads.sft_flow_scenario("SFTADDR", _acct("owner"), sc, users)
    assert a["sft"] == {"address": "SFTADDR"}
    assert a["slot"]["isTransferable"] is True
    assert a["slot"]["metadata"]["coverUrl"].startswith("https://picsum.photos/")
    assert len(a["distributions"]) == 3
    for d in a["distributions"]:
        assert set(d) <= {"to", "amount", "metadata"}
        assert d["amount"] == 1


def test_sft_flow_scenario_no_transfer_flag():
    p = build_plan("full", "s1")
    gen = {s.key: s for s in p.sfts}["genesis"]
    sc = {x.key: x for x in gen.scenarios}["no_transfer"]
    users = {u.name: f"ADDR_{u.name}" for u in p.users}
    users["owner"] = "ADDR_owner"
    a = payloads.sft_flow_scenario("SFTADDR", _acct("owner"), sc, users)
    assert a["slot"]["isTransferable"] is False


def test_vc_flow_payload():
    p = build_plan("small", "s1")
    vi = p.vc_issuers[0]
    issuer = _acct(vi.issuer_name, alias=p.issuers[0].alias, suffix=p.issuers[0].alias_suffix)
    uname = vi.certifies[0]  # "admin":合规代币 owner 须持证,发证名单含管理员
    user = _acct(uname, alias=p.admin.alias, suffix=p.admin.alias_suffix)
    a = payloads.vc_flow_payload(issuer, user, vi)
    assert a["issuerPrivateKey"] == f"PK_{vi.issuer_name}"
    assert a["issuerAddress"] == f"ADDR_{vi.issuer_name}"
    assert a["userAddress"] == f"ADDR_{uname}"
    assert a["credentialPrefix"] == vi.schema_prefix
    assert a["credentialCount"] == vi.schema_count
    # 别名后缀显式固定 → 重建后完整别名( alias-数字 )确定
    assert a["issuerDid"]["alias"] == p.issuers[0].alias
    assert a["issuerDid"]["suffix"] == p.issuers[0].alias_suffix
    assert a["issuerDid"]["avatarUri"].startswith("https://picsum.photos/")
    assert a["userDid"]["alias"] == user["alias"]
    assert a["userDid"]["suffix"] == user["alias_suffix"]


def test_compliance_args():
    p = build_plan("full", "s1")
    c = p.compliance
    a = payloads.compliance_create_args("CADDR", "OWN", c, "CRED_ID_1")
    assert set(a) == {"token", "owner", "metadata", "credential_id"}
    assert a["credential_id"] == "CRED_ID_1"
    assert a["metadata"]["decimals"] == 6
    assert payloads.compliance_mode_args("CADDR", "Any") == {"token": "CADDR", "mode": "Any"}
    add = payloads.compliance_add_args("CADDR", ["X", "Y"])
    assert add == {"token": "CADDR", "credential_ids": ["X", "Y"]}


def test_failure_expect_codes():
    p = build_plan("full", "s1")
    kinds = {x.kind: x.expect_code for x in p.txs.failures}
    assert kinds["insufficient"] == 513
    assert payloads.expected_code("insufficient") == 513
    assert payloads.expected_code("vc_required") == 521
