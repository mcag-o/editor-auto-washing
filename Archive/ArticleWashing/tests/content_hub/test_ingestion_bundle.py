from __future__ import annotations

import json
import tempfile
import unittest
from pathlib import Path

from content_hub.application.services.ingestion_service import IngestionService
from content_hub.application.workspace.article_service import WorkspaceArticleService
from content_hub.infrastructure.storage.ingestion_repository import FileHotTopicIngestionRepository
from content_hub.infrastructure.storage.ingestion_repository import FileRawContentIngestionRepository
from content_hub.infrastructure.storage.ingestion_repository import FileReferenceIngestionRepository
from content_hub.infrastructure.storage.workspace_article_repository import FileWorkspaceArticleRepository


class IngestionBundleTestCase(unittest.TestCase):
    def test_import_bundle_persists_items_and_creates_workspace_articles(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            service = IngestionService(
                reference_repository=FileReferenceIngestionRepository(tmp_path / "reference_urls.json"),
                raw_content_repository=FileRawContentIngestionRepository(tmp_path / "raw_contents.json"),
                hot_topic_repository=FileHotTopicIngestionRepository(tmp_path / "hot_topics.json"),
                workspace_article_service=WorkspaceArticleService(
                    FileWorkspaceArticleRepository(tmp_path / "workspace_articles.json")
                ),
            )
            bundle = {
                "items": [
                    {"title": "First Item", "url": "https://example.com/a", "platform": "weibo"},
                    {"title": "Second Item", "url": "https://example.com/b", "platform": "zhihu"},
                ]
            }

            summary = service.import_content_hub_bundle(
                bundle=bundle,
                provider_profile="daily-provider",
                article_profile="daily-article",
                publish_profile="record-only",
            )

            self.assertEqual(summary["imported_count"], 2)
            self.assertEqual(summary["created_article_ids"], ["https://example.com/a", "https://example.com/b"])
            self.assertEqual(summary["skipped_count"], 0)
            self.assertEqual(summary["skip_reasons"], [])
            self.assertEqual(len(service.hot_topic_repository.list_items()), 2)

    def test_import_bundle_reports_skips_when_workspace_article_service_missing(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            service = IngestionService(
                reference_repository=FileReferenceIngestionRepository(tmp_path / "reference_urls.json"),
                raw_content_repository=FileRawContentIngestionRepository(tmp_path / "raw_contents.json"),
                hot_topic_repository=FileHotTopicIngestionRepository(tmp_path / "hot_topics.json"),
            )
            bundle = {
                "items": [
                    {"title": "First Item", "url": "https://example.com/a", "platform": "weibo"},
                    {"title": "Second Item", "url": "https://example.com/b", "platform": "zhihu"},
                ]
            }

            summary = service.import_content_hub_bundle(
                bundle=bundle,
                provider_profile="daily-provider",
                article_profile="daily-article",
                publish_profile="record-only",
            )

            self.assertEqual(summary["imported_count"], 2)
            self.assertEqual(summary["created_article_ids"], [])
            self.assertEqual(summary["skipped_count"], 2)
            self.assertEqual(summary["skip_reasons"][0]["reason"], "workspace article service is not configured")
            self.assertEqual(len(service.hot_topic_repository.list_items()), 2)

    def test_import_bundle_skips_non_object_items_and_continues(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            service = IngestionService(
                reference_repository=FileReferenceIngestionRepository(tmp_path / "reference_urls.json"),
                raw_content_repository=FileRawContentIngestionRepository(tmp_path / "raw_contents.json"),
                hot_topic_repository=FileHotTopicIngestionRepository(tmp_path / "hot_topics.json"),
                workspace_article_service=WorkspaceArticleService(
                    FileWorkspaceArticleRepository(tmp_path / "workspace_articles.json")
                ),
            )
            bundle = {
                "items": [
                    {"title": "First Item", "url": "https://example.com/a", "platform": "weibo"},
                    "invalid",
                ]
            }

            summary = service.import_content_hub_bundle(
                bundle=bundle,
                provider_profile="daily-provider",
                article_profile="daily-article",
                publish_profile="record-only",
            )

            self.assertEqual(summary["imported_count"], 2)
            self.assertEqual(summary["created_article_ids"], ["https://example.com/a"])
            self.assertEqual(summary["skipped_count"], 1)
            self.assertEqual(summary["skip_reasons"][0]["reason"], "bundle item is not an object")
            self.assertEqual(len(service.hot_topic_repository.list_items()), 2)

    def test_import_bundles_from_directory_success_and_archive(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            incoming_dir = tmp_path / "incoming"
            archive_dir = tmp_path / "archive"
            incoming_dir.mkdir(parents=True, exist_ok=True)

            (incoming_dir / "a.json").write_text(
                json.dumps(
                    {
                        "items": [
                            {"title": "First Item", "url": "https://example.com/a", "platform": "weibo"}
                        ]
                    }
                ),
                encoding="utf-8",
            )
            (incoming_dir / "b.json").write_text(
                json.dumps(
                    {
                        "items": [
                            {"title": "Second Item", "url": "https://example.com/b", "platform": "zhihu"},
                            {"title": "Third Item", "url": "https://example.com/c", "platform": "weibo"},
                        ]
                    }
                ),
                encoding="utf-8",
            )

            service = IngestionService(
                reference_repository=FileReferenceIngestionRepository(tmp_path / "reference_urls.json"),
                raw_content_repository=FileRawContentIngestionRepository(tmp_path / "raw_contents.json"),
                hot_topic_repository=FileHotTopicIngestionRepository(tmp_path / "hot_topics.json"),
                workspace_article_service=WorkspaceArticleService(
                    FileWorkspaceArticleRepository(tmp_path / "workspace_articles.json")
                ),
            )

            summary = service.import_content_hub_bundles_from_directory(
                incoming_dir=incoming_dir,
                provider_profile="daily-provider",
                article_profile="daily-article",
                publish_profile="record-only",
                archive_dir=archive_dir,
            )

            self.assertEqual(summary["scanned_files"], 2)
            self.assertEqual(summary["imported_files"], 2)
            self.assertEqual(summary["failed_files"], 0)
            self.assertEqual(summary["total_imported_items"], 3)
            self.assertEqual(summary["total_created_articles"], 3)
            self.assertEqual(len(summary["file_results"]), 2)
            self.assertFalse((incoming_dir / "a.json").exists())
            self.assertFalse((incoming_dir / "b.json").exists())
            self.assertTrue((archive_dir / "a.json").exists())
            self.assertTrue((archive_dir / "b.json").exists())

    def test_import_bundles_from_directory_handles_malformed_json(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            incoming_dir = tmp_path / "incoming"
            incoming_dir.mkdir(parents=True, exist_ok=True)

            (incoming_dir / "ok.json").write_text(
                json.dumps(
                    {
                        "items": [
                            {"title": "First Item", "url": "https://example.com/a", "platform": "weibo"}
                        ]
                    }
                ),
                encoding="utf-8",
            )
            (incoming_dir / "bad.json").write_text("{not-valid-json", encoding="utf-8")

            service = IngestionService(
                reference_repository=FileReferenceIngestionRepository(tmp_path / "reference_urls.json"),
                raw_content_repository=FileRawContentIngestionRepository(tmp_path / "raw_contents.json"),
                hot_topic_repository=FileHotTopicIngestionRepository(tmp_path / "hot_topics.json"),
                workspace_article_service=WorkspaceArticleService(
                    FileWorkspaceArticleRepository(tmp_path / "workspace_articles.json")
                ),
            )

            summary = service.import_content_hub_bundles_from_directory(
                incoming_dir=incoming_dir,
                provider_profile="daily-provider",
                article_profile="daily-article",
                publish_profile="record-only",
            )

            self.assertEqual(summary["scanned_files"], 2)
            self.assertEqual(summary["imported_files"], 1)
            self.assertEqual(summary["failed_files"], 1)
            self.assertEqual(summary["total_imported_items"], 1)
            self.assertEqual(summary["total_created_articles"], 1)
            self.assertEqual(len(summary["file_results"]), 2)

            bad_result = next(
                result for result in summary["file_results"] if result["file_name"] == "bad.json"
            )
            ok_result = next(
                result for result in summary["file_results"] if result["file_name"] == "ok.json"
            )
            self.assertEqual(bad_result["status"], "failed")
            self.assertIn("Expecting property name enclosed in double quotes", bad_result["error"])
            self.assertNotIn("failed_moved", bad_result)
            self.assertEqual(ok_result["status"], "imported")
            self.assertTrue((incoming_dir / "bad.json").exists())

    def test_import_bundles_from_directory_moves_failed_files_to_failed_dir(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            incoming_dir = tmp_path / "incoming"
            failed_dir = tmp_path / "failed"
            incoming_dir.mkdir(parents=True, exist_ok=True)

            (incoming_dir / "ok.json").write_text(
                json.dumps(
                    {
                        "items": [
                            {"title": "First Item", "url": "https://example.com/a", "platform": "weibo"}
                        ]
                    }
                ),
                encoding="utf-8",
            )
            (incoming_dir / "bad_json.json").write_text("{not-valid-json", encoding="utf-8")
            (incoming_dir / "bad_bundle.json").write_text(
                json.dumps({"records": []}),
                encoding="utf-8",
            )

            service = IngestionService(
                reference_repository=FileReferenceIngestionRepository(tmp_path / "reference_urls.json"),
                raw_content_repository=FileRawContentIngestionRepository(tmp_path / "raw_contents.json"),
                hot_topic_repository=FileHotTopicIngestionRepository(tmp_path / "hot_topics.json"),
                workspace_article_service=WorkspaceArticleService(
                    FileWorkspaceArticleRepository(tmp_path / "workspace_articles.json")
                ),
            )

            summary = service.import_content_hub_bundles_from_directory(
                incoming_dir=incoming_dir,
                provider_profile="daily-provider",
                article_profile="daily-article",
                publish_profile="record-only",
                failed_dir=failed_dir,
            )

            self.assertEqual(summary["scanned_files"], 3)
            self.assertEqual(summary["imported_files"], 1)
            self.assertEqual(summary["failed_files"], 2)
            self.assertEqual(summary["total_imported_items"], 1)
            self.assertEqual(summary["total_created_articles"], 1)

            bad_json_result = next(
                result for result in summary["file_results"] if result["file_name"] == "bad_json.json"
            )
            bad_bundle_result = next(
                result for result in summary["file_results"] if result["file_name"] == "bad_bundle.json"
            )

            self.assertEqual(bad_json_result["status"], "failed")
            self.assertTrue(bad_json_result["failed_moved"])
            self.assertEqual(bad_bundle_result["status"], "failed")
            self.assertTrue(bad_bundle_result["failed_moved"])

            self.assertFalse((incoming_dir / "bad_json.json").exists())
            self.assertFalse((incoming_dir / "bad_bundle.json").exists())
            self.assertTrue((failed_dir / "bad_json.json").exists())
            self.assertTrue((failed_dir / "bad_bundle.json").exists())


if __name__ == "__main__":
    unittest.main()
