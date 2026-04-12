from __future__ import annotations

import json
from pathlib import Path

from content_hub.domain.formatting.models import ReviewTask


class FileReviewTaskRepository:
    def __init__(self, path: Path):
        self.path = path

    def save(self, task: ReviewTask) -> ReviewTask:
        payload = self._list_payload()
        remaining = [item for item in payload if item["review_id"] != task.review_id]
        remaining.append(self._to_payload(task))
        self.path.parent.mkdir(parents=True, exist_ok=True)
        self.path.write_text(json.dumps(remaining, ensure_ascii=False, indent=2), encoding="utf-8")
        return task

    def get(self, review_id: str) -> ReviewTask | None:
        for item in self._list_payload():
            if item["review_id"] == review_id:
                return self._from_payload(item)
        return None

    def list_tasks(self, article_id: str | None = None, status: str | None = None) -> list[ReviewTask]:
        tasks = [self._from_payload(item) for item in self._list_payload()]
        if article_id is not None:
            tasks = [item for item in tasks if item.article_id == article_id]
        if status is not None:
            tasks = [item for item in tasks if item.status == status]
        return tasks

    def _list_payload(self) -> list[dict]:
        if not self.path.exists():
            return []
        return json.loads(self.path.read_text(encoding="utf-8"))

    def _to_payload(self, task: ReviewTask) -> dict:
        return {
            "review_id": task.review_id,
            "article_id": task.article_id,
            "asset_ids": list(task.asset_ids),
            "status": task.status,
            "reviewer": task.reviewer,
            "notes": task.notes,
        }

    def _from_payload(self, payload: dict) -> ReviewTask:
        return ReviewTask(
            review_id=payload["review_id"],
            article_id=payload["article_id"],
            asset_ids=list(payload.get("asset_ids", [])),
            status=payload["status"],
            reviewer=payload.get("reviewer", ""),
            notes=payload.get("notes", ""),
        )
