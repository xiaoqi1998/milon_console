"""回填：Go api/test 的 WithRealProvider 两用例（provider 完成后启用）。

真实 demo/account IDL + IDLTypeResolver 驱动的 SimulateReceipt/TxHistory 往返。
"""

from milon_sdk import api, provider as pv
from milon_sdk.postcard import (
    Deserializer,
    Serializer,
    deserialize_postcard_with_resolver,
    serialize_postcard,
)
from milon_sdk.types.bitbap import Bitmap64

POOL = bytes(range(1, 21))
RECIPIENT = bytes(range(101, 121))

EVENT_CREDIT_APPLIED_TAG = 7407037194950745602
U64_TAG = 5563585020063213298


def _amount_bytes() -> bytes:
    ser = Serializer()
    ser.serialize_u64(42)
    return ser.bytes()


def _event_value() -> bytes:
    return POOL + RECIPIENT + _amount_bytes()


def _demo_resolver() -> pv.IDLTypeResolver:
    idls = pv.load_default_idls()
    demo = pv.new_provider(idls["demo"])
    account = pv.new_provider(idls["account"])
    return pv.IDLTypeResolver(
        providers={"demo": demo, "account": account}
    )


def test_simulate_receipt_with_real_provider_event_credit_applied():
    resolver = _demo_resolver()

    original = api.SimulateReceipt(
        Magic=b"\x00" * 4,
        Version=0,
        TxID=bytes(range(1, 13)),
        TxHash=bytes(range(1, 33)),
        State=1,
        Access=[
            api.AccessRecord(
                ResourceID=bytes(range(1, 19)),
                FirstSnapshot=None,
                LastWritten=api.PersistedValue(
                    Variant=1, ExternalHash=bytes(range(1, 33))
                ),
            ),
            api.AccessRecord(
                ResourceID=bytes(range(2, 20)),
                LastWritten=api.PersistedValue(
                    Variant=0, TypeTag=U64_TAG, InlineData=_amount_bytes()
                ),
            ),
        ],
        Events=[api.TypeTagWithData(TypeTag=EVENT_CREDIT_APPLIED_TAG, Value=_event_value())],
        Error=None,
        GasCharged=22,
    )

    data = serialize_postcard(original)

    def _read(d: Deserializer):
        r = api.SimulateReceipt()
        r.unmarshal_postcard(d)
        return r

    deserialized = deserialize_postcard_with_resolver(data, _read, False, resolver)

    assert deserialized.TxID == original.TxID
    assert deserialized.TxHash == original.TxHash
    assert deserialized.State == original.State
    assert deserialized.Access == original.Access
    assert deserialized.Events == original.Events
    assert deserialized.Error == original.Error


def test_tx_history_with_real_provider_event_credit_applied():
    resolver = _demo_resolver()

    from milon_sdk.crypto import new_address_from_bytes

    signer = new_address_from_bytes(bytes(range(1, 21)))

    original = api.TxHistory(
        Stamp=100,
        Payer=0,
        Signatures=[
            api.TxHistorySignature(
                Signer=signer, AuthBit=Bitmap64(1), SigBit=Bitmap64(2)
            )
        ],
        Instructions=[],
        Receipt=api.TxReceipt(
            TxID=bytes(range(1, 13)),
            TxHash=bytes(range(1, 33)),
            State=api.TX_STATE_SUCCESS,
            Access=[],
            Events=[
                api.TypeTagWithData(
                    TypeTag=EVENT_CREDIT_APPLIED_TAG, Value=_event_value()
                )
            ],
            Error=None,
            GasCharged=42,
        ),
    )

    ser = Serializer()
    original.marshal_postcard(ser)

    d = Deserializer(ser.bytes())
    d.set_type_resolver(resolver)
    back = api.TxHistory()
    back.unmarshal_postcard(d)
    d.assert_end()

    assert back.Stamp == original.Stamp
    assert back.Payer == original.Payer
    assert back.Receipt.Events == original.Receipt.Events
    assert back.Receipt.GasCharged == 42
