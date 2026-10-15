# -*- coding: utf-8 -*-
"""vc 模块:issuer 组织签发证书给用户(vc-flow 幂等编排)+ 生命周期(冻结/撤销)。"""
from . import payloads
from .api import retry

MODULE = "vc"
# 幂等兜底:已冻结 1074 / 凭证不存在或已撤销 1033 / 已撤销再操作
_ALREADY = ("1074", "1033", "AlreadyFrozen", "Revoked")


def run(ctx):
    api, st, plan, log = ctx.api, ctx.state, ctx.plan, ctx.log
    for vi in plan.vc_issuers:
        issuer = ctx.acct(vi.issuer_name)
        for uname in vi.certifies:
            stage = f"flow:{vi.issuer_name}:{uname}"
            if st.is_done(MODULE, stage):
                continue
            user = ctx.acct(uname)
            data = retry(lambda: api.vc_flow(
                payloads.vc_flow_payload(issuer, user, vi)), log=log)
            verifs = data.get("verification") or []
            all_valid = bool(verifs) and all(v.get("valid") for v in verifs)
            log(f"  {vi.issuer_name} → {uname}: {len(verifs)} 张凭证 "
                f"({'全部有效 ✓' if all_valid else '存在无效项!'})")
            st.mark(MODULE, stage, {
                "issuer": vi.issuer_name, "schemas": [v.get("schema") for v in verifs],
                "allValid": all_valid})
            st.save()

    by_issuer = {v.issuer_name: v for v in plan.vc_issuers}
    for op in plan.vc_lifecycle:
        stage = f"lifecycle:{op.kind}:{op.issuer_name}:{op.user}"
        if st.is_done(MODULE, stage):
            continue
        issuer = ctx.acct(op.issuer_name)
        subject = ctx.acct(op.user)
        vi = by_issuer[op.issuer_name]
        if op.kind == "freeze":
            cred_id = str(api.read("identity", "CredentialId",
                                   {"issuer": issuer["address"],
                                    "credential_schema": vi.schema}))
            ok, err = api.try_write(
                "identity", "FreezeVcAttestation",
                {"issuer": issuer["address"], "subject": subject["address"],
                 "credential_id": cred_id}, issuer)
            detail = {"kind": "freeze", "credentialId": cred_id,
                      "attempted": ok, "error": (err or "")[:300]}
        elif op.kind == "revoke":
            ok, err = api.try_write(
                "identity", "RevokeVcAttestation",
                {"issuer": issuer["address"], "subject": subject["address"],
                 "credential_schema": vi.schema}, issuer)
            detail = {"kind": "revoke", "attempted": ok, "error": (err or "")[:300]}
        else:
            raise ValueError(f"未知生命周期操作: {op.kind}")
        if not ok and not any(t in (err or "") for t in _ALREADY):
            raise RuntimeError(f"VC 生命周期 {op.kind} 失败: {err}")
        status = api.read("identity", "VcAttestationLifecycle",
                          {"subject": subject["address"],
                           "credential_schema": vi.schema,
                           "issuer": issuer["address"]})
        cur_status = (status or {}).get("status") if isinstance(status, dict) else status
        log(f"  {op.kind} {op.issuer_name}→{op.user} "
            f"({'新执行 ✓' if ok else '已完成,跳过'}) 链上状态={cur_status}")
        detail["chainStatus"] = cur_status
        st.mark(MODULE, stage, detail)
        st.save()
    log(f"vc 模块完成: {sum(len(v.certifies) for v in plan.vc_issuers)} 人持证 / "
        f"{len(plan.vc_lifecycle)} 条生命周期记录")
