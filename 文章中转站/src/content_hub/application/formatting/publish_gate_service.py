from __future__ import annotations

from content_hub.application.services.publish_service import PublishService
from content_hub.domain.content.entities import ContentDocument
from content_hub.domain.formatting.models import RenderedAsset, ReviewTask
from content_hub.domain.publish.models import PublishResult


class PublishGateService:
    def __init__(self, publish_service: PublishService):
        self.publish_service = publish_service

    def publish_reviewed_assets(
        self,
        review: ReviewTask,
        assets: list[RenderedAsset],
        article_title: str,
        account_info: dict | None = None,
    ) -> list[PublishResult]:
        if review.status != "approved":
            raise ValueError("review must be approved before publishing")

        asset_map = {asset.asset_id: asset for asset in assets}
        results: list[PublishResult] = []
        for asset_id in review.asset_ids:
            asset = asset_map.get(asset_id)
            if asset is None:
                continue
            if asset.status != "ready":
                raise ValueError(f"asset is not ready: {asset.asset_id}")
            document = ContentDocument(title=article_title, body=asset.content, content_format=asset.output_format)
            results.append(self.publish_service.publish_document(document, platform=asset.platform, account_info=account_info))
        return results
