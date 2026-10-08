"""IDLTypeResolver. Faithful port of gosdk-develop/provider/idlTypeResolver.go."""

from __future__ import annotations

import threading
from typing import Dict, Optional, Tuple

from .provider import Provider


class IDLTypeResolver:
    """Resolves typeTag values into raw byte ranges using the provider's
    IDL-driven deserializer, so the api package can decode resources and
    events without knowing concrete types in advance."""

    def __init__(self, providers: Dict[str, Provider]) -> None:
        # Providers maps IDL name -> Provider. It must be fully populated
        # before the first decode call and must not be mutated afterwards;
        # the typeTag indexes are built lazily on first use (thread-safe).
        self.Providers = providers
        self._lock = threading.Lock()
        self._built = False
        self._provider_by_resource_type_tag: Dict[int, Provider] = {}
        self._provider_by_type_tag: Dict[int, Provider] = {}
        self._provider_by_event_type_tag: Dict[int, Provider] = {}

    def _build_indexes(self) -> None:
        """Precomputes typeTag -> Provider maps. On collisions the first
        registered provider wins (deterministic)."""
        self._provider_by_resource_type_tag = {}
        self._provider_by_type_tag = {}
        self._provider_by_event_type_tag = {}
        for name in sorted(self.Providers.keys()):
            pd = self.Providers[name]
            for type_tag in pd.ResourceByTypeTag:
                if type_tag not in self._provider_by_resource_type_tag:
                    self._provider_by_resource_type_tag[type_tag] = pd
            # Builtin tags such as Address or u64 repeat across IDLs with the
            # same meaning, so first-wins stays deterministic.
            for type_tag in pd.IDLTypeByTypeTag:
                if type_tag not in self._provider_by_type_tag:
                    self._provider_by_type_tag[type_tag] = pd
            for type_tag in pd.EventByTypeTag:
                if type_tag not in self._provider_by_event_type_tag:
                    self._provider_by_event_type_tag[type_tag] = pd
        self._built = True

    def _ensure_indexes(self) -> None:
        if not self._built:
            with self._lock:
                if not self._built:
                    self._build_indexes()

    def decode_resource(
        self, type_tag: int, data: bytes
    ) -> Tuple[bytes, bytes]:
        """Returns the consumed bytes of the persisted value registered under
        typeTag plus the remaining bytes. Resource declarations win; when the
        tag is not declared as a resource the IDL types section is used as
        fallback."""
        self._ensure_indexes()

        target_provider = self._provider_by_resource_type_tag.get(type_tag)
        if target_provider is not None:
            target_resource = target_provider.get_resource_by_type_tag(type_tag)

            offset = 0
            try:
                _, offset = target_provider.deserialize_value(
                    target_resource.Type, data, offset
                )
            except ValueError as exc:
                raise ValueError(
                    f"deserialize resource {target_resource.Name} "
                    f"({target_resource.Type}) failed: {exc}"
                ) from exc

            return data[:offset], data[offset:]

        target_provider = self._provider_by_type_tag.get(type_tag)
        if target_provider is not None:
            target_type = target_provider.get_idl_type_by_type_tag(type_tag)

            offset = 0
            try:
                _, offset = target_provider.deserialize_value(
                    target_type.Name, data, offset
                )
            except ValueError as exc:
                raise ValueError(
                    f"deserialize type {target_type.Name} failed: {exc}"
                ) from exc

            return data[:offset], data[offset:]

        raise ValueError(
            f"unknown resource type_tag {type_tag} "
            f"(not found in any loaded IDL)"
        )

    def decode_event(self, type_tag: int, data: bytes) -> Tuple[bytes, bytes]:
        """Returns the consumed bytes of the event registered under typeTag
        plus the remaining bytes."""
        self._ensure_indexes()

        target_provider = self._provider_by_event_type_tag.get(type_tag)
        if target_provider is None:
            raise ValueError(
                f"unknown event type_tag {type_tag} "
                f"(not found in any loaded IDL)"
            )
        target_event = target_provider.get_event_by_type_tag(type_tag)

        offset = 0
        for field in target_event.Fields:
            try:
                _, offset = target_provider.deserialize_value(
                    field.Type, data, offset
                )
            except ValueError as exc:
                raise ValueError(
                    f"deserialize event field {field.Name} ({field.Type}) "
                    f"failed: {exc}"
                ) from exc

        return data[:offset], data[offset:]
