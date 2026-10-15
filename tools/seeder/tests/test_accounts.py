# -*- coding: utf-8 -*-
"""确定性密钥派生 + 领水单次尝试守卫。"""
import hashlib

from seedlib.accounts import derive_key, faucet_decision

FAUCET_AMOUNT = 10_000_000_000      # 每次领水 10,000 MIL(最小单位)
MIN_BALANCE = 100_000_000           # 100 MIL:低于它才需要领水


def test_derive_key_matches_contract():
    assert derive_key("sp", "user", "test01") == hashlib.sha256(b"sp:user:test01").hexdigest()


def test_derive_key_shape():
    k = derive_key("milon-devnet-seed-v1", "user", "test01")
    assert len(k) == 64
    int(k, 16)  # 必须是合法 hex


def test_derive_key_deterministic_and_distinct():
    a1 = derive_key("seed", "user", "test01")
    a2 = derive_key("seed", "user", "test01")
    b = derive_key("seed", "user", "test02")
    c = derive_key("seed", "issuer", "test01")
    d = derive_key("seed2", "user", "test01")
    assert a1 == a2
    assert len({a1, b, c, d}) == 4  # 名字/角色/短语任一不同 → 不同密钥


def test_faucet_decision_claim_when_poor_and_cool():
    assert faucet_decision(balance=0, cooldown_ms=0) == "claim"
    assert faucet_decision(balance=MIN_BALANCE - 1, cooldown_ms=0) == "claim"


def test_faucet_decision_skip_when_rich():
    assert faucet_decision(balance=MIN_BALANCE, cooldown_ms=0) == "skip"
    assert faucet_decision(balance=FAUCET_AMOUNT, cooldown_ms=0) == "skip"


def test_faucet_decision_never_claims_during_cooldown():
    # 冷却期内任何尝试都会续期冷却 → 唯一安全动作是跳过
    assert faucet_decision(balance=0, cooldown_ms=123456) == "skip"


def test_faucet_decision_claim_exactly_once_shape():
    # 全新创世账户:0 余额 + 无冷却 → 恰好一次 claim
    decisions = [faucet_decision(balance=0, cooldown_ms=0)]
    assert decisions.count("claim") == 1
