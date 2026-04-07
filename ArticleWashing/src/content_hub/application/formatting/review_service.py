from __future__ import annotations

import uuid

from content_hub.domain.formatting.models import ReviewTask
from content_hub.infrastructure.storage.review_task_repository import FileReviewTaskRepository


class ReviewService:
    _PROFILE_NOTES_PREFIX = "[[publish_profile:"
    _PROFILE_NOTES_SUFFIX = "]]"

    def __init__(self, repository: FileReviewTaskRepository):
        self.repository = repository

    def create_review(
        self,
        article_id: str,
        asset_ids: list[str],
        publish_profile: str = "",
        reviewer: str = "",
        notes: str = "",
    ) -> ReviewTask:
        task = ReviewTask(
            review_id=f"review-{uuid.uuid4().hex[:12]}",
            article_id=article_id,
            asset_ids=asset_ids,
            status="review_pending",
            publish_profile=publish_profile,
            reviewer=reviewer,
            notes=notes,
        )
        return self._save_with_profile(task)

    def get_review(self, review_id: str) -> ReviewTask | None:
        task = self.repository.get(review_id)
        if task is None:
            return None
        return self._with_profile_from_notes(task)

    def list_reviews(self, article_id: str | None = None, status: str | None = None) -> list[ReviewTask]:
        tasks = self.repository.list_tasks(article_id=article_id, status=status)
        return [self._with_profile_from_notes(task) for task in tasks]

    def approve_review(self, review_id: str, reviewer: str = "", notes: str = "") -> ReviewTask:
        task = self._require_review(review_id)
        task.status = "approved"
        if reviewer:
            task.reviewer = reviewer
        if notes:
            task.notes = notes
        return self._save_with_profile(task)

    def reject_review(self, review_id: str, reviewer: str = "", notes: str = "") -> ReviewTask:
        task = self._require_review(review_id)
        task.status = "review_rejected"
        if reviewer:
            task.reviewer = reviewer
        if notes:
            task.notes = notes
        return self._save_with_profile(task)

    def _require_review(self, review_id: str) -> ReviewTask:
        task = self.repository.get(review_id)
        if task is None:
            raise KeyError(f"review not found: {review_id}")
        return self._with_profile_from_notes(task)

    def _save_with_profile(self, task: ReviewTask) -> ReviewTask:
        encoded_notes = self._encode_notes(task.notes, task.publish_profile)
        task.notes = encoded_notes
        saved = self.repository.save(task)
        return self._with_profile_from_notes(saved)

    def _with_profile_from_notes(self, task: ReviewTask) -> ReviewTask:
        publish_profile, clean_notes = self._decode_notes(task.notes)
        if not task.publish_profile:
            task.publish_profile = publish_profile
        task.notes = clean_notes
        return task

    def _encode_notes(self, notes: str, publish_profile: str) -> str:
        if not publish_profile:
            return notes
        clean_notes = notes
        _, existing_clean_notes = self._decode_notes(notes)
        clean_notes = existing_clean_notes
        return f"{self._PROFILE_NOTES_PREFIX}{publish_profile}{self._PROFILE_NOTES_SUFFIX}{clean_notes}"

    def _decode_notes(self, notes: str) -> tuple[str, str]:
        if not notes.startswith(self._PROFILE_NOTES_PREFIX):
            return "", notes
        marker_end = notes.find(self._PROFILE_NOTES_SUFFIX)
        if marker_end == -1:
            return "", notes
        profile_start = len(self._PROFILE_NOTES_PREFIX)
        publish_profile = notes[profile_start:marker_end]
        clean_notes = notes[marker_end + len(self._PROFILE_NOTES_SUFFIX) :]
        return publish_profile, clean_notes
