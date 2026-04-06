from __future__ import annotations

import uuid

from content_hub.domain.formatting.models import ArticleDraft
from content_hub.infrastructure.storage.article_draft_repository import FileArticleDraftRepository


class DraftService:
    def __init__(self, repository: FileArticleDraftRepository):
        self.repository = repository

    def create_draft(
        self,
        template: str,
        meta: dict,
        headline: dict,
        sections: list[dict],
        conclusion: str,
        cta: str,
        source_refs: list[dict] | None = None,
        target_platforms: list[str] | None = None,
    ) -> ArticleDraft:
        draft = ArticleDraft(
            article_id=f"article-{uuid.uuid4().hex[:12]}",
            template=template,
            meta=meta,
            headline=headline,
            sections=sections,
            conclusion=conclusion,
            cta=cta,
            source_refs=source_refs or [],
            target_platforms=target_platforms or [],
        )
        return self.repository.save(draft)

    def get_draft(self, article_id: str) -> ArticleDraft | None:
        return self.repository.get(article_id)

    def list_drafts(self, status: str | None = None) -> list[ArticleDraft]:
        return self.repository.list_drafts(status=status)
