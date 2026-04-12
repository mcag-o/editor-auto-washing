from __future__ import annotations

import json
from pathlib import Path

from content_hub.domain.workspace import WorkspaceArticle


class FileWorkspaceArticleRepository:
    def __init__(self, path: Path):
        self.path = path

    def save(self, article: WorkspaceArticle) -> WorkspaceArticle:
        payload = self._list_payload()
        remaining = [item for item in payload if item["article_id"] != article.article_id]
        remaining.append(self._to_payload(article))
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.path.write_text(
            json.dumps(remaining, ensure_ascii=False, indent=2),
            encoding="utf-8",
        )
        return article

    def get(self, article_id: str) -> WorkspaceArticle | None:
        for item in self._list_payload():
            if item["article_id"] == article_id:
                return self._from_payload(item)
        return None

    def list_articles(self, status: str | None = None) -> list[WorkspaceArticle]:
        articles = [self._from_payload(item) for item in self._list_payload()]
        if status is not None:
            articles = [item for item in articles if item.status == status]
        return articles

    def _list_payload(self) -> list[dict]:
        if not self.path.exists():
            return []
        return json.loads(self.path.read_text(encoding="utf-8"))

    def _to_payload(self, article: WorkspaceArticle) -> dict:
        return {
            "article_id": article.article_id,
            "title": article.title,
            "status": article.status,
            "status_history": list(article.status_history),
        }

    def _from_payload(self, payload: dict) -> WorkspaceArticle:
        return WorkspaceArticle(
            article_id=payload["article_id"],
            title=payload["title"],
            status=payload.get("status", "draft"),
            status_history=list(payload.get("status_history", ["draft"])),
        )
