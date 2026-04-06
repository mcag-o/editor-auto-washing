from pathlib import Path
import tempfile
import unittest

from content_hub.application.services.config_service import ConfigService
from content_hub.application.formatting.formatting_service import FormattingService
from content_hub.application.formatting.registry import FormatterRegistry
from content_hub.application.formatting.review_service import ReviewService
from content_hub.application.services.content_service import ContentService
from content_hub.application.formatting.publish_gate_service import PublishGateService
from content_hub.application.publishers.record_only_publisher import RecordOnlyPublisher
from content_hub.application.services.publish_service import PublishService
from content_hub.application.services.template_service import TemplateService
from content_hub.application.services.workflow_service import WorkflowService
from content_hub.bootstrap.container import build_container
from content_hub.bootstrap.settings import HubSettings, LLMSettings, PublishSettings, RewriteSettings, StorageSettings, TemplateSettings, WeChatCredential, WorkflowSettings
from content_hub.domain.formatting.models import ArticleDraft, FormatTarget, RenderedAsset, ReviewTask
from content_hub.infrastructure.formatters.base import BaseFormatter
from content_hub.infrastructure.storage.article_repository import FileArticleRepository
from content_hub.infrastructure.storage.publish_record_repository import FilePublishRecordRepository
from content_hub.infrastructure.storage.rendered_asset_repository import FileRenderedAssetRepository
from content_hub.infrastructure.storage.review_task_repository import FileReviewTaskRepository
from content_hub.infrastructure.storage.template_repository import FileTemplateRepository
from content_hub.runtime.nodes.creative import CreativeEnhancementNode
from content_hub.runtime.nodes.generation import StaticGenerationNode
from content_hub.runtime.nodes.persist import PersistNode
from content_hub.runtime.nodes.publish import RecordPublishNode
from content_hub.runtime.nodes.registry import NodeRegistry
from content_hub.runtime.nodes.rewrite import SuffixRewriteNode


class ServiceTestCase(unittest.TestCase):
    class _StubFormatter(BaseFormatter):
        def validate(self, article: ArticleDraft, target: FormatTarget) -> list[str]:
            return []

        def render(self, article: ArticleDraft, target: FormatTarget) -> RenderedAsset:
            return RenderedAsset(
                asset_id=f"{article.article_id}-{target.platform}",
                article_id=article.article_id,
                platform=target.platform,
                output_format=target.output_format,
                template=target.template,
                content=f"{article.meta['title']}::{target.platform}",
                artifact_path=None,
                warnings=[],
            )

    def test_config_service_reads_legacy_config(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            project_root = Path(tmp_dir)
            config_dir = project_root / "src" / "ai_write_x" / "config"
            config_dir.mkdir(parents=True)
            (config_dir / "config.yaml").write_text(
                """
publish_platform: wechat
article_format: markdown
auto_publish: true
wechat:
  credentials:
    - appid: legacy-app
      appsecret: legacy-secret
      author: legacy-author
api:
  api_type: OpenRouter
  OpenRouter:
    model_index: 0
    key_index: 0
    model: [openrouter/legacy-model]
    api_key: [legacy-key]
    api_base: https://example.com/v1
    max_tokens: 4096
dimensional_creative:
  enabled: true
""".strip(),
                encoding="utf-8",
            )

            settings = ConfigService(project_root).load_legacy_settings()

            self.assertEqual(settings.llm.model, "openrouter/legacy-model")
            self.assertEqual(settings.publish.wechat_credentials[0].appid, "legacy-app")
            self.assertTrue(settings.rewrite.enabled)

    def test_template_content_and_publish_services_share_file_repositories(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            template_root = tmp_path / "templates"
            article_root = tmp_path / "articles"
            record_file = tmp_path / "publish_records.json"
            (template_root / "Finance").mkdir(parents=True)
            (template_root / "Finance" / "weekly.html").write_text("<html>weekly</html>", encoding="utf-8")

            template_service = TemplateService(FileTemplateRepository(template_root))
            content_service = ContentService(FileArticleRepository(article_root))
            publish_service = PublishService(FilePublishRecordRepository(record_file))

            self.assertEqual(template_service.list_categories(), ["Finance"])
            self.assertEqual(template_service.list_templates("Finance")[0].name, "weekly")

            artifact = content_service.create_document(title="Weekly Note", body="# Weekly", content_format="markdown")
            self.assertTrue(artifact.artifact_path is not None)
            self.assertTrue(artifact.artifact_path.exists())

            result = publish_service.record_success(
                article_title="Weekly Note",
                platform="wechat",
                account_info={"appid": "demo-app"},
            )
            self.assertTrue(result.success)
            self.assertEqual(publish_service.list_records()["Weekly Note"][0]["platform"], "wechat")

    def test_content_service_lists_and_updates_documents(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            service = ContentService(FileArticleRepository(Path(tmp_dir)))

            artifact = service.create_document(title="Insight", body="# Insight\n\nBody", content_format="markdown")
            documents = service.list_documents()

            self.assertEqual(len(documents), 1)
            self.assertEqual(documents[0].title, "Insight")

            updated = service.update_document(artifact.artifact_path, "# Insight\n\nUpdated")
            self.assertIn("Updated", updated.body)

    def test_template_service_returns_template_content(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir) / "templates"
            (root / "Tech").mkdir(parents=True)
            (root / "Tech" / "alpha.html").write_text("<html>alpha</html>", encoding="utf-8")
            service = TemplateService(FileTemplateRepository(root))

            self.assertEqual(service.read_template("Tech", "alpha"), "<html>alpha</html>")

    def test_template_service_supports_crud_workflow(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir) / "templates"
            service = TemplateService(FileTemplateRepository(root))

            created = service.create_template("Tech", "alpha", "<html>alpha</html>")
            renamed = service.rename_template(created, "beta")
            copied = service.copy_template(renamed, "Finance", "gamma")
            moved = service.move_template(renamed, "Lifestyle")

            self.assertTrue(created.parent.exists())
            self.assertEqual(copied.parent.name, "Finance")
            self.assertEqual(moved.parent.name, "Lifestyle")

    def test_publish_service_can_filter_history_by_article_title(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            service = PublishService(FilePublishRecordRepository(Path(tmp_dir) / "publish_records.json"))
            service.record_success("Article A", "wechat", {"appid": "a"})
            service.record_success("Article B", "wechat", {"appid": "b"})

            history = service.get_history("Article A")

            self.assertEqual(len(history), 1)
            self.assertEqual(history[0]["account_info"]["appid"], "a")

    def test_workflow_service_builds_default_registry_and_executes(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            settings = HubSettings(
                llm=LLMSettings(provider="stub", model="stub-model"),
                workflow=WorkflowSettings(publish_platform="wechat", article_format="markdown", auto_publish=True),
                rewrite=RewriteSettings(enabled=True),
                template=TemplateSettings(root_dir=tmp_path / "templates"),
                storage=StorageSettings(root_dir=tmp_path / "storage"),
                publish=PublishSettings(wechat_credentials=[WeChatCredential(appid="a", appsecret="b", author="c")]),
            )

            registry = NodeRegistry()
            registry.register("generate", StaticGenerationNode())
            registry.register("creative", CreativeEnhancementNode())
            registry.register("persist", PersistNode(FileArticleRepository(settings.storage.article_dir)))
            registry.register(
                "publish",
                RecordPublishNode(
                    PublishService(
                        FilePublishRecordRepository(settings.storage.publish_record_file),
                        {"wechat": RecordOnlyPublisher(FilePublishRecordRepository(settings.storage.publish_record_file))},
                    ),
                    "wechat",
                ),
            )

            service = WorkflowService(registry)
            result = service.run_default_workflow(settings=settings, payload={"topic": "服务化测试"})

            self.assertIn("创意增强", result.document.body)
            self.assertTrue(result.artifact_path is not None)
            self.assertEqual(result.publish_results[0].platform, "wechat")

    def test_review_service_creates_and_approves_review_tasks(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            repository = FileReviewTaskRepository(Path(tmp_dir) / "review_tasks.json")
            service = ReviewService(repository)

            review = service.create_review(article_id="article-1", asset_ids=["asset-1", "asset-2"], reviewer="alice", notes="check wechat")
            approved = service.approve_review(review.review_id, reviewer="bob", notes="ok")

            self.assertEqual(review.status, "review_pending")
            self.assertEqual(approved.status, "approved")
            self.assertEqual(approved.reviewer, "bob")

    def test_publish_gate_service_blocks_unapproved_assets(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            record_repository = FilePublishRecordRepository(Path(tmp_dir) / "publish_records.json")
            publish_service = PublishService(
                record_repository,
                {"wechat": RecordOnlyPublisher(record_repository)},
            )
            gate = PublishGateService(publish_service)
            asset = RenderedAsset(
                asset_id="asset-1",
                article_id="article-1",
                platform="wechat",
                output_format="html",
                template="daily-intelligence",
                content="<html>ok</html>",
                artifact_path=None,
                warnings=[],
            )
            review = ReviewTask(review_id="review-1", article_id="article-1", asset_ids=["asset-1"], status="review_pending")

            with self.assertRaises(ValueError):
                gate.publish_reviewed_assets(review, [asset], article_title="AI 日报")

    def test_formatting_service_and_publish_gate_can_publish_approved_assets(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            registry = FormatterRegistry()
            registry.register("wechat", "html", self._StubFormatter())
            rendered_asset_repository = FileRenderedAssetRepository(tmp_path / "rendered_assets.json")
            formatting_service = FormattingService(registry, rendered_asset_repository)
            review_service = ReviewService(FileReviewTaskRepository(tmp_path / "review_tasks.json"))
            record_repository = FilePublishRecordRepository(tmp_path / "publish_records.json")
            publish_service = PublishService(record_repository, {"wechat": RecordOnlyPublisher(record_repository)})
            gate = PublishGateService(publish_service)

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
            assets = formatting_service.format_article(article, [FormatTarget(platform="wechat", template="daily-intelligence", output_format="html")])
            review = review_service.create_review(article_id=article.article_id, asset_ids=[assets[0].asset_id])
            review = review_service.approve_review(review.review_id)

            results = gate.publish_reviewed_assets(review, assets, article_title="AI 日报")

            self.assertEqual(len(results), 1)
            self.assertTrue(results[0].success)
            self.assertEqual(results[0].platform, "wechat")
            self.assertEqual(publish_service.get_history("AI 日报")[0]["platform"], "wechat")

    def test_container_registers_wechat_formatter_with_structured_templates(self) -> None:
        project_root = Path(__file__).resolve().parents[3]
        settings = HubSettings(
            llm=LLMSettings(provider="stub", model="stub-model"),
            workflow=WorkflowSettings(publish_platform="record-only", article_format="markdown", auto_publish=False),
            rewrite=RewriteSettings(enabled=False),
            template=TemplateSettings(root_dir=project_root / "文章中转站" / "knowledge" / "templates"),
            storage=StorageSettings(root_dir=project_root / "文章中转站" / ".tmp-test-storage"),
            publish=PublishSettings(),
        )
        container = build_container(project_root, settings)

        formatter = container.formatting_service.registry.get("wechat", "html")

        self.assertIsNotNone(formatter)
        self.assertIn("daily-intelligence", formatter.template_catalog.list_templates())
        self.assertIn("studio-brief", formatter.template_catalog.list_templates())


if __name__ == "__main__":
    unittest.main()
