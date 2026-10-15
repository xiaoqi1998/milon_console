# -*- coding: utf-8 -*-
"""txs 模块:真实交易流——MIL 互转、批量转账、预期失败交易(513 等)。"""
from . import payloads
from .api import retry

MODULE = "txs"


def run(ctx):
    api, st, plan, log = ctx.api, ctx.state, ctx.plan, ctx.log

    for f, t, amt in plan.txs.mil_transfers:
        stage = f"mil:{f}->{t}"
        if st.is_done(MODULE, stage):
            continue
        src, dst = ctx.acct(f), ctx.acct(t)
        res = retry(lambda: api.write(
            "token", "Transfer",
            payloads.token_transfer_args(payloads.MIL_TOKEN_ADDRESS,
                                         src["address"], dst["address"], amt),
            src), log=log)
        if not res.ok and "513" not in (res.error or ""):
            raise RuntimeError(f"MIL 转账 {f}→{t} 失败: {res.error}")
        log(f"  MIL {f}→{t} {amt // 10**6} MIL {'✓' if res.ok else '余额不足,跳过'}")
        st.mark(MODULE, stage, {"from": f, "to": t, "amount": amt,
                                "txHash": res.tx_hash if res.ok else None,
                                "error": res.error})
        st.save()

    stage = "batch"
    if not st.is_done(MODULE, stage):
        src = ctx.acct(plan.txs.batch_from)
        res = retry(lambda: api.write(
            "token", "TransferBatch",
            {"from": src["address"], "token": payloads.MIL_TOKEN_ADDRESS,
             "to": [ctx.acct(t)["address"] for t in plan.txs.batch_to],
             "amount": list(plan.txs.batch_amounts)},
            src), log=log)
        if not res.ok and "513" not in (res.error or ""):
            raise RuntimeError(f"批量转账失败: {res.error}")
        log(f"  MIL 批量 {plan.txs.batch_from} → {len(plan.txs.batch_to)} 人 "
            f"{'✓' if res.ok else '余额不足,跳过'}")
        st.mark(MODULE, stage, {"from": plan.txs.batch_from,
                                "txHash": res.tx_hash if res.ok else None,
                                "error": res.error})
        st.save()

    for i, f in enumerate(plan.txs.failures):
        stage = f"failure:{f.kind}:{i}"
        if st.is_done(MODULE, stage):
            continue
        src = ctx.acct(f.by)
        ok, err = api.try_write(
            "token", "Transfer",
            payloads.token_transfer_args(payloads.MIL_TOKEN_ADDRESS,
                                         src["address"], ctx.acct("admin")["address"],
                                         f.amount), src)
        if ok:
            raise RuntimeError(f"预期失败交易({f.detail})竟然成功——环境异常!")
        if f.expect_code not in payloads.codes_in(err):
            raise RuntimeError(f"预期错误码 {f.expect_code} 不符: {err[:200]}")
        log(f"  预期失败实测 ✓ {f.detail}: {err[:80]}")
        st.mark(MODULE, stage, {"kind": f.kind, "expect": f.expect_code,
                                "error": err[:300]})
        st.save()
    log(f"txs 模块完成: {len(plan.txs.mil_transfers)} 笔互转 + 批量 + "
        f"{len(plan.txs.failures)} 笔预期失败")

