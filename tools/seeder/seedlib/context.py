# -*- coding: utf-8 -*-
"""模块执行上下文。"""
from dataclasses import dataclass
from typing import Optional


@dataclass
class Ctx:
    api: object            # seedlib.api.Api
    state: object          # seedlib.state.State
    plan: object           # seedlib.plan.Plan
    accts: dict            # 名称 -> 账户(含 address/privateKey/publicKey/alias…)
    log: object = print

    def acct(self, name: str) -> dict:
        return self.accts[name]

    def user_map(self) -> dict:
        """名称(含 'owner'=admin)→ 地址,SFT 分发用。"""
        m = {u.name: self.accts[u.name]["address"] for u in self.plan.users}
        m["owner"] = self.accts["admin"]["address"]
        return m
