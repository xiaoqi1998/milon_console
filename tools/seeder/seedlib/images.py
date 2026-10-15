# -*- coding: utf-8 -*-
"""测试数据图片 URL:全部使用 picsum.photos 固定 seed——

真实公网图床、浏览器直接可见;同 seed 永远同图,重新创世重建后 URL 不变。
"""
BASE = "https://picsum.photos/seed"


def picsum(seed: str, w: int = 600, h: int = 600) -> str:
    # 带 .jpg 后缀:部分环境/预览组件无法打开无扩展名 URL
    return f"{BASE}/{seed}/{w}/{h}.jpg"


def avatar_url(name: str) -> str:
    return picsum(f"milon-ava-{name}", 200, 200)


def token_icon_url(symbol: str) -> str:
    return picsum(f"milon-icon-{symbol.lower()}", 128, 128)


def token_uri(symbol: str) -> str:
    return picsum(f"milon-meta-{symbol.lower()}", 400, 400)


def sft_cover_url(key: str) -> str:
    return picsum(f"milon-sft-{key}", 600, 600)


def slot_cover_url(sft_key: str, slot_key: str) -> str:
    return picsum(f"milon-slot-{sft_key}-{slot_key}", 600, 600)


def card_cover_url(sft_key: str, slot_key: str, idx: int) -> str:
    return picsum(f"milon-card-{sft_key}-{slot_key}-{idx}", 600, 600)
