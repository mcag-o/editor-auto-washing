from __future__ import annotations

from copy import deepcopy

from content_hub.application.formatting.article_normalizer import ensure_article_defaults, normalize_paragraphs
from content_hub.domain.formatting.models import ArticleDraft


def _safe_fragment(value: str) -> str:
    cleaned = "".join(char.lower() if char.isalnum() else "-" for char in str(value))
    normalized = "-".join(part for part in cleaned.split("-") if part)
    return normalized or "image"


def _cover_prompt(article: ArticleDraft) -> str:
    meta = ensure_article_defaults(article).meta
    title = meta.get("title") or "微信公众号封面"
    date_short = meta.get("date_short")
    template = article.template
    if template == "daily-intelligence":
        return f"Futuristic AI daily news cover for {date_short}. Theme: {title}. Dark blue gradient, neural network nodes, holographic interface, editorial magazine cover, 16:9."
    if template == "weekly-financial":
        return f"Dramatic financial weekly cover for {date_short}. Theme: {title}. Dark red and black gradient, stock charts, commodities, Bloomberg or Economist style, 16:9."
    return f"Editorial deep analysis cover for {date_short}. Theme: {title}. Deep navy palette, analytical charts, serious longform magazine style, cinematic light, 16:9."


def _content_prompt(article: ArticleDraft, title: str, detail: str) -> str:
    if article.template == "daily-intelligence":
        return f"Futuristic AI news illustration about {title}. {detail}. Modern research lab, holographic data panels, blue and white palette, professional editorial image, 16:9."
    if article.template == "weekly-financial":
        return f"Financial news illustration about {title}. {detail}. Institutional market screens, macroeconomic tension, professional media photography style, 16:9."
    return f"Editorial analysis illustration about {title}. {detail}. Longform magazine style, layered charts and symbolic objects, restrained dramatic lighting, 16:9."


def _section_detail(section: dict, block: dict) -> str:
    text_bits: list[str] = []
    if section.get("cn"):
        text_bits.append(str(section["cn"]))
    if block.get("body"):
        paragraphs = normalize_paragraphs(block.get("body"))
        if paragraphs:
            text_bits.append(paragraphs[0][:120])
    if not text_bits and isinstance(block.get("days"), list):
        labels = [str(row.get("label", "")).strip() for row in block.get("days", [])[:2] if str(row.get("label", "")).strip()]
        text_bits.append(" / ".join(labels))
    return ". ".join(bit for bit in text_bits if bit)


def _choose_section_subject(section: dict) -> dict | None:
    for block in section.get("blocks", []):
        if block.get("type", "card") in {"card", "opinion", "week-ahead"}:
            return block
    return None


def attach_missing_image_plans(article: ArticleDraft, output_dir: str, max_content_images: int = 3) -> ArticleDraft:
    updated = deepcopy(article)
    updated = ensure_article_defaults(updated)
    meta = updated.meta
    date_short = str(meta["date_short"]).replace(".", "-")

    cover = dict(meta.get("cover_image") or {})
    if not cover.get("prompt"):
        cover["prompt"] = _cover_prompt(updated)
    if not cover.get("local_path"):
        cover["local_path"] = f"{output_dir}/cover-{date_short}.png"
    meta["cover_image"] = cover

    plans = [{"target": "cover", **cover}]
    headline = updated.headline or {}
    planned_slots = 0
    if max_content_images > 0:
        headline_image = dict(headline.get("image") or {})
        if not headline_image.get("prompt"):
            headline_paragraphs = normalize_paragraphs(headline.get("body"))
            headline_image["prompt"] = _content_prompt(
                updated,
                str(headline.get("title") or meta.get("title") or "头条"),
                headline_paragraphs[0][:120] if headline_paragraphs else "",
            )
        if not headline_image.get("caption"):
            headline_image["caption"] = str(headline.get("title") or "头条配图")
        if not headline_image.get("local_path"):
            headline_image["local_path"] = f"{output_dir}/headline-{date_short}.png"
        headline["image"] = headline_image
        updated.headline = headline
        plans.append({"target": "headline", **headline_image})
        planned_slots += 1

    section_index = 0
    while planned_slots < max_content_images and section_index < len(updated.sections or []):
        section = updated.sections[section_index]
        section_index += 1
        if section.get("image"):
            continue
        subject = _choose_section_subject(section)
        if subject is None:
            continue
        title = str(subject.get("title") or section.get("cn") or "正文配图")
        section["image"] = {
            "prompt": _content_prompt(updated, title, _section_detail(section, subject)),
            "caption": title,
            "local_path": f"{output_dir}/section-{str(planned_slots).zfill(2)}-{_safe_fragment(title)[:24]}.png",
        }
        plans.append({"target": f"section[{section_index - 1}]", **section["image"]})
        planned_slots += 1

    meta["image_plans"] = plans
    return updated
