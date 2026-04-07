from __future__ import annotations

from dataclasses import dataclass, field


@dataclass
class WorkspaceArticle:
    article_id: str
    title: str
    status: str = "draft"
    status_history: list[str] = field(default_factory=lambda: ["draft"])

    _transitions: dict[str, set[str]] = field(
        default_factory=lambda: {
        "draft": {"rendered"},
        "rendered": {"review_pending"},
        "review_pending": {"approved", "review_rejected"},
        "approved": {"published"},
        "review_rejected": {"draft"},
        "published": set(),
        },
        init=False,
        repr=False,
    )

    def transition_to(self, next_status: str) -> None:
        allowed = self._transitions.get(self.status, set())
        if next_status not in allowed:
            raise ValueError(
                f"invalid article status transition: {self.status} -> {next_status}"
            )
        self.status = next_status
        self.status_history.append(next_status)
