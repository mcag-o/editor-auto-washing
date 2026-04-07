from __future__ import annotations

from content_hub.domain.workspace import WorkspaceArticle
from content_hub.infrastructure.storage.workspace_article_repository import (
    FileWorkspaceArticleRepository,
)


class WorkspaceArticleService:
    def __init__(self, repository: FileWorkspaceArticleRepository):
        self.repository = repository

    def create_article(self, article_id: str, title: str) -> WorkspaceArticle:
        article = WorkspaceArticle(article_id=article_id, title=title)
        return self.repository.save(article)

    def get_article(self, article_id: str) -> WorkspaceArticle | None:
        return self.repository.get(article_id)

    def list_articles(self, status: str | None = None) -> list[WorkspaceArticle]:
        return self.repository.list_articles(status=status)

    def transition_article(self, article_id: str, next_status: str) -> WorkspaceArticle:
        article = self.repository.get(article_id)
        if article is None:
            raise KeyError(f"workspace article not found: {article_id}")
        article.transition_to(next_status)
        return self.repository.save(article)
