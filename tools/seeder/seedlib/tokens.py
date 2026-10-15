# -*- coding: utf-8 -*-
"""tokens 模块:多精度代币创建 + MintBatch 分发 + 冻结演示。"""
from . import accounts as _accounts
from . import payloads
from .api import retry

MODULE = "tokens"


def _missing(ctx, tok_acct: dict, spec) -> bool:
    """链上余额是否低于计划铸造量(检测"提交未确认"的假阳性标记)。"""
    for n in spec.mint_amounts:
        have = ctx.api.read_u64(
            "token", "BalanceOf",
            {"token": tok_acct["address"], "account": ctx.acct(n)["address"]})
        if have < spec.mint_amounts[n]:
            return True
    return False


def run(ctx):
    api, st, plan, log = ctx.api, ctx.state, ctx.plan, ctx.log
    admin = ctx.acct("admin")
    for spec in plan.tokens:
        tok = ctx.acct(spec.symbol)

        stage = f"create:{spec.symbol}"
        if not st.is_done(MODULE, stage):
            res = retry(lambda: api.write(
                "token", "Create",
                payloads.token_create_args(tok["address"], admin["address"], spec),
                admin, ix=tok), log=log)
            if not res.ok and "516" in (res.error or ""):
                log(f"  {spec.symbol} 链上已存在(516),跳过创建")
            elif not res.ok:
                raise RuntimeError(f"创建 {spec.symbol} 失败: {res.error}")
            else:
                log(f"  {spec.symbol}({spec.decimals}dp) 创建 ✓ tx={str(res.tx_hash)[:14]}…")
            st.mark(MODULE, stage, {"address": tok["address"],
                                    "txHash": res.tx_hash if res.ok else None})
            st.save()

        mint_stage = f"mint:{spec.symbol}"
        if not st.is_done(MODULE, mint_stage) or _missing(ctx, tok, spec):
            # 余额驱动对账:历史标记可能是"提交未确认"的假阳性,
            # 以链上 BalanceOf 为准补铸差额
            names = list(spec.mint_amounts)
            holders = [ctx.acct(n) for n in names]
            diffs, to_mint = [], []
            for n, h in zip(names, holders):
                want = spec.mint_amounts[n]
                have = api.read_u64("token", "BalanceOf",
                                    {"token": tok["address"], "account": h["address"]})
                if have < want:
                    diffs.append(want - have)
                    to_mint.append(h["address"])
            if to_mint:
                args = payloads.token_mint_batch_args(tok["address"], to_mint, diffs)
                res = retry(lambda: api.write("token", "MintBatch", args, admin), log=log)
                if not res.ok:
                    raise RuntimeError(f"{spec.symbol} MintBatch 失败: {res.error}")
                log(f"  {spec.symbol} 补铸 {len(to_mint)} 人(差额合计 "
                    f"{sum(diffs)}) ✓ tx={str(res.tx_hash)[:14]}…")
            else:
                log(f"  {spec.symbol} 12 人余额均已到位,跳过铸造")
            st.mark(MODULE, mint_stage, {"holders": len(names),
                                         "confirmed": res.tx_hash if to_mint else "skip"})
            st.save()

        if spec.freeze_demo_user:
            fstage = f"freeze:{spec.symbol}"
            if not st.is_done(MODULE, fstage):
                holder = ctx.acct(spec.freeze_demo_user)
                holder_bal = api.read_u64(
                    "token", "BalanceOf",
                    {"token": tok["address"], "account": holder["address"]})
                amt = holder_bal // 3
                frozen = api.read_u64("token", "FrozenOf",
                                      {"token": tok["address"], "account": holder["address"]})
                if int(frozen) > 0:
                    log(f"  {spec.symbol} 冻结演示已完成(frozen={frozen}),跳过")
                elif amt <= 0:
                    log(f"  {spec.symbol} {spec.freeze_demo_user} 链上持有 0,跳过冻结演示")
                else:
                    res = retry(lambda: api.write(
                        "token", "Freeze",
                        payloads.token_freeze_args(tok["address"], holder["address"], amt),
                        admin), log=log)
                    if not res.ok:
                        raise RuntimeError(f"{spec.symbol} 冻结失败: {res.error}")
                    log(f"  {spec.symbol} 冻结 {spec.freeze_demo_user} {amt} ✓")
                st.mark(MODULE, fstage, {"holder": spec.freeze_demo_user,
                                         "amount": int(frozen) or amt})
                st.save()
    log(f"tokens 模块完成: {len(plan.tokens)} 个代币(精度 "
        f"{','.join(str(t.decimals) for t in plan.tokens)})")
