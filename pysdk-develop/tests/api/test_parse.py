"""Port of gosdk-develop/api/test/{listResourcePath, getResource 等}_test.go 的
JSON 解析用例与 get_tx_history_proof 覆盖。"""

from milon_sdk import api
from milon_sdk.postcard import Deserializer, Serializer


def test_unmarshal_list_resource_path_list_from_raw_list():
    raw = [
        [[1, 2, 3], "a/b"],
        [[4, 5, 6], "c/d"],
    ]
    result = api.unmarshal_list_resource_path_list_from_raw_list(raw)
    assert len(result) == 2
    assert result[0].RsHash[:3] == b"\x01\x02\x03"
    assert result[0].Path == "a/b"
    assert result[1].Path == "c/d"

    import pytest

    with pytest.raises(ValueError):
        api.unmarshal_list_resource_path_list_from_raw_list([[1]])
    with pytest.raises(ValueError):
        api.unmarshal_list_resource_path_list_from_raw_list([["abc", "x"]])
    with pytest.raises(ValueError):
        api.unmarshal_list_resource_path_list_from_raw_list([[[], 123]])


def test_unmarshal_batch_resource_path_list_from_raw_list():
    raw_ok = [[list(range(1, 19)), {"Ok": "res/path"}]]
    raw_err = [[list(range(1, 19)), {"Err": "not found"}]]
    assert api.unmarshal_batch_resource_path_list_from_raw_list(raw_ok)[0].Path == "res/path"
    assert api.unmarshal_batch_resource_path_list_from_raw_list(raw_err)[0].ErrMsg == "not found"

    import pytest

    with pytest.raises(ValueError):
        api.unmarshal_batch_resource_path_list_from_raw_list([[1]])


def test_unmarshal_rs_hash_from_json_array():
    rs = api.unmarshal_rs_hash_from_json_array([1, 2, 3])
    assert rs[:3] == b"\x01\x02\x03"
    assert len(rs) == api.RS_HASH_LEN

    import pytest

    with pytest.raises(ValueError):
        api.unmarshal_rs_hash_from_json_array(list(range(19)))


def test_get_tx_history_proof_roundtrip():
    from milon_sdk.api.block import Block

    p = api.GetTxHistoryProof(
        Block=Block(Number=1, Epoch=2, Slot=3, Hash=bytes(32), PrevHash=bytes(32),
                    StateHash=bytes(32), TxRoot=bytes(32), TxCount=0, Timestamp=0),
        Index=5,
        Siblings=[bytes(range(1, 33)), bytes(range(2, 34))],
        History=b"\x01\x02\x03",
    )
    ser = Serializer()
    p.marshal_postcard(ser)

    d = Deserializer(ser.bytes())
    back = api.GetTxHistoryProof()
    back.unmarshal_postcard(d)
    d.assert_end()
    assert back.Index == 5
    assert back.Siblings == p.Siblings
    assert back.History == b"\x01\x02\x03"
    assert back.Block == p.Block
