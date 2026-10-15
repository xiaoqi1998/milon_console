# -*- coding: utf-8 -*-
"""seed.py CLI:模块选择与默认值。"""
import seed


def test_default_args():
    a = seed.parse_args([])
    assert a.base_url == "http://127.0.0.1:18080"
    assert a.scale == "full"
    assert a.seed_phrase == "milon-devnet-seed-v1"
    assert a.modules == []          # 空 = 未选,main 里报错提示
    assert a.dry_run is False
    assert a.reset is False


def test_all_selects_every_module_in_order():
    a = seed.parse_args(["--all"])
    assert a.modules == seed.MODULE_ORDER


def test_subset_selection():
    a = seed.parse_args(["--tokens", "--vc"])
    assert a.modules == ["tokens", "vc"]


def test_scale_and_flags():
    a = seed.parse_args(["--all", "--scale", "small", "--dry-run", "--reset",
                         "--base-url", "http://127.0.0.1:9999",
                         "--seed-phrase", "abc"])
    assert a.scale == "small" and a.dry_run is True and a.reset is True
    assert a.base_url == "http://127.0.0.1:9999" and a.seed_phrase == "abc"


def test_compliance_requires_vc_dependency():
    # compliance 依赖 vc 产出的证书,选了 compliance 自动带上 vc
    a = seed.parse_args(["--compliance"])
    assert "vc" in a.modules and a.modules.index("vc") < a.modules.index("compliance")


def test_list_prints_modules(capsys):
    seed.parse_args(["--list"])
    out = capsys.readouterr().out
    for m in seed.MODULE_ORDER:
        assert m in out
