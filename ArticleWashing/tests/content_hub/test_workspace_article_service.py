from __future__ import annotations

import tempfile
import unittest
from pathlib import Path

from content_hub.application.workspace.article_service import WorkspaceArticleService
from content_hub.domain.workspace import WorkspaceArticle
from content_hub.infrastructure.storage.workspace_article_repository import (
    FileWorkspaceArticleRepository,
)


class WorkspaceArticleServiceTestCase(unittest.TestCase):
    def test_workspace_article_defaults_to_draft_status(self) -> None:
        article = WorkspaceArticle(article_id="article-001", title="Daily Brief")

        self.assertIsInstance(article, WorkspaceArticle)
        self.assertEqual(article.article_id, "article-001")
        self.assertEqual(article.title, "Daily Brief")
        self.assertEqual(article.status, "draft")
        self.assertEqual(article.status_history, ["draft"])

    def test_transition_follows_valid_review_workflow_path(self) -> None:
        article = WorkspaceArticle(article_id="article-001", title="Daily Brief")

        article.transition_to("rendered")
        article.transition_to("review_pending")
        article.transition_to("approved")
        article.transition_to("published")

        self.assertEqual(article.status, "published")
        self.assertEqual(
            article.status_history,
            ["draft", "rendered", "review_pending", "approved", "published"],
        )

    def test_transition_allows_revision_cycle_after_rejection(self) -> None:
        article = WorkspaceArticle(article_id="article-001", title="Daily Brief")

        article.transition_to("rendered")
        article.transition_to("review_pending")
        article.transition_to("review_rejected")
        article.transition_to("draft")

        self.assertEqual(article.status, "draft")
        self.assertEqual(
            article.status_history,
            ["draft", "rendered", "review_pending", "review_rejected", "draft"],
        )

    def test_transition_rejects_invalid_status_change(self) -> None:
        article = WorkspaceArticle(article_id="article-001", title="Daily Brief")

        with self.assertRaises(ValueError) as context:
            article.transition_to("published")

        self.assertIn("invalid article status transition", str(context.exception))
        self.assertEqual(article.status, "draft")
        self.assertEqual(article.status_history, ["draft"])

    def test_repository_saves_and_gets_article(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            repository = FileWorkspaceArticleRepository(Path(tmp_dir) / "workspace_articles.json")
            article = WorkspaceArticle(article_id="article-001", title="Daily Brief")
            article.transition_to("rendered")

            repository.save(article)
            loaded = repository.get("article-001")

            self.assertIsNotNone(loaded)
            self.assertEqual(loaded.article_id, "article-001")
            self.assertEqual(loaded.title, "Daily Brief")
            self.assertEqual(loaded.status, "rendered")
            self.assertEqual(loaded.status_history, ["draft", "rendered"])

    def test_repository_lists_articles_with_optional_status_filter(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            repository = FileWorkspaceArticleRepository(Path(tmp_dir) / "workspace_articles.json")
            draft_article = WorkspaceArticle(article_id="article-001", title="Daily Brief")
            review_article = WorkspaceArticle(article_id="article-002", title="Weekly Review")
            review_article.transition_to("rendered")

            repository.save(draft_article)
            repository.save(review_article)

            all_articles = repository.list_articles()
            rendered_articles = repository.list_articles(status="rendered")

            self.assertEqual(len(all_articles), 2)
            self.assertEqual(len(rendered_articles), 1)
            self.assertEqual(rendered_articles[0].article_id, "article-002")

    def test_service_creates_and_transitions_article(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            repository = FileWorkspaceArticleRepository(Path(tmp_dir) / "workspace_articles.json")
            service = WorkspaceArticleService(repository)

            created = service.create_article(article_id="article-001", title="Daily Brief")
            transitioned = service.transition_article("article-001", "rendered")
            loaded = service.get_article("article-001")

            self.assertEqual(created.status, "draft")
            self.assertEqual(transitioned.status, "rendered")
            self.assertIsNotNone(loaded)
            self.assertEqual(loaded.status, "rendered")

    def test_service_raises_key_error_for_missing_article_transition(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            repository = FileWorkspaceArticleRepository(Path(tmp_dir) / "workspace_articles.json")
            service = WorkspaceArticleService(repository)

            with self.assertRaises(KeyError) as context:
                service.transition_article("missing", "rendered")

            self.assertIn("workspace article not found", str(context.exception))


if __name__ == "__main__":
    unittest.main()
