from __future__ import annotations

from copy import deepcopy
from datetime import UTC, datetime
from html import escape

from content_hub.domain.formatting.models import ArticleDraft


def normalize_text(value: object | None, allow_html: bool = False) -> str:
    text = "" if value is None else str(value).strip()
    return text if allow_html else escape(text)


def normalize_paragraphs(value: object | None) -> list[str]:
    if value is None:
        return []
    if isinstance(value, str):
        parts = [item.strip() for item in value.split("\n\n") if item.strip()]
        return parts if parts else ([value.strip()] if value.strip() else [])
    if isinstance(value, list):
        return [str(item).strip() for item in value if str(item).strip()]
    raise ValueError(f"Expected string or list of strings, got {type(value).__name__}")


def count_sources(article: ArticleDraft) -> int:
    sources: set[str] = set()
    headline = article.headline or {}
    if headline.get("source"):
        sources.add(str(headline["source"]).strip())
    for section in article.sections or []:
        for block in section.get("blocks", []):
            if block.get("source"):
                sources.add(str(block["source"]).strip())
    return len({item for item in sources if item})


def count_news_items(article: ArticleDraft) -> int:
    count = 0
    for section in article.sections or []:
        for block in section.get("blocks", []):
            if block.get("type", "card") in {"card", "opinion", "week-ahead"}:
                count += 1
    return count


def ensure_article_defaults(article: ArticleDraft) -> ArticleDraft:
    normalized = deepcopy(article)
    meta = normalized.meta if isinstance(normalized.meta, dict) else {}
    normalized.meta = meta

    now = _parse_iso_date(meta.get("date"))
    if meta.get("date") in {None, ""}:
        meta["date"] = now.strftime("%Y-%m-%d")
    if meta.get("date_cn") in {None, ""}:
        meta["date_cn"] = f"{now.year} 年 {now.month} 月 {now.day} 日"
    if meta.get("date_short") in {None, ""}:
        meta["date_short"] = now.strftime("%Y.%m.%d")
    if meta.get("author") in {None, ""}:
        meta["author"] = "39Claw"
    if meta.get("open_comment") is None:
        meta["open_comment"] = 1
    if meta.get("source_count") is None:
        meta["source_count"] = count_sources(normalized)
    if meta.get("news_count") is None:
        meta["news_count"] = count_news_items(normalized)

    return normalized


def _parse_iso_date(raw: object | None) -> datetime:
    if raw in {None, ""}:
        now = datetime.now(UTC)
        return datetime(now.year, now.month, now.day, tzinfo=UTC)
    text = str(raw)
    return datetime.strptime(text, "%Y-%m-%d").replace(tzinfo=UTC)
