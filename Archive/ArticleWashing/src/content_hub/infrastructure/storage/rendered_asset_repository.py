from __future__ import annotations

import json
from pathlib import Path

from content_hub.domain.formatting.models import RenderedAsset


class FileRenderedAssetRepository:
    def __init__(self, path: Path):
        self.path = path

    def save(self, asset: RenderedAsset) -> RenderedAsset:
        payload = self._list_payload()
        remaining = [item for item in payload if item["asset_id"] != asset.asset_id]
        remaining.append(self._to_payload(asset))
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.path.write_text(json.dumps(remaining, ensure_ascii=False, indent=2), encoding="utf-8")
        return asset

    def get(self, asset_id: str) -> RenderedAsset | None:
        for item in self._list_payload():
            if item["asset_id"] == asset_id:
                return self._from_payload(item)
        return None

    def list_assets(self, article_id: str | None = None, platform: str | None = None) -> list[RenderedAsset]:
        assets = [self._from_payload(item) for item in self._list_payload()]
        if article_id is not None:
            assets = [item for item in assets if item.article_id == article_id]
        if platform is not None:
            assets = [item for item in assets if item.platform == platform]
        return assets

    def _list_payload(self) -> list[dict]:
        if not self.path.exists():
            return []
        return json.loads(self.path.read_text(encoding="utf-8"))

    def _to_payload(self, asset: RenderedAsset) -> dict:
        return {
            "asset_id": asset.asset_id,
            "article_id": asset.article_id,
            "platform": asset.platform,
            "output_format": asset.output_format,
            "template": asset.template,
            "content": asset.content,
            "artifact_path": str(asset.artifact_path) if asset.artifact_path is not None else None,
            "warnings": list(asset.warnings),
            "status": asset.status,
        }

    def _from_payload(self, payload: dict) -> RenderedAsset:
        artifact_path = payload.get("artifact_path")
        return RenderedAsset(
            asset_id=payload["asset_id"],
            article_id=payload["article_id"],
            platform=payload["platform"],
            output_format=payload["output_format"],
            template=payload["template"],
            content=payload["content"],
            artifact_path=Path(artifact_path) if artifact_path else None,
            warnings=list(payload.get("warnings", [])),
            status=payload.get("status", "ready"),
        )
