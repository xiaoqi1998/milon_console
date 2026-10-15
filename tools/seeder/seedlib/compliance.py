# -*- coding: utf-8 -*-
"""compliance 模块:合规代币(CreateWithCompliance)挂证书条件,
实测持证可收 / 无证 521 拦截——"代币挂上证书条件"的完整链路。"""
from . import payloads
from .api import retry

MODULE = "compliance"


def run(ctx):
    api, st, plan, log = ctx.api, ctx.state, ctx.plan, ctx.log
    c = plan.compliance
    token = ctx.acct(c.symbol)
    admin = ctx.acct("admin")
    issuer = ctx.acct(c.issuer_name)
    minter = ctx.acct(c.minter_user)
    receiver = ctx.acct(c.certified_receiver)
    victim = ctx.acct(c.uncertified_victim)
    mint_amt = 1000 * 10 ** c.decimals

    def mark(stage, data):
        st.mark(MODULE, stage, data)
        st.save()

    stage = "credential"
    if not st.is_done(MODULE, stage):
        cred_id = str(api.read("identity", "CredentialId",
                               {"issuer": issuer["address"],
                                "credential_schema": c.schema}))
        log(f"  凭证 ID({c.issuer_name}/{c.schema}) = {cred_id}")
        mark(stage, {"credentialId": cred_id})
    cred_id = st.get(MODULE, stage)["credentialId"]

    stage = "create"
    if not st.is_done(MODULE, stage):
        res = retry(lambda: api.write(
            "token", "CreateWithCompliance",
            payloads.compliance_create_args(token["address"], admin["address"],
                                            c, cred_id),
            admin, ix=token), log=log)
        if not res.ok and "516" in (res.error or ""):
            log(f"  {c.symbol} 链上已存在(516),跳过创建")
        elif not res.ok:
            raise RuntimeError(f"创建合规代币 {c.symbol} 失败: {res.error}")
        else:
            log(f"  合规代币 {c.symbol} 创建 ✓ tx={str(res.tx_hash)[:14]}…")
        mark(stage, {"address": token["address"], "txHash": res.tx_hash if res.ok else None})

    stage = "mode"
    if not st.is_done(MODULE, stage):
        res = retry(lambda: api.write(
            "token", "SetComplianceMode",
            payloads.compliance_mode_args(token["address"], c.mode), admin), log=log)
        if not res.ok:
            raise RuntimeError(f"设置合规模式失败: {res.error}")
        mark(stage, {"mode": c.mode})

    stage = "requirements"
    if not st.is_done(MODULE, stage):
        ok, err = api.try_write(
            "token", "AddComplianceRequirement",
            payloads.compliance_add_args(token["address"], [cred_id]), admin)
        if not ok and not ("duplicate" in err.lower() or "1064" in err or "520" in err):
            raise RuntimeError(f"添加合规凭证要求失败: {err}")
        mark(stage, {"credentialIds": [cred_id], "attempted": ok})

    stage = "mint"
    if not st.is_done(MODULE, stage):
        bal = api.read_u64("token", "BalanceOf",
                           {"token": token["address"], "account": minter["address"]})
        if bal >= mint_amt:
            log(f"  {c.symbol} minter 余额充足({bal}),跳过铸造")
        else:
            res = retry(lambda: api.write(
                "token", "MintBatch",
                payloads.token_mint_batch_args(token["address"],
                                               [minter["address"]], [mint_amt]),
                admin), log=log)
            if not res.ok:
                raise RuntimeError(f"合规代币铸造失败: {res.error}")
            log(f"  {c.symbol} 铸给 {c.minter_user} {mint_amt} ✓")
        mark(stage, {"amount": mint_amt, "to": c.minter_user})

    stage = "accept"
    if not st.is_done(MODULE, stage):
        bal = api.read_u64("token", "BalanceOf",
                           {"token": token["address"], "account": receiver["address"]})
        if bal > 0:
            log(f"  持证接收方已有余额({bal}),跳过成功转账")
        else:
            amt = 100 * 10 ** c.decimals
            res = retry(lambda: api.write(
                "token", "Transfer",
                payloads.token_transfer_args(token["address"], minter["address"],
                                             receiver["address"], amt),
                minter), log=log)
            if not res.ok:
                raise RuntimeError(f"持证用户转账失败(应当成功!): {res.error}")
            log(f"  持证用户 {c.certified_receiver} 收款 {amt} ✓ tx={str(res.tx_hash)[:14]}…")
        mark(stage, {"to": c.certified_receiver})

    stage = "reject"
    if not st.is_done(MODULE, stage):
        ok, err = api.try_write(
            "token", "Transfer",
            payloads.token_transfer_args(token["address"], minter["address"],
                                         victim["address"], 10), minter)
        if ok:
            raise RuntimeError(f"无证用户 {c.uncertified_victim} 竟然收到合规代币——环境异常!")
        # 521 VcRequired / 522 VcPolicyRequired 皆视为合规拦截(包装码语义可能漂移)
        if not {521, 522} & set(payloads.codes_in(err)):
            raise RuntimeError(f"无证拦截错误码异常(期望 521/522): {err[:200]}")
        log(f"  无证用户 {c.uncertified_victim} 被拦截 ✓: {err[:90]}")
        mark(stage, {"victim": c.uncertified_victim, "error": err[:300]})

    stage = "verify"
    if not st.is_done(MODULE, stage):
        comp = api.read("token", "Compliance", {"token": token["address"]})
        log(f"  链上合规配置: {comp}")
        mark(stage, {"compliance": comp})

    log(f"compliance 模块完成: {c.symbol} 挂证 [{c.schema}@{c.issuer_name}] "
        f"mode={c.mode},持证可收/无证 521 拦截均实测 ✓")

