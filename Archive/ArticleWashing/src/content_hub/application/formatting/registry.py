from __future__ import annotations

from content_hub.infrastructure.formatters.base import BaseFormatter


class FormatterRegistry:
    def __init__(self):
        self._registry: dict[tuple[str, str], BaseFormatter] = {}

    def register(self, platform: str, output_format: str, formatter: BaseFormatter) -> None:
        self._registry[(platform, output_format)] = formatter

    def get(self, platform: str, output_format: str) -> BaseFormatter | None:
        return self._registry.get((platform, output_format))
