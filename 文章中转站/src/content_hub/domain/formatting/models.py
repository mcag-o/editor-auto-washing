from __future__ import annotations

from dataclasses import dataclass, field
from pathlib import Path
from typing import Any


@dataclass
class ArticleDraft:
    article_id: str
    template: str
    meta: dict[str, Any]
    headline: dict[str, Any]
    sections: list[dict[str, Any]]
    conclusion: str
    cta: str
    source_refs: list[dict[str, Any]] = field(default_factory=list)
    target_platforms: list[str] = field(default_factory=list)
    status: str = "draft"


@dataclass
class FormatTarget:
    platform: str
    template: str
    output_format: str
    variant: str = "default"


@dataclass
class RenderedAsset:
    asset_id: str
    article_id: str
    platform: str
    output_format: str
    template: str
    content: str
    artifact_path: Path | None
    warnings: list[str] = field(default_factory=list)
    status: str = "ready"


@dataclass
class ReviewTask:
    review_id: str
    article_id: str
    asset_ids: list[str]
    status: str
    reviewer: str = ""
    notes: str = ""
