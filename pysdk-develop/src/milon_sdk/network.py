"""Network 配置. Faithful port of gosdk-develop/network.go."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass
class Network:
    Name: str = ""
    ChainId: int = 0
    RpcUrl: str = ""
    InxUrl: str = ""
    GrpcAddr: str = ""  # 区块流 gRPC 地址,host:port


LOCAL_NET = Network(
    Name="localNet",
    ChainId=900_000_001,
    RpcUrl="http://127.0.0.1:6280/v1/rpc",
    GrpcAddr="127.0.0.1:50051",
)

DEV_NET = Network(
    Name="devNet",
    ChainId=900_000_001,
    RpcUrl="https://devnet.milonlabs.com/v1/rpc",
    GrpcAddr="8.218.101.239:50051",
)
