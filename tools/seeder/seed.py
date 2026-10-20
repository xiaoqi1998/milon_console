#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Milon devNet 一键环境播种器。

重新创世后一条命令重建全部测试数据:
  python seed.py --all                    # 全模块,full 规模
  python seed.py --all --scale small      # 快速冒烟
  python seed.py --tokens --sft           # 只跑部分模块
  python seed.py --compliance             # 依赖自动补齐(带 vc)
  python seed.py --all --dry-run          # 只打印计划,不上链

确定性:固定 seed 短语派生全部密钥,重新创世后重建出的地址/代币/SFT 地址不变。
断点续跑:进度写入 seed_state.json,中途失败直接重跑同一条命令。
"""
# ---------------------------------------------------------------------------
# DEPRECATED 2026-10-19: 播种器 v2 已原生进 API(POST /api/tool/seed,
# preset/overrides/config 参数化;模板+旋钮见控制台「测试数据」页)。
# 本 CLI 不再维护,仅作 v1 全量/小型矩阵的历史参考。
# 新参数模型:docs/superpowers/specs/2026-10-19-seed-builder-v2-design.md
# ---------------------------------------------------------------------------
import argparse
import sys
import time
from dataclasses import dataclass
from pathlib import Path

from seedlib import accounts as accounts_mod
from seedlib import compliance as compliance_mod
from seedlib import sft as sft_mod
from seedlib import tokens as tokens_mod
from seedlib import txs as txs_mod
from seedlib import vc as vc_mod
from seedlib.api import Api
from seedlib.context import Ctx
from seedlib.plan import DEFAULT_SEED_PHRASE, MODULE_ORDER, build_plan
from seedlib.state import State

HERE = Path(__file__).resolve().parent
STATE_PATH = HERE / "seed_state.json"

RUNNERS = {
    "accounts": accounts_mod.run,
    "tokens": tokens_mod.run,
    "sft": sft_mod.run,
    "vc": vc_mod.run,
    "compliance": compliance_mod.run,
    "txs": txs_mod.run,
}


def log(msg):
    print(f"[{time.strftime('%H:%M:%S')}] {msg}", flush=True)


def parse_args(argv=None):
    p = argparse.ArgumentParser(
        prog="seed.py", description="Milon devNet 一键环境播种器")
    p.add_argument("--all", action="store_true", help="全部模块按依赖顺序执行")
    for m in MODULE_ORDER:
        p.add_argument(f"--{m}", action="store_true", help=f"执行 {m} 模块")
    p.add_argument("--scale", choices=["full", "small"], default="full",
                   help="full=12用户/8代币/2集合 small=4用户/3代币/1集合(缺省 full)")
    p.add_argument("--seed-phrase", default=DEFAULT_SEED_PHRASE,
                   help=f"确定性派生短语(缺省 {DEFAULT_SEED_PHRASE})")
    p.add_argument("--base-url", default="http://127.0.0.1:18080",
                   help="milon-api-server 地址")
    p.add_argument("--dry-run", action="store_true", help="只打印计划,不上链")
    p.add_argument("--reset", action="store_true", help="忽略 seed_state.json 从头跑")
    p.add_argument("--list", action="store_true", help="列出模块后退出")
    a = p.parse_args(argv)
    if a.list:
        print("可用模块(按依赖顺序):")
        for m in MODULE_ORDER:
            print(f"  --{m}")
        print("一键全跑: --all    规模: --scale full|small    演练: --dry-run")
    mods = list(MODULE_ORDER) if a.all else [m for m in MODULE_ORDER if getattr(a, m)]
    if "compliance" in mods and "vc" not in mods:
        mods.insert(mods.index("compliance"), "vc")  # compliance 依赖已披露的证书
    a.modules = mods
    return a


def build_accts(api, plan) -> dict:
    """从 plan 派生全部确定性账户(server 权威派生公钥/地址)。"""
    accts = {}

    def reg(spec, role, curve=None, key_name=None):
        name = key_name or spec.name
        acct = {
            "name": name, "role": role,
            "curve": curve or getattr(spec, "curve", "secp256k1"),
            "privateKey": accounts_mod.derive_key(plan.seed_phrase, role, name),
            "subject_type": getattr(spec, "subject_type", "Personal"),
            "alias": getattr(spec, "alias", ""),
            "alias_suffix": getattr(spec, "alias_suffix", 0),
            "avatar_url": getattr(spec, "avatar_url", ""),
        }
        accounts_mod.ensure_keys(api, acct)
        accts[name] = acct
        return acct

    reg(plan.admin, "admin")
    for u in plan.users:
        reg(u, "user")
    for i in plan.issuers:
        reg(i, "issuer")  # 曲线取 spec.curve(issuer=ed25519,VC 签名要求)
    for t in plan.tokens:
        reg(t, "token", key_name=t.symbol)      # 代币资源账户(签 Create, gas 由 admin 付)
    reg(plan.compliance, "token", key_name=plan.compliance.symbol)  # 合规代币资源账户
    for s in plan.sfts:
        reg(s, "sft", curve="ed25519", key_name=s.key)  # SFT 资源账户(ed25519)
    return accts


def print_plan(plan, modules):
    print(f"== 播种计划(scale={plan.scale}, seed={plan.seed_phrase!r}) ==")
    print(f"账户: admin 1 + 用户 {len(plan.users)} + issuer {len(plan.issuers)}"
          f" + 资源账户 {len(plan.tokens) + len(plan.sfts) + 1}")
    print(f"  用户: {', '.join(u.name for u in plan.users)}")
    print(f"  别名: seed-test01-1001 … seed-test12-1012 / seed-kycorg-2001 …(重建后不变)")
    if "tokens" in modules:
        print(f"代币: {len(plan.tokens)} 个,精度 "
              f"{', '.join(str(t.decimals) for t in plan.tokens)}(图标/URI 为 picsum 真图)")
    if "sft" in modules:
        for s in plan.sfts:
            print(f"SFT {s.key}({s.symbol}): {len(s.scenarios)} 场景 → "
                  f"{', '.join(sc.key for sc in s.scenarios)}")
    if "vc" in modules:
        for v in plan.vc_issuers:
            print(f"VC: {v.issuer_name} 签发 {v.schema_prefix}1~{v.schema_count} → "
                  f"{len(v.certifies)} 人({v.certifies[0]}…{v.certifies[-1]})")
        for op in plan.vc_lifecycle:
            print(f"VC 生命周期: {op.kind} {op.issuer_name}→{op.user}")
    if "compliance" in modules:
        c = plan.compliance
        print(f"合规: {c.symbol}({c.decimals}dp) 挂证 {c.schema}@{c.issuer_name} "
              f"mode={c.mode};持证 {c.certified_receiver} 可收,无证 "
              f"{c.uncertified_victim} 521 拦截实测")
    if "txs" in modules:
        print(f"交易流: {len(plan.txs.mil_transfers)} 笔 MIL 互转 + 批量 + "
              f"{len(plan.txs.failures)} 笔预期失败(513)")
    print("(dry-run 不连接 server、不上链;真跑用 --all)")


def print_summary(ctx):
    st, plan = ctx.state, ctx.plan
    print("\n================ 播种完成 ================")
    for m, stages in st.data.items():
        print(f"  {m}: {len(stages)} 阶段")
    print(f"状态文件: {STATE_PATH}(引用测试数据以它为准)")
    print("关键地址:")
    print(f"  admin        {ctx.accts['admin']['address']}")
    print(f"  test01       {ctx.accts['test01']['address']}")
    for t in plan.tokens:
        print(f"  {t.symbol:<12} {ctx.accts[t.symbol]['address']} ({t.decimals}dp)")
    for s in plan.sfts:
        print(f"  SFT:{s.symbol:<9} {ctx.accts[s.key]['address']}")
    print("===========================================")


def main(argv=None) -> int:
    args = parse_args(argv)
    if args.list:
        return 0
    if not args.modules:
        print("未选择模块:用 --all,或 --accounts/--tokens/--sft/--vc/--compliance/--txs")
        return 2
    plan = build_plan(args.scale, args.seed_phrase)

    if args.dry_run:
        print_plan(plan, args.modules)
        return 0

    api = Api(args.base_url, log=log)
    health = api.health()
    log(f"server OK: {args.base_url} → {str(health.get('data'))[:120]}")

    state = State({}, STATE_PATH) if args.reset else State.load(STATE_PATH)
    log(f"派生确定性账户(seed={args.seed_phrase!r})…")
    accts = build_accts(api, plan)
    log(f"  {len(accts)} 个账户地址就绪(admin={accts['admin']['address'][:10]}…)")

    ctx = Ctx(api=api, state=state, plan=plan, accts=accts, log=log)
    for m in args.modules:
        log(f"===== 模块 {m} =====")
        RUNNERS[m](ctx)
        state.save()
    print_summary(ctx)
    return 0


if __name__ == "__main__":
    sys.exit(main())
