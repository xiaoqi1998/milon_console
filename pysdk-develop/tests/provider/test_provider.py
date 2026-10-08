"""Port of gosdk-develop/provider/{provider,registry,idlTypeResolver}_test.go
的核心用例（内联 fixture + 真实 IDL 集成）。"""

import pytest

from milon_sdk import provider as pv
from milon_sdk.crypto import Address, new_address_from_bytes
from milon_sdk.postcard import Serializer


def _inline_idl() -> pv.IDL:
    return pv.idl_from_json_dict(
        {
            "metadata": {"app_id": 7, "name": "tapp", "description": "test"},
            "instructions": [
                {
                    "name": "Noop",
                    "handler": "noop",
                    "kind": "entry",
                    "discriminator": 11,
                    "args": [],
                },
                {
                    "name": "Mint",
                    "handler": "mint",
                    "kind": "entry",
                    "discriminator": 22,
                    "args": [
                        {"name": "to", "role": "input", "type": "Address"},
                        {"name": "amount", "role": "input", "type": "u64"},
                    ],
                },
                {
                    "name": "BalanceOf",
                    "handler": "balance_of",
                    "kind": "view",
                    "discriminator": 33,
                    "args": [
                        {"name": "who", "role": "input", "type": "Address"}
                    ],
                    "returns": {"type": "u64"},
                },
            ],
            "types": [
                {
                    "name": "Meta",
                    "kind": "struct",
                    "typeTag": 1001,
                    "fields": [
                        {"name": "name", "type": "String"},
                        {"name": "supply", "type": "u64"},
                    ],
                },
                {
                    "name": "Mode",
                    "kind": "enum",
                    "typeTag": 1002,
                    "variants": [
                        {"name": "Off", "kind": "unit", "fields": []},
                        {
                            "name": "Set",
                            "kind": "struct",
                            "fields": [{"name": "v", "type": "u8"}],
                        },
                    ],
                },
            ],
            "resources": [
                {"name": "_Supply", "type": "u64", "typeTag": 2001},
                {"name": "_Meta", "type": "Meta", "typeTag": 2002},
            ],
            "events": [
                {
                    "name": "Transfer",
                    "typeTag": 3001,
                    "fields": [
                        {"name": "from", "type": "Address"},
                        {"name": "value", "type": "u64"},
                    ],
                }
            ],
        }
    )


def test_provider_encode_and_decode():
    pd = pv.new_provider(_inline_idl())
    addr = new_address_from_bytes(bytes(range(1, 21)))

    body = pd.encode("Mint", {"to": addr, "amount": 5000})
    assert body[0] == 7
    assert body[1] == 22
    assert body[2] == 0

    args = pd.decode("Mint", body)
    assert isinstance(args["to"], Address)
    assert args["to"].Bytes == addr.Bytes
    assert args["amount"] == 5000


def test_provider_encode_missing_argument():
    pd = pv.new_provider(_inline_idl())
    with pytest.raises(ValueError, match="missing IDL argument"):
        pd.encode("Mint", {"to": new_address_from_bytes(bytes(20))})


def test_provider_decode_errors():
    pd = pv.new_provider(_inline_idl())
    with pytest.raises(ValueError, match="empty body"):
        pd.decode("Mint", b"\x01\x02")
    with pytest.raises(ValueError, match="app_id mismatch"):
        pd.decode("Mint", b"\x99\x16\x00" + b"\x00" * 21)
    with pytest.raises(ValueError, match="discriminator mismatch"):
        pd.decode("Mint", b"\x07\x99\x99" + b"\x00" * 21)
    with pytest.raises(ValueError, match="trailing bytes"):
        pd.encode("Noop", {}) and pd.decode("Noop", b"\x07\x0b\x00\xff")


def test_serialize_value_complex_types():
    pd = pv.new_provider(_inline_idl())
    ser = Serializer()
    pd._serialize_value(ser, "vec<u64>", [1, 2, 3])
    pd._serialize_value(ser, "option<u64>", None)
    pd._serialize_value(ser, "option<u64>", 9)
    pd._serialize_value(ser, "map<String,u64>", {"a": 1})
    pd._serialize_value(ser, "tuple<u8,u16>", [5, 6])
    pd._serialize_value(ser, "Meta", {"name": "x", "supply": 3})
    pd._serialize_value(ser, "Mode", "Off")
    pd._serialize_value(ser, "Mode", {"variant": "Set", "value": {"v": 3}})

    body = ser.bytes()
    vals = []
    offset = 0
    for t in (
        "vec<u64>",
        "option<u64>",
        "option<u64>",
        "map<String,u64>",
        "tuple<u8,u16>",
        "Meta",
        "Mode",
        "Mode",
    ):
        v, offset = pd.deserialize_value(t, body, offset)
        vals.append(v)
    assert offset == len(body)
    assert vals[0] == [1, 2, 3]
    assert vals[1] is None
    assert vals[2] == 9
    assert vals[3] == {"a": 1}
    assert vals[4] == [5, 6]
    assert vals[5] == {"name": "x", "supply": 3}
    assert vals[6] == {"variant": "Off", "index": 0}
    assert vals[7] == {"variant": "Set", "index": 1, "v": 3}


def test_serialize_value_address_and_public_key():
    from milon_sdk.crypto import new_classical_secret_key, new_signature_from_bytes

    pd = pv.new_provider(_inline_idl())
    sk = new_classical_secret_key()
    pk = sk.ed25519_public()
    sig = sk.sign_ed25519(b"m")

    ser = Serializer()
    pd._serialize_value(ser, "Address", new_address_from_bytes(bytes(range(1, 21))))
    pd._serialize_value(ser, "Address", bytes(range(1, 21)))
    pd._serialize_value(ser, "PublicKey", pk)
    pd._serialize_value(ser, "Signature", sig)

    body = ser.bytes()
    v, offset = pd.deserialize_value("Address", body, 0)
    assert v.Bytes == bytes(range(1, 21))
    v2, offset = pd.deserialize_value("Address", body, offset)
    assert v2.Bytes == bytes(range(1, 21))
    v3, offset = pd.deserialize_value("PublicKey", body, offset)
    assert v3.Bytes == pk.Bytes
    v4, offset = pd.deserialize_value("Signature", body, offset)
    assert v4.Bytes == sig.Bytes
    assert offset == len(body)


def test_deserialize_value_integer_and_fixed_bytes():
    pd = pv.new_provider(_inline_idl())
    ser = Serializer()
    ser.serialize_i8(-2)
    ser.serialize_u128(12345678901234567890123456789)
    ser.serialize_fixed_bytes(b"\x01" * 32)
    ser.serialize_bytes(b"abc")

    body = ser.bytes()
    v, offset = pd.deserialize_value("i8", body, 0)
    assert v == -2
    v, offset = pd.deserialize_value("u128", body, offset)
    assert v == 12345678901234567890123456789
    v, offset = pd.deserialize_value("B256", body, offset)
    assert v == b"\x01" * 32
    v, offset = pd.deserialize_value("bytes", body, offset)
    assert v == b"abc"
    assert offset == len(body)


def test_decode_view_datas_result_branches():
    pd = pv.new_provider(_inline_idl())

    # 1 个 Ok(u64=77)
    ser = Serializer()
    ser.serialize_u32(1)  # vec len
    ser.serialize_enum_variant(0)  # Ok
    ser.serialize_bytes(  # Ok payload = u64(77) 序列化
        b"\x4d"
    )
    values = pd.decode_view_datas("BalanceOf", ser.bytes())
    assert values[0].Value == 77

    # Err 分支
    ser = Serializer()
    ser.serialize_u32(1)
    ser.serialize_enum_variant(1)  # Err
    ser.serialize_u16(5)  # code
    ser.serialize_str("boom")  # message
    ser.serialize_bytes(b"\x01")  # data
    values = pd.decode_view_datas("BalanceOf", ser.bytes())
    assert values[0].Value.Code == 5
    assert values[0].Value.Message == "boom"

    # 非 view 报错
    with pytest.raises(ValueError, match="is not a view instruction"):
        pd.decode_view_datas("Mint", b"\x01\x00")


def test_decode_view_varint_errors():
    with pytest.raises(ValueError, match="unexpected end of input"):
        pv.provider._decode_view_var_uint(b"\x80", 0)
    with pytest.raises(ValueError, match="varint is too long"):
        pv.provider._decode_view_var_uint(b"\x80" * 10, 0)


def test_idl_type_resolver_decode_resource():
    idl = _inline_idl()
    resolver = pv.IDLTypeResolver({"tapp": pv.new_provider(idl)})

    # resource 声明优先：_Supply(u64)
    value_bytes, rest = resolver.decode_resource(2001, b"\x2a\x99\x99")
    assert value_bytes == b"\x2a"
    assert rest == b"\x99\x99"

    # 未声明 resource → types fallback（Meta）
    ser = Serializer()
    ser.serialize_str("n")
    ser.serialize_u64(4)
    value_bytes, rest = resolver.decode_resource(1001, ser.bytes())
    assert rest == b""

    with pytest.raises(ValueError, match="unknown resource type_tag 999"):
        resolver.decode_resource(999, b"")


def test_idl_type_resolver_multiple_providers_collision_first_wins():
    a = _inline_idl()
    b_idl = _inline_idl()
    b_idl.Metadata.Name = "bapp"
    b_idl.Metadata.AppID = 8
    b_idl.Events[0].TypeTag = 3002
    resolver = pv.IDLTypeResolver(
        {"aapp": pv.new_provider(a), "bapp": pv.new_provider(b_idl)}
    )
    # 共享 builtin tag（u64=2001 资源）first wins：aapp 排序在前
    value_bytes, _ = resolver.decode_resource(2001, b"\x2a")
    assert value_bytes == b"\x2a"
    # event 3002 归 bapp
    _, rest = resolver.decode_event(3002, b"\x00" * 20 + b"\x05")
    assert rest == b""


def test_idl_type_resolver_decode_event():
    idl = _inline_idl()
    resolver = pv.IDLTypeResolver({"tapp": pv.new_provider(idl)})
    ser = Serializer()
    ser.serialize_fixed_bytes(bytes(range(1, 21)))  # from Address
    ser.serialize_u64(9)  # value
    event_bytes, rest = resolver.decode_event(3001, ser.bytes())
    assert rest == b""
    assert len(event_bytes) == 21

    with pytest.raises(ValueError, match="unknown event type_tag 999"):
        resolver.decode_event(999, b"")


def _mk_registry() -> pv.IDLRegistry:
    return pv.new_idl_registry({"tapp": pv.new_provider(_inline_idl())})


def test_new_idl_registry_duplicates():
    a = _inline_idl()
    b_idl = _inline_idl()
    b_idl.Metadata.Name = "tapp2"
    b_idl.Metadata.AppID = 7  # 与 a 重复
    b_idl.Events[0].TypeTag = 3002
    with pytest.raises(ValueError, match="duplicate app_id: 7"):
        pv.new_idl_registry({"a": pv.new_provider(a), "b": pv.new_provider(b_idl)})


def test_idl_registry_decode_instructions():
    reg = _mk_registry()
    pd = reg.providerByAppID[7]
    body = pd.encode("Mint", {"to": new_address_from_bytes(bytes(range(1, 21))), "amount": 5})

    decoded = pv.decode_instruction(reg, body)
    assert decoded["app_id"] == 7
    assert decoded["instruction_name"] == "Mint"
    assert decoded["args"]["amount"] == 5

    decoded_list = pv.decode_instructions(reg, [body])
    assert len(decoded_list) == 1

    with pytest.raises(ValueError, match="unknown app_id"):
        pv.decode_instruction(reg, b"\x99\x01\x02")


def test_idl_registry_decode_view_datas():
    reg = _mk_registry()
    ser = Serializer()
    ser.serialize_u32(1)  # vec len
    ser.serialize_enum_variant(0)  # Ok
    ser.serialize_bytes(b"\x4d")  # u64(77)

    values = pv.decode_view_datas(reg, ["tapp::BalanceOf"], ser.bytes())
    assert values[0].Value == 77

    with pytest.raises(ValueError, match="invalid format"):
        pv.decode_view_datas(reg, ["BalanceOf"], ser.bytes())


def test_idl_registry_decode_event_data_by_tag():
    reg = _mk_registry()
    ser = Serializer()
    ser.serialize_u64(3001)  # 前置 type_tag varint（应被跳过）
    ser.serialize_fixed_bytes(bytes(range(1, 21)))
    ser.serialize_u64(9)

    decoded = pv.decode_event_data_by_tag(reg, 3001, ser.bytes())
    assert decoded["event_name"] == "Transfer"
    assert decoded["data"]["value"] == 9

    with pytest.raises(ValueError, match="unknown type tag"):
        pv.decode_event_data_by_tag(reg, 999, b"")


def test_idl_registry_decode_resource_data_by_tag():
    reg = _mk_registry()
    decoded = pv.decode_resource_data_by_tag(reg, 2001, b"\x2a")
    assert decoded["resource_name"] == "_Supply"
    assert decoded["data"] == 42

    # types fallback
    ser = Serializer()
    ser.serialize_str("n")
    ser.serialize_u64(4)
    decoded = pv.decode_resource_data_by_tag(reg, 1001, ser.bytes())
    assert decoded["data"] == {"name": "n", "supply": 4}

    with pytest.raises(ValueError, match="unknown resource type tag"):
        pv.decode_resource_data_by_tag(reg, 999, b"")


def test_idl_registry_format_decoded():
    reg = _mk_registry()
    pd = reg.providerByAppID[7]
    body = pd.encode("Mint", {"to": new_address_from_bytes(bytes(range(1, 21))), "amount": 5})
    decoded = pv.decode_instruction(reg, body)
    text = pv.format_decoded_instruction(decoded)
    assert "[tapp] Mint" in text
    assert 'instructionName: "Mint"' in text

    ser = Serializer()
    ser.serialize_fixed_bytes(bytes(range(1, 21)))
    ser.serialize_u64(9)
    ev = pv.decode_event_data_by_tag(reg, 3001, ser.bytes())
    etext = pv.format_decoded_event(ev)
    assert "[tapp] Transfer" in etext


def test_real_idl_token_encode_decode_roundtrip():
    """对应 Go TestProviderTokenEncodeAndDecode：真实 token IDL 集成。"""
    idls = pv.load_default_idls()
    pd = pv.new_provider(idls["token"])
    addr = new_address_from_bytes(bytes(range(1, 21)))

    name = next(
        n
        for n, i in pd.InstructionByName.items()
        if i.Kind == "entry" and len(i.Args) >= 1 and i.Args[0].Type == "Address"
    )
    args_in = {a.Name: addr if a.Type == "Address" else 1 for a in pd.InstructionByName[name].Args}
    body = pd.encode(name, args_in)
    args_out = pd.decode(name, body)
    assert args_out[args_in and pd.InstructionByName[name].Args[0].Name] == addr


def test_decode_data_by_idl_type_name():
    pd = pv.new_provider(_inline_idl())
    ser = Serializer()
    ser.serialize_str("n")
    ser.serialize_u64(4)
    value = pd.decode_data_by_idl_type_name("Meta", ser.bytes())
    assert value == {"name": "n", "supply": 4}

    with pytest.raises(ValueError, match="empty resource data"):
        pd.decode_data_by_idl_type_name("Meta", b"")


def test_serialize_enum_variant():
    """对应 Go TestSerializeEnumVariant：enum 输入三形态。"""
    pd = pv.new_provider(_inline_idl())
    ser = Serializer()
    pd._serialize_value(ser, "Mode", {"Set": {"v": 3}})  # 单键对象
    pd._serialize_value(ser, "Mode", {"variant": "Set", "value": {"v": 4}})
    body = ser.bytes()
    v1, offset = pd.deserialize_value("Mode", body, 0)
    v2, offset = pd.deserialize_value("Mode", body, offset)
    assert v1["v"] == 3
    assert v2["v"] == 4

    with pytest.raises(ValueError, match="unknown enum variant"):
        ser2 = Serializer()
        pd._serialize_value(ser2, "Mode", "Nope")
