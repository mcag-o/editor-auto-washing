from __future__ import annotations

import json
import shutil
from pathlib import Path

from content_hub.application.workspace.article_service import WorkspaceArticleService
from content_hub.infrastructure.storage.ingestion_repository import FileHotTopicIngestionRepository
from content_hub.infrastructure.storage.ingestion_repository import FileRawContentIngestionRepository
from content_hub.infrastructure.storage.ingestion_repository import FileReferenceIngestionRepository


class IngestionService:
    _AUTOMATION_METADATA_FILENAMES = {
        "automation_state.json",
        "automation_alert.json",
    }

    def __init__(
        self,
        reference_repository: FileReferenceIngestionRepository,
        raw_content_repository: FileRawContentIngestionRepository | None = None,
        hot_topic_repository: FileHotTopicIngestionRepository | None = None,
        workspace_article_service: WorkspaceArticleService | None = None,
    ):
        self.reference_repository = reference_repository
        self.raw_content_repository = raw_content_repository
        self.hot_topic_repository = hot_topic_repository
        self.workspace_article_service = workspace_article_service

    def submit_reference_urls(self, urls: list[str]) -> dict:
        self.reference_repository.add_urls(urls)
        return {"submitted": len(urls), "items": self.reference_repository.list_urls()[-len(urls):]}

    def submit_raw_content(self, items: list[dict]) -> dict:
        if self.raw_content_repository is None:
            raise ValueError("raw content repository is not configured")
        self.raw_content_repository.add_items(items)
        return {"submitted": len(items), "items": self.raw_content_repository.list_items()[-len(items):]}

    def submit_hot_topics(self, items: list[dict]) -> dict:
        if self.hot_topic_repository is None:
            raise ValueError("hot topic repository is not configured")
        self.hot_topic_repository.add_items(items)
        return {"submitted": len(items), "items": self.hot_topic_repository.list_items()[-len(items):]}

    def import_content_hub_bundle(
        self,
        bundle: dict,
        provider_profile: str,
        article_profile: str,
        publish_profile: str,
    ) -> dict:
        if self.hot_topic_repository is None:
            raise ValueError("hot topic repository is not configured")

        _ = (provider_profile, article_profile, publish_profile)
        items = bundle.get("items") if isinstance(bundle, dict) else None
        if not isinstance(items, list):
            raise ValueError("bundle must include a list field: items")

        self.hot_topic_repository.add_items(items)

        created_article_ids: list[str] = []
        skip_reasons: list[dict] = []
        for index, item in enumerate(items):
            if not isinstance(item, dict):
                skip_reasons.append({"index": index, "reason": "bundle item is not an object"})
                continue

            if self.workspace_article_service is None:
                skip_reasons.append(
                    {"index": index, "reason": "workspace article service is not configured"}
                )
                continue

            article_id = str(item.get("url") or f"bundle-item-{index}")
            title = str(item.get("title") or article_id)
            article = self.workspace_article_service.create_article(article_id=article_id, title=title)
            created_article_ids.append(article.article_id)

        return {
            "imported_count": len(items),
            "created_article_ids": created_article_ids,
            "skipped_count": len(skip_reasons),
            "skip_reasons": skip_reasons,
        }

    def import_content_hub_bundles_from_directory(
        self,
        incoming_dir: Path | str,
        provider_profile: str,
        article_profile: str,
        publish_profile: str,
        archive_dir: Path | str | None = None,
        failed_dir: Path | str | None = None,
    ) -> dict:
        incoming_path = Path(incoming_dir)
        archive_path = Path(archive_dir) if archive_dir is not None else None
        failed_path = Path(failed_dir) if failed_dir is not None else None
        json_files = sorted(
            file_path
            for file_path in incoming_path.glob("*.json")
            if file_path.name not in self._AUTOMATION_METADATA_FILENAMES
        )

        imported_files = 0
        failed_files = 0
        total_imported_items = 0
        total_created_articles = 0
        file_results: list[dict] = []

        for file_path in json_files:
            try:
                bundle = json.loads(file_path.read_text(encoding="utf-8"))
                bundle_result = self.import_content_hub_bundle(
                    bundle=bundle,
                    provider_profile=provider_profile,
                    article_profile=article_profile,
                    publish_profile=publish_profile,
                )
            except Exception as error:
                failed_files += 1
                if failed_path is not None:
                    failed_path.mkdir(parents=True, exist_ok=True)
                    shutil.move(str(file_path), str(failed_path / file_path.name))
                    file_result: dict[str, object] = {
                        "file_name": file_path.name,
                        "status": "failed",
                        "error": str(error),
                        "failed_moved": True,
                    }
                else:
                    file_result = {
                        "file_name": file_path.name,
                        "status": "failed",
                        "error": str(error),
                    }

                file_results.append(file_result)
                continue

            imported_files += 1
            imported_count = int(bundle_result.get("imported_count", 0))
            created_article_ids = bundle_result.get("created_article_ids", [])
            created_article_count = (
                len(created_article_ids) if isinstance(created_article_ids, list) else 0
            )
            total_imported_items += imported_count
            total_created_articles += created_article_count

            if archive_path is not None:
                archive_path.mkdir(parents=True, exist_ok=True)
                shutil.move(str(file_path), str(archive_path / file_path.name))
                file_result: dict[str, object] = {
                    "file_name": file_path.name,
                    "status": "imported",
                    "imported_items": imported_count,
                    "created_articles": created_article_count,
                    "bundle_result": bundle_result,
                    "archived": True,
                }
            else:
                file_result = {
                    "file_name": file_path.name,
                    "status": "imported",
                    "imported_items": imported_count,
                    "created_articles": created_article_count,
                    "bundle_result": bundle_result,
                }

            file_results.append(file_result)

        return {
            "scanned_files": len(json_files),
            "imported_files": imported_files,
            "failed_files": failed_files,
            "total_imported_items": total_imported_items,
            "total_created_articles": total_created_articles,
            "file_results": file_results,
        }

    def list_records(self) -> dict:
        return {
            "reference_urls": self.reference_repository.list_urls(),
            "raw_content": self.raw_content_repository.list_items() if self.raw_content_repository is not None else [],
            "hot_topics": self.hot_topic_repository.list_items() if self.hot_topic_repository is not None else [],
        }
