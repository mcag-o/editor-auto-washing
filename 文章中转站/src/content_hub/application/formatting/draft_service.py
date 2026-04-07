from __future__ import annotations

import uuid
from typing import Any

from content_hub.domain.formatting.models import ArticleDraft
from content_hub.infrastructure.storage.article_draft_repository import FileArticleDraftRepository


class DraftService:
    _PROFILE_META_KEY = "_workspace_profiles"

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
        provider_profile: str = "",
        article_profile: str = "",
        publish_profile: str = "",
    ) -> ArticleDraft:
        normalized_meta = dict(meta)
        normalized_meta[self._PROFILE_META_KEY] = {
            "provider_profile": provider_profile,
            "article_profile": article_profile,
            "publish_profile": publish_profile,
        }
        draft = ArticleDraft(
            article_id=f"article-{uuid.uuid4().hex[:12]}",
            template=template,
            meta=normalized_meta,
            headline=headline,
            sections=sections,
            conclusion=conclusion,
            cta=cta,
            source_refs=source_refs or [],
            target_platforms=target_platforms or [],
            provider_profile=provider_profile,
            article_profile=article_profile,
            publish_profile=publish_profile,
        )
        return self.repository.save(draft)

    def get_draft(self, article_id: str) -> ArticleDraft | None:
        draft = self.repository.get(article_id)
        if draft is None:
            return None
        return self._with_profiles_from_meta(draft)

    def list_drafts(self, status: str | None = None) -> list[ArticleDraft]:
        drafts = self.repository.list_drafts(status=status)
        return [self._with_profiles_from_meta(draft) for draft in drafts]

    def _with_profiles_from_meta(self, draft: ArticleDraft) -> ArticleDraft:
        profile_payload = draft.meta.get(self._PROFILE_META_KEY)
        if not isinstance(profile_payload, dict):
            return draft

        if not draft.provider_profile:
            draft.provider_profile = self._safe_profile_value(profile_payload, "provider_profile")
        if not draft.article_profile:
            draft.article_profile = self._safe_profile_value(profile_payload, "article_profile")
        if not draft.publish_profile:
            draft.publish_profile = self._safe_profile_value(profile_payload, "publish_profile")
        return draft

    def _safe_profile_value(self, profile_payload: dict[str, Any], key: str) -> str:
        value = profile_payload.get(key, "")
        return value if isinstance(value, str) else ""
