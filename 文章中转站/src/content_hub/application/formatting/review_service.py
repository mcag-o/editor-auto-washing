from __future__ import annotations

import uuid

from content_hub.domain.formatting.models import ReviewTask
from content_hub.infrastructure.storage.review_task_repository import FileReviewTaskRepository


class ReviewService:
    def __init__(self, repository: FileReviewTaskRepository):
        self.repository = repository

    def create_review(
        self,
        article_id: str,
        asset_ids: list[str],
        reviewer: str = "",
        notes: str = "",
    ) -> ReviewTask:
        task = ReviewTask(
            review_id=f"review-{uuid.uuid4().hex[:12]}",
            article_id=article_id,
            asset_ids=asset_ids,
            status="review_pending",
            reviewer=reviewer,
            notes=notes,
        )
        return self.repository.save(task)

    def get_review(self, review_id: str) -> ReviewTask | None:
        return self.repository.get(review_id)

    def list_reviews(self, article_id: str | None = None, status: str | None = None) -> list[ReviewTask]:
        return self.repository.list_tasks(article_id=article_id, status=status)

    def approve_review(self, review_id: str, reviewer: str = "", notes: str = "") -> ReviewTask:
        task = self._require_review(review_id)
        task.status = "approved"
        if reviewer:
            task.reviewer = reviewer
        if notes:
            task.notes = notes
        return self.repository.save(task)

    def reject_review(self, review_id: str, reviewer: str = "", notes: str = "") -> ReviewTask:
        task = self._require_review(review_id)
        task.status = "review_rejected"
        if reviewer:
            task.reviewer = reviewer
        if notes:
            task.notes = notes
        return self.repository.save(task)

    def _require_review(self, review_id: str) -> ReviewTask:
        task = self.repository.get(review_id)
        if task is None:
            raise KeyError(f"review not found: {review_id}")
        return task
