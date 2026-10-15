# -*- coding: utf-8 -*-
"""sft 模块:SFT 集合 + 差异化场景 slot(不可转移/元数据继承/覆盖/烧卡/多持有人/空槽)。"""
from . import payloads
from .api import retry

MODULE = "sft"
# 链端报这些码说明早前部分跑过、状态已就位(断点续跑兜底)
_ALREADY = ("1282", "1283", "1284", "1291")


def run(ctx):
    api, st, plan, log = ctx.api, ctx.state, ctx.plan, ctx.log
    admin = ctx.acct("admin")
    users_map = ctx.user_map()
    for sftspec in plan.sfts:
        res_acct = ctx.acct(sftspec.key)

        stage = f"create:{sftspec.key}"
        if not st.is_done(MODULE, stage):
            payload = payloads.sft_flow_bootstrap(
                res_acct["address"], res_acct["privateKey"], admin, sftspec)
            data = retry(lambda: api.sft_flow(payload), log=log)
            created = (data.get("sft") or {}).get("created")
            log(f"  SFT {sftspec.key}({sftspec.symbol}) "
                f"{'创建' if created else '已存在,复用'} → {res_acct['address'][:12]}…")
            st.mark(MODULE, stage, {"address": res_acct["address"],
                                    "created": bool(created)})
            st.save()

        sft_addr = res_acct["address"]
        for sc in sftspec.scenarios:
            stage = f"slot:{sftspec.key}:{sc.key}"
            if not st.is_done(MODULE, stage):
                payload = payloads.sft_flow_scenario(sft_addr, admin, sc, users_map)
                data = retry(lambda: api.sft_flow(payload), log=log)
                slot_id = (data.get("slot") or {}).get("slotId")
                token_ids = [d.get("tokenId") for d in (data.get("distributions") or [])]
                log(f"  [{sftspec.key}] slot {slot_id} [{sc.key}] "
                    f"transferable={sc.transferable} tokens={token_ids}")
                st.mark(MODULE, stage, {"slotId": slot_id, "tokenIds": token_ids,
                                        "transferable": sc.transferable})
                st.save()
            _post_ops(ctx, sftspec, sc, st.get(MODULE, stage), users_map)
    log(f"sft 模块完成: {len(plan.sfts)} 个集合 / "
        f"{sum(len(s.scenarios) for s in plan.sfts)} 个场景 slot")


def _post_ops(ctx, sftspec, sc, rec, users_map):
    """场景后置:烧卡、多持有人转移、不可转移拒绝实测。"""
    if not sc.post_ops or not rec or not rec.get("tokenIds"):
        return
    api, st, log = ctx.api, ctx.state, ctx.log
    admin = ctx.acct("admin")
    sft_addr = ctx.acct(sftspec.key)["address"]
    for i, op in enumerate(sc.post_ops):
        stage = f"post:{sftspec.key}:{sc.key}:{i}"
        if st.is_done(MODULE, stage):
            continue
        kind = op["op"]
        if kind == "burn":
            holder = ctx.acct(op["by"])
            tid = rec["tokenIds"][op["index"]]
            res = retry(lambda: api.write(
                "sftoken", "Burn",
                {"owner": holder["address"], "sft": sft_addr,
                 "token_id": tid, "amount": 1}, holder), log=log)
            if not res.ok and not any(c in (res.error or "") for c in _ALREADY):
                raise RuntimeError(f"burn 失败: {res.error}")
            log(f"  [{sftspec.key}/{sc.key}] burn token {tid} by {op['by']} "
                f"{'✓' if res.ok else '已完成,跳过'}")
            st.mark(MODULE, stage, {"op": kind, "tokenId": tid, "ok": res.ok,
                                    "error": res.error})
        elif kind == "transfer":
            tid = rec["tokenIds"][0]
            res = retry(lambda: api.write(
                "sftoken", "Transfer",
                {"owner": admin["address"], "sft": sft_addr, "token_id": tid,
                 "to": users_map[op["to"]], "amount": op["amount"]}, admin), log=log)
            if not res.ok and not any(c in (res.error or "") for c in _ALREADY):
                raise RuntimeError(f"transfer 失败: {res.error}")
            log(f"  [{sftspec.key}/{sc.key}] transfer token {tid} → {op['to']} "
                f"×{op['amount']} {'✓' if res.ok else '已完成,跳过'}")
            st.mark(MODULE, stage, {"op": kind, "tokenId": tid, "to": op["to"],
                                    "ok": res.ok, "error": res.error})
        elif kind == "reject_transfer":
            by = ctx.acct(op["by"])
            tid = rec["tokenIds"][0]
            ok, err = api.try_write(
                "sftoken", "Transfer",
                {"owner": by["address"], "sft": sft_addr, "token_id": tid,
                 "to": users_map[op["to"]], "amount": 1}, by)
            if ok:
                raise RuntimeError(f"[{sftspec.key}/{sc.key}] 不可转移槽位竟然转移成功——环境异常!")
            log(f"  [{sftspec.key}/{sc.key}] 拒绝转移实测 ✓ 被链端拦截: {err[:80]}")
            st.mark(MODULE, stage, {"op": kind, "ok": False, "error": err[:300]})
        st.save()
