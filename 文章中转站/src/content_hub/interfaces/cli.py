from __future__ import annotations

import argparse
import json
from pathlib import Path

from content_hub.application.formatting.pipeline import run_formatting_pipeline
from content_hub.domain.formatting.models import ArticleDraft, FormatTarget
from content_hub.infrastructure.formatters.template_catalog import FileTemplateCatalog
from content_hub.infrastructure.formatters.wechat_html_formatter import WechatHtmlFormatter


def load_article(path: Path) -> ArticleDraft:
    payload = json.loads(path.read_text(encoding="utf-8"))
    return ArticleDraft(
        article_id=payload.get("article_id", path.stem),
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


def create_formatter(template_root: Path) -> WechatHtmlFormatter:
    return WechatHtmlFormatter(FileTemplateCatalog(template_root))


def handle_render(args: argparse.Namespace) -> int:
    article = load_article(Path(args.input))
    formatter = create_formatter(Path(args.template_root))
    asset = formatter.render(article, FormatTarget(platform="wechat", template=article.template, output_format="html"))
    if args.check:
        errors = formatter.validate(article, FormatTarget(platform="wechat", template=article.template, output_format="html"))
        errors.extend(formatter.validate_rendered_output(asset.content))
        if errors:
            print(json.dumps({"ok": False, "errors": errors}, ensure_ascii=False, indent=2))
            return 1
    if args.output:
        output_path = Path(args.output)
        output_path.parent.mkdir(parents=True, exist_ok=True)
        output_path.write_text(asset.content, encoding="utf-8")
        print(output_path)
    else:
        print(asset.content)
    return 0


def handle_validate(args: argparse.Namespace) -> int:
    article = load_article(Path(args.input))
    formatter = create_formatter(Path(args.template_root))
    errors = formatter.validate(article, FormatTarget(platform="wechat", template=article.template, output_format="html"))
    result = {"ok": len(errors) == 0, "errors": errors, "warnings": formatter.last_warnings}
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0 if result["ok"] else 1


def handle_pipeline(args: argparse.Namespace) -> int:
    article = load_article(Path(args.input))
    formatter = create_formatter(Path(args.template_root))
    summary = run_formatting_pipeline(
        article,
        formatter=formatter,
        output_dir=Path(args.output_dir),
        dry_run=bool(args.dry_run),
    )
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="content_hub_cli")
    subparsers = parser.add_subparsers(dest="command", required=True)

    render_parser = subparsers.add_parser("render")
    render_parser.add_argument("input")
    render_parser.add_argument("-o", "--output")
    render_parser.add_argument("--check", action="store_true")
    render_parser.add_argument("--template-root", default="文章中转站/knowledge/structured_templates")
    render_parser.set_defaults(handler=handle_render)

    validate_parser = subparsers.add_parser("validate")
    validate_parser.add_argument("input")
    validate_parser.add_argument("--template-root", default="文章中转站/knowledge/structured_templates")
    validate_parser.set_defaults(handler=handle_validate)

    pipeline_parser = subparsers.add_parser("pipeline")
    pipeline_parser.add_argument("input")
    pipeline_parser.add_argument("--output-dir", default="build")
    pipeline_parser.add_argument("--dry-run", action="store_true")
    pipeline_parser.add_argument("--template-root", default="文章中转站/knowledge/structured_templates")
    pipeline_parser.set_defaults(handler=handle_pipeline)

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    return args.handler(args)


if __name__ == "__main__":
    raise SystemExit(main())
