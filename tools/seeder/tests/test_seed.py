# -*- coding: utf-8 -*-
"""seed.py 账户构建:spec 的曲线必须真实传到派生环节。

回归锚点:issuer 曾因 reg() 未透传 spec.curve 而按 secp256k1 派生,
导致 vc-flow 报"issuer 私钥类型不支持 VC 凭证签名"。
"""
from seedlib.plan import build_plan
import seed as seed_mod


class FakeApi:
    def derive_public_key(self, private_key, curve="secp256k1"):
        return f"PUB[{curve}]"

    def derive_address(self, public_key, curve="secp256k1"):
        return f"ADDR[{curve}]"


def test_build_accts_passes_spec_curve():
    plan = build_plan("small", "s1")
    accts = seed_mod.build_accts(FakeApi(), plan)
    assert accts["kycorg"]["curve"] == "ed25519"          # issuer: VC 签名要求
    assert accts["kycorg"]["publicKey"] == "PUB[ed25519]"
    assert accts["kycorg"]["address"] == "ADDR[ed25519]"
    assert accts["admin"]["curve"] == "secp256k1"
    assert accts["test01"]["curve"] == "secp256k1"
    assert accts["genesis"]["curve"] == "ed25519"          # SFT 资源账户
    assert accts["SEED0"]["curve"] == "secp256k1"          # 代币资源账户
    assert accts["SCUSD"]["curve"] == "secp256k1"          # 合规代币资源账户
