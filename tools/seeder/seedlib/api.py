# -*- coding: utf-8 -*-
"""milon-api-server HTTP 客户端。

封装 /api/write(统一签/双签)、/api/read、等待确认、领水、
sft-flow / vc-flow 编排端点;错误统一为 ApiError(status, body)。
"""
import json
import time
import urllib.error
import urllib.request
from dataclasses import dataclass
from typing import Optional


class ApiError(Exception):
    def __init__(self, status: int, body: str):
        self.status = status
        self.body = body or ""
        super().__init__(f"HTTP {status}: {self.body[:300]}")


@dataclass
class WriteResult:
    ok: bool
    tx_hash: Optional[str] = None
    confirmed: Optional[bool] = None
    error: Optional[str] = None


class Api:
    def __init__(self, base_url: str, timeout: int = 300, log=print):
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout
        self.log = log

    # ---------- 基础 ----------

    def _request(self, method: str, path: str, payload=None) -> dict:
        url = self.base_url + path
        data = json.dumps(payload).encode() if payload is not None else None
        req = urllib.request.Request(url, data=data, method=method,
                                     headers={"Content-Type": "application/json"})
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as resp:
                return json.loads(resp.read().decode())
        except urllib.error.HTTPError as e:
            raise ApiError(e.code, e.read().decode(errors="replace")) from None
        except urllib.error.URLError as e:
            raise ApiError(0, f"连接失败 {url}: {e.reason}") from None

    def get(self, path: str) -> dict:
        return self._request("GET", path)

    def post(self, path: str, payload: dict) -> dict:
        return self._request("POST", path, payload)

    # ---------- 系统/派生/余额 ----------

    def health(self) -> dict:
        return self.get("/api/health")

    def derive_public_key(self, private_key: str, curve: str = "secp256k1") -> str:
        r = self.post("/api/util/key/derive-public",
                      {"privateKey": private_key, "keyType": curve})
        return r["data"]["publicKey"]

    def derive_address(self, public_key: str, curve: str = "secp256k1") -> str:
        r = self.post("/api/util/address/derive",
                      {"publicKey": public_key, "keyType": curve})
        return r["data"]["address"]

    def balance(self, address: str) -> int:
        try:
            r = self.get(f"/api/faucet/balance/{address}")
        except ApiError as e:
            # 旧形态:未激活账户时服务端视图失败(HTTP 500 "failed to get balance")
            if "failed to get balance" in e.body:
                return 0
            raise
        data = r.get("data")
        if isinstance(data, dict):
            # 新 SDK 形态:链端 view 错误 {Message,Code,Data} 以 HTTP 200 返回
            # (账户无 MIL 记录 = 0,领水守卫据此走 claim 激活)
            return 0
        return int(data or 0)

    # ---------- 读写 ----------

    def read(self, app: str, method: str, args: dict):
        r = self.post("/api/read", {"appName": app, "methodName": method, "args": args})
        data = r.get("data")
        # 新 SDK:链端 view 错误以 HTTP 200 + data={Message,Code,Data} 返回
        if isinstance(data, dict) and "Code" in data and "Message" in data:
            raise ApiError(200, f"chain view error {data['Code']}: {data['Message']}")
        return data

    def read_u64(self, app: str, method: str, args: dict) -> int:
        """数值视图;链端 512(AccountNotFound,如从未有余额记录)视为 0。"""
        try:
            v = self.read(app, method, args)
        except ApiError as e:
            if "512" in e.body or "AccountNotFound" in e.body:
                return 0
            raise
        return int(v if not isinstance(v, dict) else v.get("value", 0))

    def write(self, app: str, method: str, args: dict, payer: dict,
              ix: Optional[dict] = None, wait: bool = True,
              wait_secs: int = 60) -> WriteResult:
        """写交易。payer=付 gas+签交易账户;ix=指令资源账户(dual_sign)。"""
        payload = {"appName": app, "methodName": method, "args": args,
                   "paymentMode": "unified_dual_sign" if ix else "unified_payer_all",
                   "payerPrivateKey": payer["privateKey"],
                   "payerAddress": payer["address"],
                   "signatureMode": {"type": "pubkey", "publicKey": payer["publicKey"]}}
        if ix:
            payload.update({"ixAddress": ix["address"],
                            "ixPrivateKey": ix["privateKey"],
                            "ixSignatureMode": {"type": "pubkey",
                                                "publicKey": ix["publicKey"]}})
        r = self.post("/api/write", payload)
        ok = bool(r.get("success"))
        txh = (r.get("data") or {}).get("txHash")
        error = None if ok else (r.get("message") or json.dumps(r, ensure_ascii=False)[:300])
        confirmed = None
        if ok and txh and wait:
            confirmed = self.wait_tx(txh, wait_secs)
            if not confirmed:
                error = f"已提交但确认超时: {txh}"
        return WriteResult(ok=ok, tx_hash=txh, confirmed=confirmed, error=error)

    def try_write(self, *a, **kw):
        """预期失败的写:永不抛异常,返回 (ok, error_text)。"""
        try:
            res = self.write(*a, **kw)
            return res.ok, (res.error or "")
        except ApiError as e:
            return False, e.body

    def wait_tx(self, tx_hash: str, timeout_secs: int = 60) -> bool:
        r = self.get(f"/api/transactions/{tx_hash}/wait?timeoutSecs={timeout_secs}")
        return bool(r.get("success"))

    # ---------- 水龙头 / 编排端点 ----------

    def faucet_claim(self, acct: dict) -> dict:
        r = self.post("/api/faucet/claim",
                      {"privateKey": acct["privateKey"], "address": acct["address"],
                       "keyType": acct.get("curve", "secp256k1")})
        return r.get("data") or {}

    def sft_flow(self, payload: dict) -> dict:
        r = self.post("/api/tool/sft-flow", payload)
        return r.get("data") or {}

    def vc_flow(self, payload: dict) -> dict:
        r = self.post("/api/tool/vc-flow", payload)
        return r.get("data") or {}


def retry(fn, attempts: int = 3, delay: float = 10.0, log=print):
    """网络/瞬时错误重试;ApiError 语义失败也重试(链上偶发超时)。"""
    last = None
    for i in range(attempts):
        try:
            return fn()
        except ApiError as e:
            last = e
            log(f"    重试 {i + 1}/{attempts}: {str(e)[:160]}")
            if i < attempts - 1:
                time.sleep(delay)
    raise last
