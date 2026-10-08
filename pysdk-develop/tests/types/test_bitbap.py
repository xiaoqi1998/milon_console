"""Port of gosdk-develop/types/bitmap_test.go."""

import pytest

from milon_sdk.postcard import Deserializer, Serializer
from milon_sdk.types.bitbap import Bitmap64, low_bits_mask


def test_bitmap64_set_and_clear():
    b = Bitmap64()
    assert b.is_empty()
    assert b.count_ones() == 0
    assert not b.test(3)

    b = b.set(3)
    assert not b.is_empty()
    assert b.count_ones() == 1
    assert b.test(3)

    b = b.clear(3)
    assert b.is_empty()
    assert b.count_ones() == 0
    assert not b.test(3)


def test_bitmap64_count_ones():
    tests = [
        ("zero", 0, 0),
        ("one", 1, 1),
        ("binary_1010", 0b1010, 2),
        ("binary_11111111", 0b11111111, 8),
        ("all_ones", 0xFFFFFFFFFFFFFFFF, 64),
    ]
    for name, bits, expected in tests:
        assert Bitmap64(bits).count_ones() == expected, name


def test_bitmap64_lowest_vacant_index():
    tests = [
        ("empty", 0, 0),
        ("only_bit0_set", 1, 1),
        ("bits_0-2_set", 0b111, 3),
        ("bits_0-3_set", 0b1111, 4),
        ("alternating", 0b10101, 1),
        ("all_set", 0xFFFFFFFFFFFFFFFF, 64),
    ]
    for name, bits, expected in tests:
        assert Bitmap64(bits).lowest_vacant_index() == expected, name


def test_bitmap64_is_subset_of():
    occupied = Bitmap64(0b1111)

    assert Bitmap64(0b1).is_subset_of(occupied)

    assert Bitmap64(0b01).is_subset_of(occupied)
    assert Bitmap64(0b10).is_subset_of(occupied)
    assert Bitmap64(0b11).is_subset_of(occupied)

    assert Bitmap64(0b001).is_subset_of(occupied)
    assert Bitmap64(0b010).is_subset_of(occupied)
    assert Bitmap64(0b100).is_subset_of(occupied)
    assert Bitmap64(0b011).is_subset_of(occupied)
    assert Bitmap64(0b101).is_subset_of(occupied)
    assert Bitmap64(0b110).is_subset_of(occupied)
    assert Bitmap64(0b111).is_subset_of(occupied)

    assert Bitmap64(0b1000).is_subset_of(occupied)
    assert Bitmap64(0b0100).is_subset_of(occupied)
    assert Bitmap64(0b0010).is_subset_of(occupied)
    assert Bitmap64(0b0001).is_subset_of(occupied)

    assert not Bitmap64(0b10000).is_subset_of(occupied)


def test_bitmap64_iter_set_bits():
    b = Bitmap64(0b1010)
    assert b.iter_set_bits() == [1, 3]


def test_bitmap64_string_and_go_string_and_format():
    tests = [
        (
            "zero",
            0b0,
            "0000000000000000000000000000000000000000000000000000000000000000",
            "Bitmap64(0b0000000000000000000000000000000000000000000000000000000000000000)",
        ),
        (
            "bit_0_set",
            0b01,
            "0000000000000000000000000000000000000000000000000000000000000001",
            "Bitmap64(0b0000000000000000000000000000000000000000000000000000000000000001)",
        ),
        (
            "multiple_bits",
            0b1010,
            "0000000000000000000000000000000000000000000000000000000000001010",
            "Bitmap64(0b0000000000000000000000000000000000000000000000000000000000001010)",
        ),
    ]
    for name, bits, expected_str, expected_go_string in tests:
        b = Bitmap64(bits)
        assert str(b) == expected_str, name
        assert repr(b) == expected_go_string, name


def test_bitmap64_marshal_postcard():
    tests = [
        ("zero", 0),
        ("one", 1),
        ("small_value", 42),
        ("medium_value", 0xFF),
        ("large_value", 0xFFFF),
        ("very_large_value", 0xFFFFFFFF),
        ("max_value", 0xFFFFFFFFFFFFFFFF),
        ("alternating_bits", 0xAAAAAAAAAAAAAAAA),
    ]
    for name, bits in tests:
        original = Bitmap64(bits)

        serializer = Serializer()
        original.marshal_postcard(serializer)

        deserializer = Deserializer(serializer.bytes())
        back = Bitmap64()
        back.unmarshal_postcard(deserializer)
        assert back == original, name
        assert back.raw() == bits, name

        deserializer.assert_end()


def test_low_bits_mask():
    tests = [
        ("n=0", 0, 0),
        ("n=1", 1, 1),
        ("n=2", 2, 3),
        ("n=3", 3, 7),
        ("n=8", 8, 255),
        ("n=16", 16, 65535),
        ("n=32", 32, 4294967295),
        ("n=64", 64, 0xFFFFFFFFFFFFFFFF),
    ]
    for name, n, expected in tests:
        assert low_bits_mask(n).raw() == expected, name
