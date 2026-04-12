from pathlib import Path
import tempfile
import unittest

from content_hub.application.formatting.formatting_service import FormattingService
from content_hub.application.formatting.registry import FormatterRegistry
from content_hub.domain.formatting.models import ArticleDraft, FormatTarget, RenderedAsset
from content_hub.infrastructure.formatters.base import BaseFormatter
from content_hub.infrastructure.storage.rendered_asset_repository import FileRenderedAssetRepository


class StubFormatter(BaseFormatter):
    def validate(self, article: ArticleDraft, target: FormatTarget) -> list[str]:
        if article.meta.get("title") == "":
            return ["title is required"]
        return []

    def render(self, article: ArticleDraft, target: FormatTarget) -> RenderedAsset:
        return RenderedAsset(
            asset_id=f"{article.article_id}-{target.platform}",
            article_id=article.article_id,
            platform=target.platform,
            output_format=target.output_format,
            template=target.template,
            content=f"rendered:{article.meta['title']}:{target.platform}",
            artifact_path=None,
            warnings=[],
        )


class FormattingServiceTestCase(unittest.TestCase):
    def test_registry_returns_formatter_for_platform_and_output_format(self) -> None:
        registry = FormatterRegistry()
        formatter = StubFormatter()

        registry.register("wechat", "html", formatter)

        self.assertIs(registry.get("wechat", "html"), formatter)

    def test_service_renders_and_persists_assets_for_targets(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            registry = FormatterRegistry()
            registry.register("wechat", "html", StubFormatter())
            repository = FileRenderedAssetRepository(Path(tmp_dir) / "rendered_assets.json")
            service = FormattingService(registry, repository)
            article = ArticleDraft(
                article_id="article-1",
                template="daily-intelligence",
                meta={"title": "AI 日报", "digest": "摘要", "author": "editor"},
                headline={"title": "头条", "body": ["第一段"]},
                sections=[],
                conclusion="结论",
                cta="行动",
                source_refs=[],
                target_platforms=["wechat"],
            )
            target = FormatTarget(platform="wechat", template="daily-intelligence", output_format="html")

            assets = service.format_article(article, [target])

            self.assertEqual(len(assets), 1)
            self.assertEqual(assets[0].content, "rendered:AI 日报:wechat")
            self.assertEqual(repository.list_assets(article_id="article-1")[0].asset_id, assets[0].asset_id)

    def test_service_raises_for_unknown_formatter(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            service = FormattingService(FormatterRegistry(), FileRenderedAssetRepository(Path(tmp_dir) / "rendered_assets.json"))
            article = ArticleDraft(
                article_id="article-1",
                template="daily-intelligence",
                meta={"title": "AI 日报", "digest": "摘要", "author": "editor"},
                headline={"title": "头条", "body": ["第一段"]},
                sections=[],
                conclusion="结论",
                cta="行动",
                source_refs=[],
                target_platforms=["wechat"],
            )

            with self.assertRaises(KeyError):
                service.format_article(article, [FormatTarget(platform="wechat", template="daily-intelligence", output_format="html")])


if __name__ == "__main__":
    unittest.main()
