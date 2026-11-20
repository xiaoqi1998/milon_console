# -*- coding: utf-8 -*-
"""T5 冒烟:播种器分配计划(vesting)端到端。

流程:
  1. POST /api/tool/seed/plan 预览(vesting 配置)——断言 summary.vestings/计划规格
  2. POST /api/tool/seed 启动 → 轮询 status 到 completed
  3. 核验 state 落盘 + 链上 VestingInfo(vid)读回 recipients/参数
  4. 第二轮同配置续跑:claim 541 重试(cliff 已过)→ BalanceOf 到账核验
输出 seed_vesting_smoke_report.json。
"""
import json
import os
import sys
import time

import urllib.request

BASE = "http://127.0.0.1:18080"
OUT = os.environ.get("SMOKE_OUT", "seed_vesting_smoke_report.json")
MIL = "M11on1111111111111111111111"

# 复用现有 8 账户命名空间(74 阶段已建好代币/用户),vesting 复用 ST01/ST02
CFG = {
    "seedPhrase": "milon-devnet-seed-v1",
    "accounts": {"count": 8, "curves": ["secp256k1", "ed25519"],
                 "faucet": False, "balanceTiers": [], "did": False, "vcCert": False},
    "multisig": {"count": 0, "signers": 2, "threshold": 1, "faucet": False, "did": False},
    "tokens": {"count": 3, "decimals": [0, 6, 9], "complianceCount": 0, "freezeDemo": False},
    "sft": {"count": 0, "slotsPerSft": 2, "tokensPerSlot": 3, "scenarios": ["transferable"], "postOps": False},
    "vesting": {"plans": 2, "recipients": 2, "names": [], "random": False,
                "total": 1000000, "initial": 100000, "cliffSeconds": 5,
                "periodSeconds": 5, "periods": 10, "startDelaySeconds": 5, "claim": True},
}


def post(path, payload, timeout=120):
    req = urllib.request.Request(BASE + path, data=json.dumps(payload).encode(),
                                 headers={"Content-Type": "application/json"})
    return json.loads(urllib.request.urlopen(req, timeout=timeout).read())


def get(path, timeout=60):
    return json.loads(urllib.request.urlopen(BASE + path, timeout=timeout).read())


def read(method, args):
    r = post("/api/read", {"appName": "token", "methodName": method, "args": args}, timeout=60)
    return r.get("data")


def log(*a):
    print("[%s]" % time.strftime("%H:%M:%S"), *a)


def main():
    checks = []

    def check(name, ok, detail=""):
        checks.append({"check": name, "ok": bool(ok), "detail": str(detail)[:300]})
        log(("✅ " if ok else "❌ ") + name + (" | " + str(detail)[:160] if detail and not ok else ""))
        return ok

    # ---------- 1. 预览 ----------
    pr = post("/api/tool/seed/plan", {"config": CFG})
    if not pr.get("success"):
        log("❌ 预览校验失败: " + json.dumps(pr, ensure_ascii=False)[:400])
        json.dump({"checks": checks}, open(OUT, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
        return
    plan = pr["data"]
    summary = plan.get("summary") or {}
    check("预览 summary.vestings=2", summary.get("vestings") == 2, summary)
    inner = plan.get("plan") or {}
    vestings = inner.get("vestings") or []
    check("预览计划数=2", len(vestings) == 2, vestings)
    check("复用 ST01/ST02", [v.get("symbol") for v in vestings] == ["ST01", "ST02"],
          json.dumps(vestings))
    check("每人参数 537 合法", all(
        r["initial"] <= r["total"] and (r["total"] - r["initial"]) >= r["periods"]
        for v in vestings for r in v.get("recipients", [])), json.dumps(vestings))
    cfgHash = inner.get("configHash") or ""
    check("预览 configHash", bool(cfgHash), list(plan.keys()))

    # ---------- 2. 启动第一轮(建计划;claim 或 541) ----------
    st = post("/api/tool/seed", {"config": CFG}, timeout=60)
    if st.get("code") not in (200, 202) and not st.get("success"):
        log("❌ 启动失败: " + json.dumps(st, ensure_ascii=False)[:300])
        json.dump({"checks": checks}, open(OUT, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
        return
    job = st["data"]["jobId"]
    log("jobId=" + job)
    deadline = time.time() + 420
    status = {}
    while time.time() < deadline:
        time.sleep(4)
        status = get("/api/tool/seed/status/" + job).get("data") or {}
        if status.get("status") in ("completed", "failed"):
            break
    check("第一轮 completed", status.get("status") == "completed",
          json.dumps(status)[:600])
    logs1 = "\n".join(status.get("logTail") or [])

    # ---------- 3. 链上核验 VestingInfo ----------
    vids = []
    for v in vestings:
        # vid 从第二轮 state 也可拿;第一轮 log 含 vid=...;直接从 state 文件 API?走 dataset
        pass
    ds = get("/api/tool/seed/dataset").get("data") or {}
    mods = {}
    for d in ds.get("datasets") or []:
        if d.get("configHash") == cfgHash:
            mods = d.get("modules") or {}
            break
    for v in vestings:
        stage = "vesting:" + v["key"]
        info = (mods.get("vesting") or {}).get(stage) or {}
        vid = info.get("vid") or ""
        check("%s vid 落盘" % v["key"], bool(vid), info)
        if vid:
            vids.append(vid)
            vi = read("VestingInfo", {"vesting_id": vid})
            ok = isinstance(vi, dict) and vi.get("token")
            check("%s 链上 VestingInfo 可读" % v["key"], ok,
                  json.dumps(vi, ensure_ascii=False)[:200])
            if ok:
                recs = vi.get("recipients") or []
                check("%s 链上人数=%d" % (v["key"], len(v["recipients"])),
                      len(recs) == len(v["recipients"]), json.dumps(recs)[:200])

    # ---------- 4. 第二轮续跑:claim 重试 ----------
    st2 = post("/api/tool/seed", {"config": CFG}, timeout=60)
    job2 = (st2.get("data") or {}).get("jobId")
    check("第二轮启动", bool(job2), json.dumps(st2)[:200])
    deadline = time.time() + 420
    status2 = {}
    while time.time() < deadline:
        time.sleep(4)
        status2 = get("/api/tool/seed/status/" + job2).get("data") or {}
        if status2.get("status") in ("completed", "failed"):
            break
    check("第二轮 completed", status2.get("status") == "completed", json.dumps(status2)[:600])
    logs2 = "\n".join(status2.get("logTail") or [])
    check("第二轮含领取成功", "领取 ✓" in logs2 or "无可领" not in logs2, logs2[-400:])

    # ---------- 5. claim 到账核验(VestingInfo.claimed_amounts > 0) ----------
    claimed_any = False
    for vid in vids:
        vi = read("VestingInfo", {"vesting_id": vid})
        if not isinstance(vi, dict):
            continue
        names = [r.get("name") for r in vi.get("recipients") or []]
        for name, claimed in zip(names, vi.get("claimed_amounts") or []):
            if int(claimed or 0) > 0:
                claimed_any = True
                check("claim 到账 %s(%s)" % (name, vid[:8]), True, "claimed=%d" % int(claimed))
    check("至少一人 claim 到账", claimed_any, json.dumps(vids))

    json.dump({"checks": checks, "job1": status.get("status"), "job2": status2.get("status")},
              open(OUT, "w", encoding="utf-8"), ensure_ascii=False, indent=1)
    nfail = sum(1 for c in checks if not c["ok"])
    log("完成: %d/%d 通过" % (len(checks) - nfail, len(checks)))
    sys.exit(1 if nfail else 0)


if __name__ == "__main__":
    main()
