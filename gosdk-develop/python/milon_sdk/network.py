"""网络配置（对应 Go network.go）。

Network 描述一条链的连接信息：名称、ChainId、RPC 地址（双线协议的入口）、
可选的 Inx 地址（索引/查询入口）。
"""
from __future__ import annotations

from dataclasses import dataclass


@dataclass
class Network:
    name: str
    chain_id: int
    rpc_url: str
    inx_url: str = ""

    def __post_init__(self) -> None:
        # 防止误把 None 当成空字符串
        if self.inx_url is None:
            self.inx_url = ""


# 内置网络预设（与 Go 保持一致）
LocalNet = Network(
    name="localNet",
    chain_id=900_000_001,
    rpc_url="http://127.0.0.1:6280/milon/v1",
)

DevNet = Network(
    name="devNet",
    chain_id=900_000_001,
    rpc_url="http://47.84.39.153:6280/milon/v1",
)
