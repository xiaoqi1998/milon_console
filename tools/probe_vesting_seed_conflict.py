# -*- coding: utf-8 -*-
"""T0 探针:同 token 第二笔 CreateVesting 是否仍报 539(vesting_id 派生输入复核)。

背景:2026-11-11 新链实测「同 token 第二笔必撞 539,name/start/recipient 全不参与
seed」,但 idl_method_docs.go 官方文档称 seed=creator+token+start_at_ms+recipients
全字段一致才 539。链 2026-11-20 又迁移端点(dev-api.milonlabs.com),本探针在当前
链实例上复核,结论决定 seed_runner 的 539 容忍语义与校验文案。

流程(admin=seed-admin 主角,自建探针代币 PC1):
  A : CreateVesting(recipients=[R1], start=now+25s)          —— 基线,须成功
  B1: 同 token 不同 recipients 参数(start 亦重算)            —— 观察 539?
  B2: 同 token 沿用 A 的 recipients 参数(仅 start 重算)      —— 观察 539?
可重复跑:A 已存在(539)视作基线成立继续;token 516 复用。
输出 probe_vesting_seed_conflict_report.json。
"""
import json
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
from verify_vesting import (  # noqa: E402
    post, read, write_tx, wait_tx, now_ms, rpc_chain_head,
    extract_vesting_id, log,
)

ADMIN = "2rHKRGW4vftL9rtWFnJSj7vhWNwJ"
ADMIN_PRIV = "f7c4b028d203b290184688a08f5e726c8db284e4e7c9fc30d0a842801fc143b9"
ADMIN_PUB = "03ae411e354c42f0c0129c7a32b20d2f445e9b7b43f1c2d1932ecc7a19df8a123c"
# 播种账号(seed 短语 milon-devnet-seed-v1 派生,recipient 仅登记不须存在)
R1 = "3stHPVEDhA9Y79hBqJV5cXgJjxKz"
R2 = "3N5H6gX6dpixeVxmyhmWVdPrS5UG"
MIL = "M11on1111111111111111111111"
MINT = 2_000_000
START_OFFSET_MS = 25_000
OUT = os.environ.get("PROBE_OUT", "probe_vesting_seed_conflict_report.json")


def mil_balance(addr):
    d = read("BalanceOf", {"token": MIL, "account": addr})
    return 0 if isinstance(d, dict) else int(d)


def token_balance(addr):
    d = read("BalanceOf", {"token": TOKEN, "account": addr})
    if isinstance(d, dict):
        return 0 if d.get("Code") == 512 else d
    return int(d)


def write_multi(method, args, signers):
    """multi_signer 双签:signers=[(addr,priv,pub),...]。"""
    signer_list = [{"address": a, "privateKey": p,
                    "signatureMode": {"type": "pubkey", "publicKey": pub}}
                   for a, p, pub in signers]
    return post("/api/write", {
        "appName": "token", "methodName": method, "args": args,
        "paymentMode": "multi_signer", "signers": signer_list}, timeout=120)


def start_ts():
    return max((rpc_chain_head() or {}).get("timestampMsecs") or 0, now_ms()) + START_OFFSET_MS


def error_code(resp):
    """链端错误码优先(simulate 报文形如 'TxFailurePayload { code: 539, ...'),
    httpStatus 500 会干扰裸 5\\d\\d 匹配。"""
    s = json.dumps(resp, ensure_ascii=False)
    m = re.search(r"code:\s*(\d+)", s)
    if m:
        return int(m.group(1))
    m = re.search(r"\b(5\d\d)\b", s)
    return int(m.group(1)) if m else None


def fresh_start_recipients(name, addr, total, init, ip, ps, periods):
    return [{"recipient": addr, "name": name, "total_amount": total,
             "initial_period_seconds": ip, "initial_amount": init,
             "period_seconds": ps, "periods": periods}]


def main():
    report = {"admin": ADMIN, "steps": {}}

    # ---------- 0. gas 自检(不足直接退出,领水有 24h 冷却不自动重试) ----------
    bal = mil_balance(ADMIN)
    report["adminMil"] = bal
    log(f"admin MIL 余额 = {bal}")
    if bal < 5_000_000:
        log("❌ admin MIL 不足(领水有冷却,请先手动补给)——终止")
        json.dump(report, open(OUT, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
        return

    # ---------- 1. 探针代币 PC1(幂等:报告已有则复用,否则创建) ----------
    global TOKEN
    if os.path.exists(OUT):
        old = json.load(open(OUT, encoding="utf-8"))
        if old.get("token"):
            TOKEN = old["token"]
            log(f"复用探针代币 {TOKEN}")
    if not TOKEN:
        res = (post("/api/accounts/generate", {}, timeout=15).get("data") or {})
        r = write_multi("Create", {
            "token": res["address"], "owner": ADMIN,
            "metadata": {"name": "ProbeConflict", "symbol": "PC1", "decimals": 6,
                         "icon": "https://scan.milonlabs.com/imgs/logo.svg",
                         "uri": "https://milon.dev/pc1.json"}},
            [(ADMIN, ADMIN_PRIV, ADMIN_PUB),
             (res["address"], res["privateKey"], res["publicKey"])])
        if not r.get("success"):
            if "516" in json.dumps(r):
                log("⚠ 代币已存在(516),跳过创建")
            else:
                log(f"❌ 代币创建失败: {json.dumps(r, ensure_ascii=False)[:300]}")
                json.dump(report, open(OUT, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
                return
        else:
            wait_tx(r["data"]["txHash"])
        TOKEN = res["address"]
        report["token"] = TOKEN
        log(f"✅ 探针代币 {TOKEN}")

    # ---------- 2. Mint 给 admin(余额不足才补铸) ----------
    if token_balance(ADMIN) < MINT:
        r, txh = write_tx("Mint", {"token": TOKEN, "to": ADMIN, "amount": MINT},
                          ADMIN, ADMIN_PRIV, ADMIN_PUB)
        if r.get("success"):
            wait_tx(txh)
            log(f"✅ Mint {MINT} PC1")
        else:
            log(f"❌ Mint 失败: {json.dumps(r, ensure_ascii=False)[:200]}")

    # ---------- 3. A 基线:CreateVesting(recipients=[R1]) ----------
    rec_a = fresh_start_recipients("A-base", R1, 600, 60, 3, 2, 15)
    r, txh = write_tx("CreateVesting", {"creator": ADMIN, "token": TOKEN,
                                        "start_at_ms": start_ts(), "recipients": rec_a},
                      ADMIN, ADMIN_PRIV, ADMIN_PUB)
    step_a = {"ok": bool(r.get("success")), "msg": json.dumps(r, ensure_ascii=False)[:200]}
    if step_a["ok"]:
        w = wait_tx(txh)
        step_a["vesting_id"] = extract_vesting_id(w)
        log(f"✅ A 基线建成 vesting_id={step_a['vesting_id']}")
    elif error_code(r) == 539:
        step_a["note"] = "已存在(重跑),视作基线成立"
        log("⚠ A 基线已存在(539),继续")
    else:
        log(f"❌ A 基线失败: {step_a['msg']}")
    report["steps"]["A_first"] = step_a

    # ---------- 4. B1:同 token 不同 recipients 参数 ----------
    rec_b1 = fresh_start_recipients("B1-diff", R2, 800, 80, 2, 3, 10)
    r, _ = write_tx("CreateVesting", {"creator": ADMIN, "token": TOKEN,
                                      "start_at_ms": start_ts(), "recipients": rec_b1},
                    ADMIN, ADMIN_PRIV, ADMIN_PUB)
    step_b1 = {"ok": bool(r.get("success")), "code": error_code(r),
               "msg": json.dumps(r, ensure_ascii=False)[:200]}
    log(f"B1(不同 recipients): ok={step_b1['ok']} code={step_b1['code']}")
    report["steps"]["B1_diff_recipients"] = step_b1

    # ---------- 5. B2:同 token 沿用 A 参数(仅 start 重算) ----------
    r, _ = write_tx("CreateVesting", {"creator": ADMIN, "token": TOKEN,
                                      "start_at_ms": start_ts(), "recipients": rec_a},
                    ADMIN, ADMIN_PRIV, ADMIN_PUB)
    step_b2 = {"ok": bool(r.get("success")), "code": error_code(r),
               "msg": json.dumps(r, ensure_ascii=False)[:200]}
    log(f"B2(同 recipients 仅 start 变): ok={step_b2['ok']} code={step_b2['code']}")
    report["steps"]["B2_same_recipients_diff_start"] = step_b2

    # ---------- 6. 结论 ----------
    if not step_b1["ok"] and step_b1["code"] == 539 and not step_b2["ok"] and step_b2["code"] == 539:
        report["conclusion"] = "同 token 第二笔恒 539(seed 只由 token 派生)——每计划须独立 token"
    elif step_b1["ok"] or step_b2["ok"]:
        report["conclusion"] = "同 token 不同参数可建多计划(seed 含参数字段)——文档口径成立"
    else:
        report["conclusion"] = "两笔均失败且非 539,需人工检查"
    log("结论: " + report["conclusion"])
    json.dump(report, open(OUT, "w", encoding="utf-8"), ensure_ascii=False, indent=1)


TOKEN = ""
if __name__ == "__main__":
    # 报告里已有 token 则提前载入(见 main 内复用逻辑)
    if os.path.exists(OUT):
        try:
            TOKEN = json.load(open(OUT, encoding="utf-8")).get("token") or ""
        except Exception:
            TOKEN = ""
    main()
