"""type_tag → Provider 解析器（对应 Go provider/idlTypeResolver.go）。

供 postcard.Deserializer 注入：把 type_tag 解析为值字节区间，使 api 层无需
预知具体类型即可解码资源/事件。type_tag 冲突时第一个注册的 Provider 胜出。
"""
from __future__ import annotations

from .provider import Provider, _Offset


class IDLTypeResolver:
    def __init__(self, providers: dict[str, Provider]):
        self._providers = providers
        self._resource_index: dict[int, Provider] | None = None
        self._event_index: dict[int, Provider] | None = None

    def _ensure_indexes(self) -> None:
        if self._resource_index is not None:
            return
        resource: dict[int, Provider] = {}
        event: dict[int, Provider] = {}
        for name in sorted(self._providers):
            pd = self._providers[name]
            for type_tag in pd.idl_type_by_type_tag:
                resource.setdefault(type_tag, pd)
            for type_tag in pd.event_by_type_tag:
                event.setdefault(type_tag, pd)
        self._resource_index = resource
        self._event_index = event

    def decode_resource(self, type_tag: int, data: bytes) -> tuple[bytes, bytes]:
        self._ensure_indexes()
        pd = self._resource_index.get(type_tag)
        if pd is None:
            raise ValueError(f"unknown resource type_tag {type_tag} (not found in any loaded IDL)")
        idl_type = pd.get_idl_type_by_type_tag(type_tag)
        off = _Offset(0)
        off = pd.deserialize_value(idl_type.name, data, off)
        return data[: off.pos], data[off.pos:]

    def decode_event(self, type_tag: int, data: bytes) -> tuple[bytes, bytes]:
        self._ensure_indexes()
        pd = self._event_index.get(type_tag)
        if pd is None:
            raise ValueError(f"unknown event type_tag {type_tag} (not found in any loaded IDL)")
        event = pd.get_event_by_type_tag(type_tag)
        off = _Offset(0)
        for field_ in event.fields:
            off = pd.deserialize_value(field_.type, data, off)
        return data[: off.pos], data[off.pos:]
