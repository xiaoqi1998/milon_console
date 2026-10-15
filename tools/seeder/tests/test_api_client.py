# -*- coding: utf-8 -*-
"""API 客户端:请求形态与响应解包(本地桩 server,不碰真链)。"""
import http.server
import json
import threading

import pytest

from seedlib.api import Api, ApiError


class StubHandler(http.server.BaseHTTPRequestHandler):
    routes = {}   # (method, path-without-query) -> (status, obj)
    seen = []     # (method, path, body|None)

    def log_message(self, *a):
        pass

    def _handle(self, method):
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length else b""
        body = json.loads(raw) if raw else None
        path = self.path.split("?")[0]
        StubHandler.seen.append((method, path, body))
        hit = StubHandler.routes.get((method, path))
        if hit is None:
            status, obj = 404, {"success": False, "message": "no route"}
        else:
            status, obj = hit
        data = json.dumps(obj).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        self._handle("GET")

    def do_POST(self):
        self._handle("POST")


@pytest.fixture()
def stub():
    StubHandler.routes = {}
    StubHandler.seen = []
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), StubHandler)
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    yield f"http://127.0.0.1:{server.server_address[1]}"
    server.shutdown()


def _acct(name="alice"):
    return {"name": name, "address": f"A_{name}", "privateKey": f"K_{name}",
            "publicKey": f"P_{name}"}


def test_health(stub):
    StubHandler.routes[("GET", "/api/health")] = (200, {"success": True, "data": {"chainId": 900000001}})
    assert Api(stub).health()["data"]["chainId"] == 900000001


def test_post_error_raises_api_error(stub):
    StubHandler.routes[("POST", "/api/x")] = (400, {"success": False, "message": "boom"})
    with pytest.raises(ApiError) as ei:
        Api(stub).post("/api/x", {})
    assert ei.value.status == 400
    assert "boom" in ei.value.body


def test_derive_public_key_passes_keytype(stub):
    StubHandler.routes[("POST", "/api/util/key/derive-public")] = (
        200, {"success": True, "data": {"publicKey": "PUB1"}})
    out = Api(stub).derive_public_key("aa", "ed25519")
    assert out == "PUB1"
    method, path, body = StubHandler.seen[-1]
    assert (method, path) == ("POST", "/api/util/key/derive-public")
    assert body == {"privateKey": "aa", "keyType": "ed25519"}


def test_derive_address(stub):
    StubHandler.routes[("POST", "/api/util/address/derive")] = (
        200, {"success": True, "data": {"address": "ADDR1"}})
    assert Api(stub).derive_address("PUB1", "secp256k1") == "ADDR1"


def test_read_unwraps_data(stub):
    StubHandler.routes[("POST", "/api/read")] = (200, {"success": True, "data": {"x": 1}})
    assert Api(stub).read("token", "Metadata", {"token": "T"}) == {"x": 1}


def test_read_chain_view_error_raises(stub):
    # 新 SDK:链端 view 错误以 HTTP 200 + data={Message,Code,Data} 返回
    StubHandler.routes[("POST", "/api/read")] = (
        200, {"success": True,
              "data": {"Message": "账户不存在", "Code": 512, "Data": "abc"}})
    with pytest.raises(ApiError) as ei:
        Api(stub).read("token", "BalanceOf", {})
    assert "512" in ei.value.body


def test_read_u64_success_and_missing_record(stub):
    api = Api(stub)
    StubHandler.routes[("POST", "/api/read")] = (200, {"success": True, "data": 50})
    assert api.read_u64("token", "BalanceOf", {}) == 50
    StubHandler.routes[("POST", "/api/read")] = (
        200, {"success": True,
              "data": {"Message": "账户不存在 (token: x, account: y)", "Code": 512,
                       "Data": "abc"}})
    assert api.read_u64("token", "FrozenOf", {}) == 0   # 无记录 = 0,不炸
    StubHandler.routes[("POST", "/api/read")] = (
        200, {"success": True,
              "data": {"Message": "TokenNotFound", "Code": 515, "Data": "abc"}})
    with pytest.raises(ApiError):
        api.read_u64("token", "BalanceOf", {})          # 其他链端错误照样抛


def test_write_success_and_wait(stub):
    StubHandler.routes[("POST", "/api/write")] = (
        200, {"success": True, "data": {"txHash": "0xabc"}})
    StubHandler.routes[("GET", "/api/transactions/0xabc/wait")] = (
        200, {"success": True, "data": {"confirmed": True}})
    api = Api(stub)
    res = api.write("token", "Mint", {"token": "T"}, _acct())
    assert res.ok and res.tx_hash == "0xabc" and res.confirmed is True
    method, path, body = StubHandler.seen[0]
    assert (method, path) == ("POST", "/api/write")
    assert body["appName"] == "token" and body["methodName"] == "Mint"
    assert body["paymentMode"] == "unified_payer_all"
    assert body["payerPrivateKey"] == "K_alice"
    assert body["signatureMode"] == {"type": "pubkey", "publicKey": "P_alice"}
    wait_methods = [m for m, p, _ in StubHandler.seen if "wait" in p]
    assert wait_methods == ["GET"]


def test_write_dual_sign_payload(stub):
    StubHandler.routes[("POST", "/api/write")] = (200, {"success": True, "data": {"txHash": "0x1"}})
    StubHandler.routes[("GET", "/api/transactions/0x1/wait")] = (200, {"success": True})
    api = Api(stub)
    api.write("token", "Create", {"token": "T"}, _acct("owner"), ix=_acct("tokres"))
    body = StubHandler.seen[0][2]
    assert body["paymentMode"] == "unified_dual_sign"
    assert body["ixAddress"] == "A_tokres"
    assert body["ixPrivateKey"] == "K_tokres"
    assert body["ixSignatureMode"] == {"type": "pubkey", "publicKey": "P_tokres"}


def test_write_chain_reject_returns_not_ok(stub):
    StubHandler.routes[("POST", "/api/write")] = (200, {"success": False, "message": "链端拒绝"})
    res = Api(stub).write("sftoken", "Transfer", {}, _acct())
    assert res.ok is False and "链端拒绝" in res.error


def test_try_write_http_error_captured(stub):
    StubHandler.routes[("POST", "/api/write")] = (400, {"success": False, "message": "521 VcRequired"})
    ok, err = Api(stub).try_write("token", "Transfer", {}, _acct())
    assert ok is False and "521" in err


def test_faucet_claim_payload(stub):
    StubHandler.routes[("POST", "/api/faucet/claim")] = (
        200, {"success": True, "data": {"claimed": True, "txHash": "0xf"}})
    data = Api(stub).faucet_claim(_acct())
    assert data["claimed"] is True
    body = StubHandler.seen[-1][2]
    assert body == {"privateKey": "K_alice", "address": "A_alice", "keyType": "secp256k1"}


def test_faucet_claim_ed25519_passes_keytype(stub):
    # 回归锚点:server 自动派生缺省 secp256k1,ed25519 账户不传 keyType 会
    # 报 "public key does not match owner address"
    StubHandler.routes[("POST", "/api/faucet/claim")] = (
        200, {"success": True, "data": {"claimed": True}})
    acct = _acct("kycorg")
    acct["curve"] = "ed25519"
    Api(stub).faucet_claim(acct)
    assert StubHandler.seen[-1][2]["keyType"] == "ed25519"


def test_balance_coerces_int(stub):
    StubHandler.routes[("GET", "/api/faucet/balance/A_1")] = (200, {"success": True, "data": "999"})
    assert Api(stub).balance("A_1") == 999


def test_balance_unactivated_account_is_zero(stub):
    # 创世后账户未激活:服务端视图失败 500 → 视为 0(领水守卫据此走 claim)
    StubHandler.routes[("GET", "/api/faucet/balance/A_new")] = (
        500, {"success": False,
              "message": "failed to get balance: failed to view BalanceOf: invalid wire"})
    assert Api(stub).balance("A_new") == 0


def test_balance_chain_error_dict_is_zero(stub):
    # 新 SDK 形态:无 MIL 记录的账户返回 HTTP 200 + data={Message,Code,Data}
    StubHandler.routes[("GET", "/api/faucet/balance/A_empty")] = (
        200, {"success": True,
              "data": {"Message": "账户不存在 (token: MIL, account: A_empty)",
                       "Code": 512, "Data": "abc"}})
    assert Api(stub).balance("A_empty") == 0


def test_balance_other_500_still_raises(stub):
    StubHandler.routes[("GET", "/api/faucet/balance/A_bad")] = (
        500, {"success": False, "message": "chain exploded"})
    with pytest.raises(ApiError):
        Api(stub).balance("A_bad")


def test_sft_flow_unwrap_and_error(stub):
    StubHandler.routes[("POST", "/api/tool/sft-flow")] = (
        200, {"success": True, "data": {"slot": {"slotId": 2}}})
    assert Api(stub).sft_flow({"a": 1})["slot"]["slotId"] == 2
    StubHandler.routes[("POST", "/api/tool/sft-flow")] = (
        500, {"success": False, "message": "distributions[0]: 1282"})
    with pytest.raises(ApiError) as ei:
        Api(stub).sft_flow({"a": 1})
    assert "1282" in ei.value.body


def test_vc_flow_unwrap(stub):
    StubHandler.routes[("POST", "/api/tool/vc-flow")] = (
        200, {"success": True, "data": {"verification": [{"valid": True}]}})
    assert Api(stub).vc_flow({})["verification"][0]["valid"] is True


def test_wait_tx_timeout_reported(stub):
    StubHandler.routes[("GET", "/api/transactions/0xz/wait")] = (
        200, {"success": False, "message": "timeout"})
    ok = Api(stub).wait_tx("0xz", timeout_secs=5)
    assert ok is False
