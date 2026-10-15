# -*- coding: utf-8 -*-
"""seed_state.json:断点续跑状态。模块→阶段→产物数据。"""
import json
import os


class State:
    def __init__(self, data: dict, path):
        self.data = data if isinstance(data, dict) else {}
        self.path = str(path)

    @classmethod
    def load(cls, path) -> "State":
        try:
            with open(path, encoding="utf-8") as f:
                data = json.load(f)
        except (OSError, ValueError):
            data = {}
        return cls(data, path)

    def is_done(self, module: str, stage: str) -> bool:
        return stage in self.data.get(module, {})

    def get(self, module: str, stage: str, default=None):
        return self.data.get(module, {}).get(stage, default)

    def mark(self, module: str, stage: str, data: dict) -> None:
        self.data.setdefault(module, {})[stage] = data

    def save(self) -> None:
        tmp = self.path + ".tmp"
        with open(tmp, "w", encoding="utf-8") as f:
            json.dump(self.data, f, ensure_ascii=False, indent=1)
        os.replace(tmp, self.path)  # 原子替换,不留半截文件
