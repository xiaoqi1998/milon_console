# -*- coding: utf-8 -*-
"""seed_state.json 断点续跑:读写、原子落盘、缺省空。"""
import json

from seedlib.state import State


def test_load_missing_file_is_empty(tmp_path):
    st = State.load(tmp_path / "nope.json")
    assert st.is_done("tokens", "create") is False
    assert st.get("tokens", "create") is None


def test_mark_and_is_done(tmp_path):
    st = State.load(tmp_path / "s.json")
    st.mark("accounts", "faucet", {"test01": "ok"})
    assert st.is_done("accounts", "faucet") is True
    assert st.is_done("accounts", "did") is False
    assert st.is_done("tokens", "faucet") is False  # 模块隔离
    assert st.get("accounts", "faucet") == {"test01": "ok"}


def test_save_load_roundtrip(tmp_path):
    p = tmp_path / "s.json"
    st = State.load(p)
    st.mark("sft", "slot:budget", {"slotId": 3, "tokens": [7, 8]})
    st.mark("sft", "slot:empty", {"slotId": 4, "tokens": []})
    st.save()
    st2 = State.load(p)
    assert st2.get("sft", "slot:budget") == {"slotId": 3, "tokens": [7, 8]}
    assert st2.is_done("sft", "slot:empty")


def test_save_is_atomic_no_tmp_leftover(tmp_path):
    p = tmp_path / "s.json"
    st = State.load(p)
    st.mark("m", "g", {"a": 1})
    st.save()
    leftovers = [f.name for f in tmp_path.iterdir() if f.name != "s.json"]
    assert leftovers == []
    assert json.loads(p.read_text(encoding="utf-8"))["m"]["g"] == {"a": 1}


def test_overwrite_same_stage(tmp_path):
    p = tmp_path / "s.json"
    st = State.load(p)
    st.mark("m", "g", {"v": 1})
    st.mark("m", "g", {"v": 2})
    st.save()
    assert State.load(p).get("m", "g") == {"v": 2}
