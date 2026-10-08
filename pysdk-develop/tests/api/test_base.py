"""Port of gosdk-develop/api/test/base_test.go."""

import base58 as b58lib
import pytest

from milon_sdk import api
from milon_sdk.postcard import Deserializer, Serializer

HASH = bytes(range(1, 33))


def test_new_tx_hash_from_relaxed():
    assert api.new_tx_hash_from_relaxed(HASH) == HASH
    assert api.new_tx_hash_from_relaxed(HASH.hex()) == HASH
    assert api.new_tx_hash_from_relaxed(b58lib.b58encode(HASH).decode()) == HASH
    with pytest.raises(ValueError):
        api.new_tx_hash_from_relaxed("0102")
    with pytest.raises(ValueError):
        api.new_tx_hash_from_relaxed("a")
    with pytest.raises(ValueError):
        api.new_tx_hash_from_relaxed("!")
    with pytest.raises(ValueError):
        api.new_tx_hash_from_relaxed(42)
    with pytest.raises(ValueError):
        api.new_tx_hash_from_relaxed(None)


def test_tx_hash_string():
    assert api.tx_hash_str(HASH) == b58lib.b58encode(HASH).decode()


def test_deserialize_access_record_inline_persisted_value():
    ser = Serializer()
    ser.serialize_fixed_bytes(b"\x00" * api.RS_HASH_LEN)  # ResourceID
    ser.serialize_bool(False)  # FirstSnapshot: None
    ser.serialize_u32(0)  # LastWritten variant: Inline
    ser.serialize_u64(42)  # type_tag
    ser.serialize_bytes(b"\x01\x02\x03")  # InlineData (Vec<u8>)

    rec = api.deserialize_access_record(Deserializer(ser.bytes()))
    assert rec.FirstSnapshot is None
    assert rec.LastWritten.Variant == 0
    assert rec.LastWritten.TypeTag == 42
    assert rec.LastWritten.InlineData == b"\x01\x02\x03"


def test_deserialize_access_record_external_with_first_snapshot():
    ser = Serializer()
    ser.serialize_fixed_bytes(b"\x00" * api.RS_HASH_LEN)  # ResourceID
    ser.serialize_bool(True)  # FirstSnapshot: Some
    ser.serialize_u32(1)  # FirstSnapshot variant: External
    ser.serialize_fixed_bytes(b"\x00" * api.BLOB_HASH_LEN)
    ser.serialize_u32(1)  # LastWritten variant: External
    ser.serialize_fixed_bytes(b"\x00" * api.BLOB_HASH_LEN)

    rec = api.deserialize_access_record(Deserializer(ser.bytes()))
    assert rec.FirstSnapshot is not None
    assert rec.FirstSnapshot.Variant == 1
    assert rec.LastWritten.Variant == 1
    assert rec.LastWritten.ExternalHash == b"\x00" * api.BLOB_HASH_LEN


def test_serialize_persisted_value_round_trip():
    # inline variant serialized format
    pv = api.PersistedValue(Variant=0, TypeTag=7, InlineData=b"\x09\x08\x07")
    ser = Serializer()
    api.serialize_persisted_value(ser, pv)

    d = Deserializer(ser.bytes())
    assert d.deserialize_u32() == 0
    assert d.deserialize_u64() == 7
    # Inline values are length-prefixed
    assert d.buffer()[d.offset() :] == b"\x03\x09\x08\x07"
    assert d.deserialize_bytes() == b"\x09\x08\x07"

    # external variant round trip
    pv = api.PersistedValue(Variant=1, ExternalHash=b"\x01\x02\x03" + b"\x00" * 29)
    ser = Serializer()
    ser.serialize_fixed_bytes(b"\x00" * api.RS_HASH_LEN)  # ResourceID
    ser.serialize_bool(False)  # FirstSnapshot: None
    api.serialize_persisted_value(ser, pv)

    rec = api.deserialize_access_record(Deserializer(ser.bytes()))
    assert rec.LastWritten == pv

    # inline variant no length prefix
    pv = api.PersistedValue(Variant=0, TypeTag=7, InlineData=b"\x09\x08\x07")
    ser = Serializer()
    api.serialize_persisted_value_no_len(ser, pv)

    d = Deserializer(ser.bytes())
    assert d.deserialize_u32() == 0
    assert d.deserialize_u64() == 7
    # No length prefix: value 直接跟在 type_tag 后
    assert d.buffer()[d.offset() :] == b"\x09\x08\x07"


def test_persisted_value_unknown_variant():
    ser = Serializer()
    ser.serialize_fixed_bytes(b"\x00" * api.RS_HASH_LEN)
    ser.serialize_bool(False)
    ser.serialize_u32(9)  # unknown variant

    with pytest.raises(ValueError, match="unknown PersistedValue variant: 9"):
        api.deserialize_access_record(Deserializer(ser.bytes()))


def _build_no_len_access_record(ser: Serializer):
    ser.serialize_fixed_bytes(b"\x00" * api.RS_HASH_LEN)  # ResourceID
    ser.serialize_bool(False)  # FirstSnapshot: None
    ser.serialize_u32(0)  # variant Inline (no length prefix)
    ser.serialize_u64(42)  # type_tag
    ser.serialize_u32(1)  # value 长度 1（无长度前缀语义下由内容界定）
    ser.serialize_u32(3)  # 额外字节


def test_deserialize_access_record_no_len_inline_persisted_value():
    # 无 resolver：fallback DeserializeBytes（带长度前缀语义）
    ser = Serializer()
    ser.serialize_fixed_bytes(b"\x00" * api.RS_HASH_LEN)
    ser.serialize_bool(False)
    ser.serialize_u32(0)
    ser.serialize_u64(42)
    ser.serialize_bytes(b"\x01\x02\x03")

    rec = api.deserialize_access_record_no_len(Deserializer(ser.bytes()))
    assert rec.LastWritten.Variant == 0
    assert rec.LastWritten.TypeTag == 42
    assert rec.LastWritten.InlineData == b"\x01\x02\x03"


class _FakeResolver:
    """批3 的 provider.IDLTypeResolver 语义桩（对应 Go 测试的 BuiltinTypeFallback）。"""

    def decode_resource(self, type_tag, data):
        return data, b""

    def decode_event(self, type_tag, data):
        return data, b""


def test_deserialize_access_record_no_len_builtin_type_fallback():
    ser = Serializer()
    ser.serialize_fixed_bytes(b"\x00" * api.RS_HASH_LEN)
    ser.serialize_bool(False)
    ser.serialize_u32(0)
    ser.serialize_u64(42)
    ser.serialize_fixed_bytes(b"\x01\x02\x03")  # 无长度前缀 value

    d = Deserializer(ser.bytes())
    d.set_type_resolver(_FakeResolver())
    rec = api.deserialize_access_record_no_len(d)
    assert rec.LastWritten.TypeTag == 42
    assert rec.LastWritten.InlineData == b"\x01\x02\x03"


def test_deserialize_event_entry():
    ser = Serializer()
    ser.serialize_u64(7)  # type_tag
    ser.serialize_bytes(b"\x01\x02")  # body

    entry = api.deserialize_event_entry(Deserializer(ser.bytes()))
    assert entry.TypeTag == 7
    assert entry.Value == b"\x01\x02"


def test_deserialize_event_entry_no_len():
    # 无 resolver：fallback DeserializeBytes
    ser = Serializer()
    ser.serialize_u64(7)
    ser.serialize_bytes(b"\x01\x02")

    entry = api.deserialize_event_entry_no_len(Deserializer(ser.bytes()))
    assert entry.TypeTag == 7
    assert entry.Value == b"\x01\x02"

    # 有 resolver：消费全部剩余字节
    ser = Serializer()
    ser.serialize_u64(7)
    ser.serialize_fixed_bytes(b"\x01\x02")

    d = Deserializer(ser.bytes())
    d.set_type_resolver(_FakeResolver())
    entry = api.deserialize_event_entry_no_len(d)
    assert entry.TypeTag == 7
    assert entry.Value == b"\x01\x02"
    assert d.remaining() == 0
