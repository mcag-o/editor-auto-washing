from pathlib import Path
import tempfile
import unittest

from content_hub.domain.formatting.models import ArticleDraft, FormatTarget, RenderedAsset, ReviewTask
from content_hub.infrastructure.storage.rendered_asset_repository import FileRenderedAssetRepository
from content_hub.infrastructure.storage.review_task_repository import FileReviewTaskRepository


class FormattingModelsTestCase(unittest.TestCase):
    def test_article_draft_and_rendered_asset_can_roundtrip_via_file_repositories(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            asset_repo = FileRenderedAssetRepository(tmp_path / "rendered_assets.json")
            review_repo = FileReviewTaskRepository(tmp_path / "review_tasks.json")

            draft = ArticleDraft(
                article_id="article-1",
                template="daily-intelligence",
                meta={"title": "AI 日报", "digest": "摘要", "author": "editor"},
                headline={"title": "头条", "body": ["第一段", "第二段"]},
                sections=[
                    {
                        "cn": "要闻",
                        "en": "BRIEFING",
                        "blocks": [
                            {
                                "type": "card",
                                "title": "新闻标题",
                                "body": "新闻正文",
                                "source": "Example Source",
                            }
                        ],
                    }
                ],
                conclusion="结论",
                cta="行动建议",
                source_refs=[{"platform": "weibo", "url": "https://example.com/1"}],
                target_platforms=["wechat"],
            )
            target = FormatTarget(platform="wechat", template="daily-intelligence", output_format="html")
            asset = RenderedAsset(
                asset_id="asset-1",
                article_id=draft.article_id,
                platform=target.platform,
                output_format=target.output_format,
                template=target.template,
                content="<html><body>ok</body></html>",
                artifact_path=tmp_path / "wechat.html",
                warnings=["warning-1"],
            )
            review = ReviewTask(
                review_id="review-1",
                article_id=draft.article_id,
                asset_ids=[asset.asset_id],
                status="review_pending",
                reviewer="alice",
                notes="check title",
            )

            asset_repo.save(asset)
            review_repo.save(review)

            saved_assets = asset_repo.list_assets(article_id=draft.article_id)
            saved_review = review_repo.get("review-1")

            self.assertEqual(len(saved_assets), 1)
            self.assertEqual(saved_assets[0].asset_id, "asset-1")
            self.assertEqual(saved_assets[0].warnings, ["warning-1"])
            self.assertEqual(saved_assets[0].artifact_path, tmp_path / "wechat.html")
            self.assertIsNotNone(saved_review)
            self.assertEqual(saved_review.asset_ids, ["asset-1"])
            self.assertEqual(saved_review.reviewer, "alice")

    def test_rendered_asset_repository_can_filter_by_platform(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            repo = FileRenderedAssetRepository(Path(tmp_dir) / "rendered_assets.json")
            repo.save(
                RenderedAsset(
                    asset_id="asset-wechat",
                    article_id="article-1",
                    platform="wechat",
                    output_format="html",
                    template="daily-intelligence",
                    content="wechat",
                    artifact_path=None,
                    warnings=[],
                )
            )
            repo.save(
                RenderedAsset(
                    asset_id="asset-note",
                    article_id="article-1",
                    platform="xiaohongshu",
                    output_format="note",
                    template="daily-intelligence",
                    content="note",
                    artifact_path=None,
                    warnings=[],
                )
            )

            wechat_assets = repo.list_assets(article_id="article-1", platform="wechat")

            self.assertEqual(len(wechat_assets), 1)
            self.assertEqual(wechat_assets[0].asset_id, "asset-wechat")


if __name__ == "__main__":
    unittest.main()
