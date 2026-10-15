# -*- coding: utf-8 -*-
"""测试数据计划:纯数据、完全确定性。

build_plan(scale, seed_phrase) 只依赖这两个参数——
同一 seed 短语 + 同一规模,每次重建产出的地址/代币/SFT/证书矩阵完全一致。
"""
from dataclasses import dataclass, field
from typing import NamedTuple, Optional

from . import images

MODULE_ORDER = ["accounts", "tokens", "sft", "vc", "compliance", "txs"]
DEFAULT_SEED_PHRASE = "milon-devnet-seed-v1"

# 每精度基准铸造量(人类可读单位):raw = base*(i+1)*10^decimals。
# 新节点链端限制 decimals 0..=9(519),全精度 0~9 各建一代币;
# 各档总量均远小于 u64 上限
_HUMAN_BASE = {0: 1000, 1: 500, 2: 100, 3: 50, 4: 25, 5: 20,
               6: 50, 7: 20, 8: 20, 9: 10}


class Dist(NamedTuple):
    to: str            # 用户名或 "owner"(=集合 owner/admin)
    amount: int
    override: Optional[dict]


@dataclass(frozen=True)
class AccountSpec:
    name: str
    role: str
    curve: str = "secp256k1"
    subject_type: str = "Personal"
    alias: str = ""
    alias_suffix: int = 0
    avatar_url: str = ""


@dataclass(frozen=True)
class TokenSpec:
    symbol: str
    name: str
    decimals: int
    icon_url: str
    uri: str
    mint_amounts: dict          # 用户名 -> 最小单位数量
    freeze_demo_user: str = ""  # 非空则对该用户做一笔真实冻结(FrozenOf 数据)


@dataclass(frozen=True)
class ScenarioSpec:
    key: str
    name: str
    transferable: bool
    slot_meta: dict
    dists: tuple                # Dist 元组
    post_ops: tuple = ()


@dataclass(frozen=True)
class SftSpec:
    key: str
    name: str
    symbol: str
    royalty_bps: int
    cover_url: str
    metadata_url: str
    scenarios: tuple


@dataclass(frozen=True)
class VcIssuerSpec:
    issuer_name: str
    schema_prefix: str
    schema_count: int
    schema: str                 # 主 schema(prefix+1),合规代币引用它
    certifies: tuple            # 用户名元组


@dataclass(frozen=True)
class LifecycleOp:
    issuer_name: str
    kind: str                   # freeze | revoke
    user: str


@dataclass(frozen=True)
class ComplianceSpec:
    symbol: str
    name: str
    decimals: int
    icon_url: str
    uri: str
    issuer_name: str
    schema: str
    mode: str                   # All | Any
    minter_user: str
    certified_receiver: str
    uncertified_victim: str


@dataclass(frozen=True)
class FailureSpec:
    kind: str
    expect_code: int
    by: str
    amount: int
    detail: str


@dataclass(frozen=True)
class TxsSpec:
    mil_transfers: tuple        # (from, to, amount) 最小单位
    batch_from: str
    batch_to: tuple
    batch_amounts: tuple
    failures: tuple


@dataclass(frozen=True)
class Plan:
    scale: str
    seed_phrase: str
    admin: AccountSpec
    users: tuple
    issuers: tuple
    tokens: tuple
    sfts: tuple
    vc_issuers: tuple
    vc_lifecycle: tuple
    compliance: ComplianceSpec
    txs: TxsSpec


def _scenario_pack(sft_key: str, names: tuple) -> tuple:
    """差异化 slot 场景(移植 make_scenarios.py 六场景,按可用用户参数化)。"""
    def u(i):
        return names[i % len(names)]

    def meta(key, name, **extra):
        m = {"name": name, "coverUrl": images.slot_cover_url(sft_key, key),
             "attribute": f"scenario={key}"}
        m.update(extra)
        return m

    return (
        ScenarioSpec("no_transfer", "Slot-NoTransfer", False,
                     meta("no_transfer", "Slot-NoTransfer",
                          attribute="scenario=no_transfer;transferable=false"),
                     (Dist(u(0), 1, None), Dist(u(1), 1, None), Dist(u(2), 1, None)),
                     ({"op": "reject_transfer", "by": u(0), "to": u(3)},)),
        ScenarioSpec("metadata_full", "Slot-MetaFull", True,
                     meta("metadata_full", "Slot-MetaFull", symbol="SMF",
                          metadata="scenario=metadata_full;all-fields-overridden"),
                     (Dist(u(4), 1, None), Dist(u(4), 1, None))),
        ScenarioSpec("token_override", "Slot-TokenOverride", True,
                     meta("token_override", "Slot-TokenOverride"),
                     (Dist(u(0), 1, {"name": "OV-01", "symbol": "OV",
                                     "coverUrl": images.card_cover_url(sft_key, "token_override", 1),
                                     "metadata": "scenario=token_override;full-override",
                                     "attribute": "grade=S"}),
                      Dist(u(0), 1, {"name": "OV-02", "attribute": "grade=A"}))),
        ScenarioSpec("burn_shell", "Slot-BurnShell", True,
                     meta("burn_shell", "Slot-BurnShell",
                          attribute="scenario=burn_shell;3-minted-1-burned"),
                     (Dist(u(4), 1, {"name": "BS-01"}), Dist(u(4), 1, {"name": "BS-02"}),
                      Dist(u(4), 1, {"name": "BS-03"})),
                     ({"op": "burn", "by": u(4), "index": -1},)),
        ScenarioSpec("multi_holder", "Slot-MultiHolder", True,
                     meta("multi_holder", "Slot-MultiHolder",
                          attribute="scenario=multi_holder;1-token-3-holders"),
                     (Dist("owner", 5, {"name": "MH-01"}),),
                     ({"op": "transfer", "by": "owner", "to": u(1), "amount": 2},
                      {"op": "transfer", "by": "owner", "to": u(5), "amount": 2})),
        ScenarioSpec("empty", "Slot-Empty", True,
                     meta("empty", "Slot-Empty", attribute="scenario=empty;zero-tokens"), ()),
    )


ALL_SCENARIO_KEYS = ("no_transfer", "metadata_full", "token_override",
                     "burn_shell", "multi_holder", "empty")


def _tokens(decimals_seq, user_names, freeze_decimals):
    out = []
    for d in decimals_seq:
        base = _HUMAN_BASE[d]
        # d>=12 用人均等量:递增公式会让 18 位精度总量溢出 u64
        mult = (lambda i: i + 1) if d < 12 else (lambda i: 1)
        mint = {name: base * mult(i) * 10 ** d for i, name in enumerate(user_names)}
        out.append(TokenSpec(
            symbol=f"SEED{d}", name=f"Seed Token {d}dp", decimals=d,
            icon_url=images.token_icon_url(f"SEED{d}"),
            uri=images.token_uri(f"SEED{d}"),
            mint_amounts=mint,
            freeze_demo_user=user_names[4 % len(user_names)]
            if d == freeze_decimals else ""))
    return tuple(out)


def build_plan(scale: str, seed_phrase: str) -> Plan:
    if scale == "full":
        user_names = tuple(f"test{i:02d}" for i in range(1, 13))
        decimals = tuple(range(10))  # 0..9 全精度覆盖(链端上限 9)
        sfts = (
            SftSpec("genesis", "Seed Genesis Cards", "SGC", 250,
                    images.sft_cover_url("genesis"), images.picsum("milon-meta-sft-genesis", 400, 400),
                    tuple(sc for sc in _scenario_pack("genesis", user_names)
                          if sc.key in ALL_SCENARIO_KEYS)),
            SftSpec("gameitems", "Seed Game Items", "SGI", 0,
                    images.sft_cover_url("gameitems"), images.picsum("milon-meta-sft-gameitems", 400, 400),
                    tuple(sc for sc in _scenario_pack("gameitems", user_names)
                          if sc.key in ("multi_holder", "burn_shell", "token_override", "empty"))),
        )
        # issuer 必须用 Ed25519:VC 凭证签名链端仅支持 Ed25519 / FN-DSA-512
        issuers = (
            AccountSpec("kycorg", "issuer", curve="ed25519", subject_type="Organization",
                        alias="seed-kycorg", alias_suffix=2011,
                        avatar_url=images.avatar_url("kycorg")),
            AccountSpec("gameorg", "issuer", curve="ed25519", subject_type="Organization",
                        alias="seed-gameorg", alias_suffix=2012,
                        avatar_url=images.avatar_url("gameorg")),
        )
        vc_issuers = (
            VcIssuerSpec("kycorg", "SeedKyc", 2, "SeedKyc1",
                         ("admin",) + tuple(f"test{i:02d}" for i in range(1, 9))),
            VcIssuerSpec("gameorg", "SeedPlayer", 1, "SeedPlayer1",
                         tuple(f"test{i:02d}" for i in range(3, 11))),
        )
        vc_lifecycle = (LifecycleOp("gameorg", "freeze", "test05"),
                        LifecycleOp("gameorg", "revoke", "test08"))
        compliance = ComplianceSpec(
            "SCUSD", "Seed Compliant USD", 6,
            images.token_icon_url("SCUSD"), images.token_uri("SCUSD"),
            "kycorg", "SeedKyc1", "Any",
            minter_user="test02", certified_receiver="test01", uncertified_victim="test09")
        mil = 1_000_000
        txs = TxsSpec(
            mil_transfers=(("test01", "test03", 137 * mil), ("test03", "test05", 89 * mil),
                           ("test05", "test07", 45 * mil), ("test02", "test04", 200 * mil),
                           ("test04", "test06", 64 * mil), ("test06", "test08", 100 * mil),
                           ("test08", "test10", 33 * mil), ("test10", "test01", 21 * mil)),
            batch_from="test01",
            batch_to=("test02", "test03", "test04", "test05", "test06"),
            batch_amounts=(10 * mil,) * 5,
            failures=(FailureSpec("insufficient", 513, "test04", 2 ** 62,
                                  "超额转账触发 InsufficientBalance(513)"),),
        )
        admin = AccountSpec("admin", "admin", alias="seed-admin", alias_suffix=3001,
                            avatar_url=images.avatar_url("admin"))
    elif scale == "small":
        user_names = tuple(f"test{i:02d}" for i in range(1, 5))
        decimals = (0, 6, 9)
        sfts = (
            SftSpec("genesis", "Seed Genesis Cards", "SGC", 250,
                    images.sft_cover_url("genesis"), images.picsum("milon-meta-sft-genesis", 400, 400),
                    tuple(sc for sc in _scenario_pack("genesis", user_names)
                          if sc.key in ("no_transfer", "burn_shell", "multi_holder"))),
        )
        issuers = (AccountSpec("kycorg", "issuer", curve="ed25519",
                               subject_type="Organization",
                               alias="seed-kycorg", alias_suffix=2011,
                               avatar_url=images.avatar_url("kycorg")),)
        vc_issuers = (VcIssuerSpec("kycorg", "SeedKyc", 2, "SeedKyc1",
                                   ("admin", "test01", "test02", "test03")),)
        vc_lifecycle = ()
        compliance = ComplianceSpec(
            "SCUSD", "Seed Compliant USD", 6,
            images.token_icon_url("SCUSD"), images.token_uri("SCUSD"),
            "kycorg", "SeedKyc1", "Any",
            minter_user="test02", certified_receiver="test01", uncertified_victim="test04")
        mil = 1_000_000
        txs = TxsSpec(
            mil_transfers=(("test01", "test03", 137 * mil), ("test02", "test04", 89 * mil),
                           ("test03", "test01", 45 * mil)),
            batch_from="test01",
            batch_to=("test02", "test03"),
            batch_amounts=(10 * mil,) * 2,
            failures=(FailureSpec("insufficient", 513, "test04", 2 ** 62,
                                  "超额转账触发 InsufficientBalance(513)"),),
        )
        admin = AccountSpec("admin", "admin", alias="seed-admin", alias_suffix=3001,
                            avatar_url=images.avatar_url("admin"))
    else:
        raise ValueError(f"未知规模: {scale}(可选 full/small)")

    users = tuple(AccountSpec(name, "user", alias=f"seed-{name}", alias_suffix=1001 + i,
                              avatar_url=images.avatar_url(name))
                  for i, name in enumerate(user_names))
    tokens = _tokens(decimals, user_names, freeze_decimals=6 if scale == "small" else 8)
    return Plan(scale=scale, seed_phrase=seed_phrase, admin=admin, users=users,
                issuers=issuers, tokens=tokens, sfts=sfts, vc_issuers=vc_issuers,
                vc_lifecycle=vc_lifecycle, compliance=compliance, txs=txs)
