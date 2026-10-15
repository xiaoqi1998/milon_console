# -*- coding: utf-8 -*-
"""确定性密钥派生与账户基础操作。

privkey = sha256("{seed短语}:{角色}:{名字}") → 32 字节 hex。
公钥/地址一律由 server 派生(/api/util/*),不本地臆造——
同一 32 字节在不同曲线下地址不同,以 server 口径为准。
"""
import hashlib

from .api import ApiError

# faucet 每次发放 10,000 MIL(最小单位),每地址 24h 冷却,
# 冷却期内任何尝试(含失败)都会续期 → 唯一安全动作是跳过
FAUCET_AMOUNT = 10_000_000_000
MIN_BALANCE = 100_000_000  # 100 MIL,低于才需要领水


def derive_key(seed_phrase: str, role: str, name: str) -> str:
    return hashlib.sha256(f"{seed_phrase}:{role}:{name}".encode()).hexdigest()


def faucet_decision(balance: int, cooldown_ms: int = 0,
                    min_balance: int = MIN_BALANCE) -> str:
    if cooldown_ms > 0:
        return "skip"  # 冷却期内绝不重试,防止续期
    return "claim" if balance < min_balance else "skip"


def ensure_keys(api, acct: dict) -> dict:
    """补齐 acct 的 publicKey/address(幂等,已有一侧不重算)。"""
    curve = acct.get("curve", "secp256k1")
    if not acct.get("publicKey"):
        acct["publicKey"] = api.derive_public_key(acct["privateKey"], curve)
    if not acct.get("address"):
        acct["address"] = api.derive_address(acct["publicKey"], curve)
    return acct


def ensure_faucet(api, acct: dict, log=print) -> dict:
    """领水守卫:查余额+冷却视图,单次尝试。返回 {"balance","action","claimed","txHash","error"}。"""
    addr = acct["address"]
    bal = api.balance(addr)
    try:
        cooldown = api.read_u64("token", "FaucetCooldownRemaining", {"account": addr})
    except ApiError:
        cooldown = 0  # 冷却视图不可用时退化为仅余额判断
    action = faucet_decision(bal, cooldown)
    out = {"balance": bal, "cooldownMs": cooldown, "action": action,
           "claimed": False, "txHash": None, "error": None}
    if action == "claim":
        r = api.faucet_claim(acct)
        out["claimed"] = bool(r.get("claimed"))
        out["txHash"] = r.get("txHash")
        out["error"] = r.get("error")
        try:
            out["balance"] = api.balance(addr)
        except ApiError:
            out["balance"] = None  # 领水成功但视图暂不可用:不要让回读毁掉领水
        log(f"    领水 {addr[:10]}… claimed={out['claimed']} "
            f"bal={out['balance']} tx={str(out['txHash'])[:12]}…"
            f"{' err=' + str(out['error'])[:60] if out['error'] else ''}")
    else:
        reason = f"冷却中 {cooldown // 60000}min" if cooldown else f"余额充足({bal})"
        log(f"    跳过 {addr[:10]}… ({reason})")
    return out


def ensure_did(api, acct: dict, services=None, log=print) -> dict:
    """DID 一键创建(幂等,server 补齐差异):类型/别名/头像。"""
    payload = {
        "privateKey": acct["privateKey"],
        "address": acct["address"],
        "subjectType": acct.get("subject_type", "Personal"),
        "avatarUri": acct.get("avatar_url"),
    }
    if acct.get("alias"):
        payload["alias"] = acct["alias"]
        if acct.get("alias_suffix"):
            payload["suffix"] = acct["alias_suffix"]
    if services:
        payload["services"] = services
    r = api.post("/api/tool/did/create", payload)
    data = r.get("data") or {}
    log(f"    DID {acct['address'][:10]}… created={data.get('created')} "
        f"alias={acct.get('alias')}")
    return data


# ---------- accounts 模块入口 ----------

def run(ctx):
    """accounts 模块:全员领水(守卫单次尝试)+ DID(个人/组织)。"""
    api, st, plan, log = ctx.api, ctx.state, ctx.plan, ctx.log
    people = [plan.admin, *plan.users, *plan.issuers]

    if not st.is_done("accounts", "faucet"):
        log(f"  领水 {len(people)} 个账户(冷却守卫,单次尝试)…")
        results = {}
        for spec in people:
            results[spec.name] = ensure_faucet(api, ctx.acct(spec.name), log)
        st.mark("accounts", "faucet",
                {k: {"balance": v["balance"], "claimed": v["claimed"]}
                 for k, v in results.items()})
        st.save()

    if not st.is_done("accounts", "did"):
        log(f"  创建 DID {len(people)} 个(个人 {len(plan.users) + 1} / "
            f"组织 {len(plan.issuers)})…")
        for spec in people:
            ensure_did(api, ctx.acct(spec.name), log=log)
        st.mark("accounts", "did", {"count": len(people)})
        st.save()

    log(f"accounts 模块完成: {len(people)} 账户(admin {len(plan.users)} 用户 "
        f"{len(plan.issuers)} issuer),全部带 picsum 头像与确定性别名")
