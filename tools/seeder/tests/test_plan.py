# -*- coding: utf-8 -*-
"""plan:规模预设与确定性计划(全链测试数据的数据源)。"""
from seedlib.plan import MODULE_ORDER, build_plan


def test_module_order_fixed():
    assert MODULE_ORDER == ["accounts", "tokens", "sft", "vc", "compliance", "txs"]


def test_full_users():
    p = build_plan("full", "s1")
    assert [u.name for u in p.users] == [f"test{i:02d}" for i in range(1, 13)]
    assert all(u.role == "user" and u.curve == "secp256k1" for u in p.users)
    assert [u.alias_suffix for u in p.users] == list(range(1001, 1013))
    assert p.users[0].alias == "seed-test01"
    assert p.users[0].avatar_url.startswith("https://picsum.photos/seed/milon-ava-test01/")


def test_full_issuers():
    p = build_plan("full", "s1")
    names = [i.name for i in p.issuers]
    assert names == ["kycorg", "gameorg"]
    kyc, game = p.issuers
    assert kyc.subject_type == "Organization" and game.subject_type == "Organization"
    assert kyc.curve == "ed25519" and game.curve == "ed25519"  # VC 签名仅支持 Ed25519+
    assert kyc.alias == "seed-kycorg" and kyc.alias_suffix == 2011
    assert game.alias_suffix == 2012


def test_full_vc_issuers():
    p = build_plan("full", "s1")
    kyc, game = p.vc_issuers
    assert kyc.schema_prefix == "SeedKyc" and kyc.schema_count == 2
    assert kyc.schema == "SeedKyc1"
    assert kyc.certifies == ("admin",) + tuple(f"test{i:02d}" for i in range(1, 9))
    assert game.schema == "SeedPlayer1" and game.schema_count == 1
    assert game.certifies == tuple(f"test{i:02d}" for i in range(3, 11))


def test_full_tokens_cover_precisions():
    # 新节点链端限制 decimals 0..=9(519):覆盖全精度 0~9,一精度一代币
    p = build_plan("full", "s1")
    assert {t.decimals for t in p.tokens} == set(range(10))
    assert len(p.tokens) == 10
    syms = [t.symbol for t in p.tokens]
    assert len(set(syms)) == 10
    for t in p.tokens:
        assert t.name and t.icon_url.startswith("https://picsum.photos/")
        assert t.uri.startswith("https://picsum.photos/")
        assert set(t.mint_amounts) == {u.name for u in p.users}
        assert all(a > 0 for a in t.mint_amounts.values())


def test_full_sft_scenarios():
    p = build_plan("full", "s1")
    by_key = {s.key: s for s in p.sfts}
    assert set(by_key) == {"genesis", "gameitems"}
    gen = {sc.key for sc in by_key["genesis"].scenarios}
    assert gen == {"no_transfer", "metadata_full", "token_override",
                   "burn_shell", "multi_holder", "empty"}
    game = {sc.key for sc in by_key["gameitems"].scenarios}
    assert game == {"multi_holder", "burn_shell", "token_override", "empty"}
    sc = {x.key: x for x in by_key["genesis"].scenarios}
    assert sc["no_transfer"].transferable is False
    assert len(sc["burn_shell"].dists) == 3
    assert list(sc["multi_holder"].dists) == [("owner", 5, {"name": "MH-01"})]
    assert sc["empty"].dists == ()
    for s in p.sfts:
        assert s.cover_url.startswith("https://picsum.photos/")


def test_full_vc_and_lifecycle():
    p = build_plan("full", "s1")
    kyc = p.vc_issuers[0]
    assert kyc.issuer_name == "kycorg" and kyc.schema == "SeedKyc1"
    assert "test01" in kyc.certifies and "test09" not in kyc.certifies
    game = p.vc_issuers[1]
    assert game.schema == "SeedPlayer1"
    kinds = {(x.issuer_name, x.kind, x.user) for x in p.vc_lifecycle}
    assert ("gameorg", "freeze", "test05") in kinds
    assert ("gameorg", "revoke", "test08") in kinds


def test_full_compliance():
    p = build_plan("full", "s1")
    c = p.compliance
    assert c.symbol == "SCUSD" and c.decimals == 6
    assert c.issuer_name == "kycorg" and c.schema == "SeedKyc1"
    assert c.mode == "Any"
    assert c.certified_receiver in {f"test{i:02d}" for i in range(1, 9)}   # 持证
    assert c.uncertified_victim == "test09"                                # 无证 → 521
    assert c.minter_user in {u.name for u in p.users}


def test_token_mint_within_u64():
    # 链端 MintBatch 任一溢出整笔失败(517),总量也不得溢出 u64
    p = build_plan("full", "s1")
    for t in p.tokens:
        total = sum(t.mint_amounts.values())
        assert total < 2**64 - 1, f"{t.symbol} 总量溢出 u64"
        assert max(t.mint_amounts.values()) < 2**64 - 1


def test_freeze_demo_exactly_one_token():
    p = build_plan("full", "s1")
    demos = [t for t in p.tokens if t.freeze_demo_user]
    assert len(demos) == 1
    assert demos[0].freeze_demo_user in {u.name for u in p.users}
    assert sum(bool(t.freeze_demo_user) for t in build_plan("small", "s1").tokens) == 1


def test_full_txs():
    p = build_plan("full", "s1")
    assert len(p.txs.mil_transfers) >= 5
    for f, t, amt in p.txs.mil_transfers:
        assert f != t and amt > 0
        assert f in {u.name for u in p.users} and t in {u.name for u in p.users}
    kinds = {x.kind: x.expect_code for x in p.txs.failures}
    assert kinds.get("insufficient") == 513


def test_plan_internal_consistency():
    p = build_plan("full", "s1")
    names = {u.name for u in p.users} | {"owner", p.admin.name}
    for s in p.sfts:
        for sc in s.scenarios:
            for to, amt, _md in sc.dists:
                assert to in names and amt > 0
    for i in p.vc_issuers:
        assert set(i.certifies) <= names


def test_plan_deterministic():
    assert build_plan("full", "s1") == build_plan("full", "s1")
    assert build_plan("full", "s1") != build_plan("full", "s2")


def test_small_preset():
    p = build_plan("small", "s1")
    assert len(p.users) == 4
    assert len(p.tokens) == 3
    assert len(p.sfts) == 1 and len(p.sfts[0].scenarios) == 3
    assert len(p.vc_issuers) == 1
    assert p.compliance.uncertified_victim not in p.vc_issuers[0].certifies
