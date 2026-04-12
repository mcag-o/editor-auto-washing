from __future__ import annotations

from typing import Protocol

from content_hub.domain.formatting.models import ArticleDraft, FormatTarget, RenderedAsset


class BaseFormatter(Protocol):
    def validate(self, article: ArticleDraft, target: FormatTarget) -> list[str]: ...

    def render(self, article: ArticleDraft, target: FormatTarget) -> RenderedAsset: ...
