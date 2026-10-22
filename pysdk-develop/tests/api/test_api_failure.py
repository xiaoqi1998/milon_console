"""ApiFailure（对照 gosdk-develop/api/apiFailure.go）。

RPC 响应信封的 error 字段 + batch 响应体每个 Err 项的载荷；取代旧的
lib.RpcResponseError 与 view Err 的 TxFailurePayload 布局。
"""

import pytest

from milon_sdk.api import ApiFailure
from milon_sdk.postcard import Deserializer, Serializer


def test_api_failure_postcard_round_trip():
    original = ApiFailure(Message="boom", Code=285, Data=b"\xaa\xbb")
    ser = Serializer()
    original.marshal_postcard(ser)

    back = ApiFailure()
    d = Deserializer(ser.bytes())
    back.unmarshal_postcard(d)
    d.assert_end()

    assert back == original


def test_api_failure_wire_layout_matches_postcard_option():
    # message String(len+bytes) | code Option<u16>(flag+len) | data Option<Vec<u8>>
    ser = Serializer()
    ApiFailure(Message="ok", Code=5, Data=b"\xaa\xbb").marshal_postcard(ser)
    assert ser.bytes() == bytes(
        [2, ord("o"), ord("k"), 1, 5, 1, 2, 0xAA, 0xBB]
    )

    ser = Serializer()
    ApiFailure().marshal_postcard(ser)
    assert ser.bytes() == bytes([0, 0, 0])

    ser = Serializer()
    ApiFailure(Message="e").marshal_postcard(ser)
    assert ser.bytes() == bytes([1, ord("e"), 0, 0])


def test_api_failure_deserialize_missing_option_flags():
    back = ApiFailure()
    back.unmarshal_postcard(Deserializer(bytes([2, ord("o"), ord("k"), 0, 0])))
    assert back.Message == "ok"
    assert back.Code is None
    assert back.Data is None


def test_api_failure_truncated_message_raises():
    with pytest.raises(ValueError, match="failed to deserialize Message"):
        ApiFailure().unmarshal_postcard(Deserializer(b""))


def test_api_failure_code_over_u16_raises():
    ser = Serializer()
    with pytest.raises(ValueError, match="failed to serialize Code"):
        ApiFailure(Message="m", Code=70000).marshal_postcard(ser)


def test_api_failure_str_format():
    assert (
        str(ApiFailure(Message="boom", Code=285, Data=b"\xaa\xbb"))
        == 'message="boom" code=285 data=aabb'
    )
    assert str(ApiFailure(Message="boom")) == 'message="boom"'
