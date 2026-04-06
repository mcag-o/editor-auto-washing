from __future__ import annotations

import json
from pathlib import Path

from content_hub.domain.formatting.models import ArticleDraft


class FileArticleDraftRepository:
    def __init__(self, path: Path):
        self.path = path

    def save(self, draft: ArticleDraft) -> ArticleDraft:
        payload = self._list_payload()
        remaining = [item for item in payload if item["article_id"] != draft.article_id]
        remaining.append(self._to_payload(draft))
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.path.write_text(json.dumps(remaining, ensure_ascii=False, indent=2), encoding="utf-8")
        return draft

    def get(self, article_id: str) -> ArticleDraft | None:
        for item in self._list_payload():
            if item["article_id"] == article_id:
                return self._from_payload(item)
        return None

    def list_drafts(self, status: str | None = None) -> list[ArticleDraft]:
        drafts = [self._from_payload(item) for item in self._list_payload()]
        if status is not None:
            drafts = [item for item in drafts if item.status == status]
        return drafts

    def _list_payload(self) -> list[dict]:
        if not self.path.exists():
            return []
        return json.loads(self.path.read_text(encoding="utf-8"))

    def _to_payload(self, draft: ArticleDraft) -> dict:
        return {
            "article_id": draft.article_id,
            "template": draft.template,
            "meta": draft.meta,
            "headline": draft.headline,
            "sections": draft.sections,
            "conclusion": draft.conclusion,
            "cta": draft.cta,
            "source_refs": draft.source_refs,
            "target_platforms": draft.target_platforms,
            "status": draft.status,
        }

    def _from_payload(self, payload: dict) -> ArticleDraft:
        return ArticleDraft(
            article_id=payload["article_id"],
            template=payload["template"],
            meta=payload.get("meta", {}),
            headline=payload.get("headline", {}),
            sections=payload.get("sections", []),
            conclusion=payload.get("conclusion", ""),
            cta=payload.get("cta", ""),
            source_refs=payload.get("source_refs", []),
            target_platforms=payload.get("target_platforms", []),
            status=payload.get("status", "draft"),
        )
