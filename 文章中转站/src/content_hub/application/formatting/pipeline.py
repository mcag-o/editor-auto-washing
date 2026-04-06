from __future__ import annotations

import json
from pathlib import Path

from content_hub.application.formatting.image_plan import attach_missing_image_plans
from content_hub.domain.formatting.models import ArticleDraft, FormatTarget


def run_formatting_pipeline(
    article: ArticleDraft,
    formatter,
    output_dir: Path,
    dry_run: bool = False,
    skip_plan_images: bool = False,
    max_content_images: int = 3,
) -> dict:
    output_dir.mkdir(parents=True, exist_ok=True)
    resolved = article if skip_plan_images else attach_missing_image_plans(article, output_dir=str(output_dir / "images"), max_content_images=max_content_images)
    asset = formatter.render(resolved, FormatTarget(platform="wechat", template=resolved.template, output_format="html"))
    html_path = output_dir / f"{resolved.article_id}.html"
    resolved_path = output_dir / f"{resolved.article_id}.resolved.json"
    if not dry_run:
        html_path.write_text(asset.content, encoding="utf-8")
        resolved_path.write_text(json.dumps(_draft_to_payload(resolved), ensure_ascii=False, indent=2), encoding="utf-8")
    return {"html": str(html_path), "resolved_article": str(resolved_path)}


def _draft_to_payload(article: ArticleDraft) -> dict:
    return {
        "article_id": article.article_id,
        "template": article.template,
        "meta": article.meta,
        "headline": article.headline,
        "sections": article.sections,
        "conclusion": article.conclusion,
        "cta": article.cta,
        "source_refs": article.source_refs,
        "target_platforms": article.target_platforms,
        "status": article.status,
    }
