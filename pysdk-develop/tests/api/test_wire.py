"""Port of gosdk-develop/api/test/{block,chainHead,getResource,getAccessValue,
eventsByTxHash,accountView,simulateReceipt,txHistory}_test.go 的 postcard 用例。

WithRealProvider 用例（TestSimulateReceipt_WithRealProvider /
TestTxHistory_WithRealProvider）依赖 provider 包，留批3 回填。
"""

import pytest

from milon_sdk import api
from milon_sdk.crypto import Address
from milon_sdk.postcard import Deserializer, Serializer


def _roundtrip(obj, check):
    ser = Serializer()
    obj.marshal_postcard(ser)
    d = Deserializer(ser.bytes())
    check(obj)
    return ser, d


def _mk_block():
    return api.Block(
        Number=11,
        Epoch=22,
        Slot=33,
        Hash=bytes(range(1, 33)),
        PrevHash=bytes(range(2, 34)),
        StateHash=bytes(range(3, 35)),
        TxRoot=bytes(range(4, 36)),
        TxCount=7,
        Timestamp=1700000000,
    )


def test_block_marshal_postcard():
    b = _mk_block()
    ser = Serializer()
    b.marshal_postcard(ser)

    d = Deserializer(ser.bytes())
    back = api.Block()
    back.unmarshal_postcard(d)
    d.assert_end()
    assert back == b


def test_block_deserialize_errors():
    b = api.Block()
    with pytest.raises(ValueError, match="failed to deserialize Number"):
        b.unmarshal_postcard(Deserializer(b""))


def test_chain_head_marshal_postcard():
    ch = api.ChainHead(
        ChainId=900000001,
        BlockHeight=123,
        BlockHash=bytes(range(5, 37)),
        TimestampMsecs=1700000000123,
    )
    ser = Serializer()
    ch.marshal_postcard(ser)

    d = Deserializer(ser.bytes())
    back = api.ChainHead()
    back.unmarshal_postcard(d)
    d.assert_end()
    assert back == ch


def test_chain_head_deserialize_errors():
    ch = api.ChainHead()
    with pytest.raises(ValueError, match="failed to deserialize ChainId"):
        ch.unmarshal_postcard(Deserializer(b""))


def test_get_resource_marshal_postcard():
    gr = api.GetResource(
        Data=api.TypeTagWithData(TypeTag=99, Value=b"\x0a\x0b")
    )
    ser = Serializer()
    gr.marshal_postcard(ser)

    d = Deserializer(ser.bytes())
    back = api.GetResource()
    back.unmarshal_postcard(d)
    d.assert_end()
    assert back.Data.TypeTag == 99
    assert back.Data.Value == b"\x0a\x0b"


def test_get_resource_deserialize_errors():
    gr = api.GetResource()
    with pytest.raises(ValueError, match="failed to deserialize TypeTag"):
        gr.unmarshal_postcard(Deserializer(b""))


def test_get_access_value_info_marshal_postcard():
    av = api.GetAccessValueInfo(
        BlobHash=bytes(range(1, 33)),
        Data=api.TypeTagWithData(TypeTag=55, Value=b"\x01"),
    )
    ser = Serializer()
    av.marshal_postcard(ser)

    d = Deserializer(ser.bytes())
    back = api.GetAccessValueInfo()
    back.unmarshal_postcard(d)
    d.assert_end()
    assert back.BlobHash == av.BlobHash
    assert back.Data.TypeTag == 55
    assert back.Data.Value == b"\x01"

    # Data 为 None
    av2 = api.GetAccessValueInfo(BlobHash=bytes(32), Data=None)
    ser = Serializer()
    av2.marshal_postcard(ser)
    back2 = api.GetAccessValueInfo()
    back2.unmarshal_postcard(Deserializer(ser.bytes()))
    assert back2.Data is None


def test_get_access_value_info_deserialize_errors():
    av = api.GetAccessValueInfo()
    with pytest.raises(ValueError, match="failed to deserialize BlobHash"):
        av.unmarshal_postcard(Deserializer(b""))


def test_events_by_tx_hash_request_marshal_postcard():
    req = api.EventsByTxHashReq(
        TxHash=bytes(range(1, 33)), TypeTagFilter=7
    )
    ser = Serializer()
    req.marshal_postcard(ser)

    d = Deserializer(ser.bytes())
    back = api.EventsByTxHashReq()
    back.unmarshal_postcard(d)
    d.assert_end()
    assert back.TxHash == req.TxHash
    assert back.TypeTagFilter == 7

    # TypeTagFilter None
    req2 = api.EventsByTxHashReq(TxHash=bytes(32), TypeTagFilter=None)
    ser = Serializer()
    req2.marshal_postcard(ser)
    back2 = api.EventsByTxHashReq()
    back2.unmarshal_postcard(Deserializer(ser.bytes()))
    assert back2.TypeTagFilter is None


def test_event_entry_marshal_postcard():
    e = api.EventEntry(
        BlockHeight=9,
        TxHash=bytes(range(3, 35)),
        TxIndex=4,
        EventIndex=5,
        Data=api.TypeTagWithData(TypeTag=77, Value=b"\xde\xad"),
    )
    ser = Serializer()
    e.marshal_postcard(ser)

    d = Deserializer(ser.bytes())
    back = api.EventEntry()
    back.unmarshal_postcard(d)
    d.assert_end()
    assert back == e


def test_events_by_tx_hash_marshal_postcard():
    evts = api.EventsByTxHash(
        Events=[
            api.EventEntry(
                BlockHeight=1,
                TxHash=bytes(range(1, 33)),
                TxIndex=0,
                EventIndex=0,
                Data=api.TypeTagWithData(TypeTag=1, Value=b"\x01"),
            ),
            api.EventEntry(
                BlockHeight=2,
                TxHash=bytes(range(2, 34)),
                TxIndex=1,
                EventIndex=1,
                Data=api.TypeTagWithData(TypeTag=2, Value=b""),
            ),
        ]
    )
    ser = Serializer()
    evts.marshal_postcard(ser)

    d = Deserializer(ser.bytes())
    back = api.EventsByTxHash()
    back.unmarshal_postcard(d)
    d.assert_end()
    assert back.Events == evts.Events


def test_events_by_tx_hash_response_deserialize_errors():
    r = api.EventsByTxHash()
    with pytest.raises(ValueError, match="failed to deserialize Events"):
        r.unmarshal_postcard(Deserializer(b"\x01"))


def test_account_view_marshal_postcard():
    addr = Address(bytes(range(1, 21)))
    ac = api.AccountView(
        Address=addr,
        Threshold=2,
        PublicKeysBs58=["keyA", "keyB"],
    )
    ser = Serializer()
    ac.marshal_postcard(ser)

    d = Deserializer(ser.bytes())
    back = api.AccountView()
    back.unmarshal_postcard(d)
    d.assert_end()
    assert back.Address == addr
    assert back.Threshold == 2
    assert back.PublicKeysBs58 == ["keyA", "keyB"]


def test_account_view_deserialize_errors():
    ac = api.AccountView()
    with pytest.raises(ValueError, match="failed to deserialize Address"):
        ac.unmarshal_postcard(Deserializer(b""))


def _mk_simulate_receipt():
    return api.SimulateReceipt(
        Magic=b"MSIM",
        Version=1,
        TxID=bytes(range(1, 13)),
        TxHash=bytes(range(1, 33)),
        State=1,
        Access=[
            api.AccessRecord(
                ResourceID=b"\x0a" * api.RS_HASH_LEN,
                FirstSnapshot=None,
                LastWritten=api.PersistedValue(
                    Variant=0, TypeTag=42, InlineData=b"\x01\x02\x03"
                ),
            )
        ],
        Events=[api.TypeTagWithData(TypeTag=7, Value=b"\x0e\x0f")],
        Error=None,
        GasCharged=999,
    )


def test_simulate_receipt_with_error():
    r = api.SimulateReceipt(
        Magic=b"MSIM",
        Version=1,
        TxID=bytes(12),
        TxHash=bytes(32),
        State=2,
        Access=[],
        Events=[],
        Error=api.TxFailurePayload(Code=5, Message="boom", Data=b"\x01"),
        GasCharged=1,
    )
    ser = Serializer()
    r.marshal_postcard(ser)

    d = Deserializer(ser.bytes())
    back = api.SimulateReceipt()
    back.unmarshal_postcard(d)
    d.assert_end()
    assert back.Error.Code == 5
    assert back.Error.Message == "boom"
    assert back.Error.Data == b"\x01"


def test_simulate_receipt_deserialize_errors():
    r = api.SimulateReceipt()
    with pytest.raises(ValueError, match="failed to deserialize Magic"):
        r.unmarshal_postcard(Deserializer(b""))


def _mk_tx_history():
    return api.TxHistory(
        Stamp=111,
        Payer=3,
        Signatures=[
            api.TxHistorySignature(
                Signer=Address(bytes(range(1, 21))),
                AuthBit=__import__("milon_sdk.types.bitbap", fromlist=["Bitmap64"]).Bitmap64(0b11),
                SigBit=__import__("milon_sdk.types.bitbap", fromlist=["Bitmap64"]).Bitmap64(0b01),
            )
        ],
        Instructions=[b"\x01\x02", b"\x03"],
        Receipt=api.TxReceipt(
            TxID=bytes(range(1, 13)),
            TxHash=bytes(range(1, 33)),
            State=api.TX_STATE_SUCCESS,
            Access=[
                api.AccessRecord(
                    ResourceID=b"\x0b" * api.RS_HASH_LEN,
                    FirstSnapshot=api.PersistedValue(
                        Variant=1, ExternalHash=bytes(32)
                    ),
                    LastWritten=api.PersistedValue(
                        Variant=1, ExternalHash=bytes(range(1, 33))
                    ),
                )
            ],
            Events=[],  # no-len Events 读取需精确 resolver，Inline 路径在 test_base 覆盖
            Error=None,
            GasCharged=42,
        ),
    )


def test_tx_history_with_error():
    h = _mk_tx_history()
    h.Receipt.Error = 7
    ser = Serializer()
    h.marshal_postcard(ser)

    d = Deserializer(ser.bytes())
    back = api.TxHistory()
    back.unmarshal_postcard(d)
    d.assert_end()
    assert back == h
    assert back.Receipt.Error == 7


def test_tx_history_deserialize_errors():
    h = api.TxHistory()
    with pytest.raises(ValueError, match="failed to deserialize Stamp"):
        h.unmarshal_postcard(Deserializer(b""))
