# -*- coding: utf-8 -*-
"""图片 URL 生成器:真实可见(picsum.photos)、按 seed 确定性、重建不变。"""
from seedlib import images


def test_picsum_basic_format():
    assert images.picsum("abc", 600, 600) == "https://picsum.photos/seed/abc/600/600.jpg"


def test_picsum_deterministic():
    assert images.picsum("x") == images.picsum("x")
    assert images.picsum("x") != images.picsum("y")


def test_avatar_url():
    u = images.avatar_url("test01")
    assert u == "https://picsum.photos/seed/milon-ava-test01/200/200.jpg"


def test_token_icon_url():
    u = images.token_icon_url("SEED6")
    assert u == "https://picsum.photos/seed/milon-icon-seed6/128/128.jpg"


def test_token_uri():
    u = images.token_uri("SEED6")
    assert u == "https://picsum.photos/seed/milon-meta-seed6/400/400.jpg"


def test_sft_cover_url():
    u = images.sft_cover_url("genesis")
    assert u == "https://picsum.photos/seed/milon-sft-genesis/600/600.jpg"


def test_slot_cover_url():
    u = images.slot_cover_url("genesis", "burn_shell")
    assert u == "https://picsum.photos/seed/milon-slot-genesis-burn_shell/600/600.jpg"


def test_card_cover_url():
    u = images.card_cover_url("genesis", "multi_holder", 3)
    assert u == "https://picsum.photos/seed/milon-card-genesis-multi_holder-3/600/600.jpg"


def test_all_urls_https_and_seed_stable():
    for u in (images.avatar_url("a"), images.token_icon_url("B"),
              images.sft_cover_url("c"), images.slot_cover_url("d", "e"),
              images.card_cover_url("d", "e", 1)):
        assert u.startswith("https://picsum.photos/seed/")
