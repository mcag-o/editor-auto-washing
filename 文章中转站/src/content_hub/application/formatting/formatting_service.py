from __future__ import annotations

from content_hub.application.formatting.registry import FormatterRegistry
from content_hub.domain.formatting.models import ArticleDraft, FormatTarget, RenderedAsset
from content_hub.infrastructure.storage.rendered_asset_repository import FileRenderedAssetRepository


class FormattingService:
    def __init__(self, registry: FormatterRegistry, rendered_asset_repository: FileRenderedAssetRepository):
        self.registry = registry
        self.rendered_asset_repository = rendered_asset_repository

    def get_asset(self, asset_id: str) -> RenderedAsset | None:
        return self.rendered_asset_repository.get(asset_id)

    def list_assets(self, article_id: str | None = None, platform: str | None = None) -> list[RenderedAsset]:
        return self.rendered_asset_repository.list_assets(article_id=article_id, platform=platform)

    def format_article(self, article: ArticleDraft, targets: list[FormatTarget]) -> list[RenderedAsset]:
        assets: list[RenderedAsset] = []
        for target in targets:
            formatter = self.registry.get(target.platform, target.output_format)
            if formatter is None:
                raise KeyError(f"formatter is not registered: {target.platform}/{target.output_format}")
            errors = formatter.validate(article, target)
            if errors:
                raise ValueError("; ".join(errors))
            asset = formatter.render(article, target)
            self.rendered_asset_repository.save(asset)
            assets.append(asset)
        return assets
