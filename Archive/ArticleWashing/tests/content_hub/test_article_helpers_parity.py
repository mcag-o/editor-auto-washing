from pathlib import Path
import tempfile
import unittest

from content_hub.application.formatting.article_normalizer import count_news_items, count_sources, ensure_article_defaults, normalize_paragraphs
from content_hub.application.formatting.pipeline import run_formatting_pipeline
from content_hub.domain.formatting.models import ArticleDraft
from content_hub.infrastructure.formatters.template_catalog import FileTemplateCatalog
from content_hub.infrastructure.formatters.wechat_html_formatter import WechatHtmlFormatter


class ArticleHelpersParityTestCase(unittest.TestCase):
    def test_ensure_article_defaults_fills_expected_meta_defaults(self) -> None:
        article = ArticleDraft(
            article_id="article-1",
            template="daily-intelligence",
            meta={"title": "T", "digest": "D", "date": "2026-03-31"},
            headline={"title": "H", "body": ["B"], "source": "Headline Source"},
            sections=[
                {
                    "cn": "要闻",
                    "en": "BRIEFING",
                    "blocks": [{"type": "card", "title": "x", "body": ["y"], "source": "Section Source"}],
                }
            ],
            conclusion="",
            cta="",
            source_refs=[],
            target_platforms=["wechat"],
        )

        normalized = ensure_article_defaults(article)

        self.assertEqual(normalized.meta["author"], "39Claw")
        self.assertEqual(normalized.meta["open_comment"], 1)
        self.assertEqual(normalized.meta["source_count"], 2)
        self.assertEqual(normalized.meta["news_count"], 1)
        self.assertEqual(normalized.meta["date_short"], "2026.03.31")

    def test_normalize_paragraphs_and_summary_helpers_match_expected_semantics(self) -> None:
        self.assertEqual(normalize_paragraphs("第一段\n\n第二段"), ["第一段", "第二段"])
        self.assertEqual(normalize_paragraphs([" A ", "", "B "]), ["A", "B"])

        article = ArticleDraft(
            article_id="article-1",
            template="daily-intelligence",
            meta={"title": "T", "digest": "D"},
            headline={"title": "H", "body": ["B"], "source": "Headline Source"},
            sections=[
                {
                    "cn": "要闻",
                    "en": "BRIEFING",
                    "blocks": [
                        {"type": "card", "source": "A"},
                        {"type": "opinion", "source": "A"},
                        {"type": "week-ahead", "source": "B"},
                        {"type": "paragraph", "source": "Ignored"},
                    ],
                }
            ],
            conclusion="",
            cta="",
            source_refs=[],
            target_platforms=["wechat"],
        )

        self.assertEqual(count_sources(article), 4)
        self.assertEqual(count_news_items(article), 3)

    def test_run_formatting_pipeline_supports_dry_run_and_returns_summary_shape(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            template_root = tmp_path / "templates"
            output_dir = tmp_path / "build"
            template_root.mkdir(parents=True)
            (template_root / "daily-intelligence.html").write_text(
                "<html>{{TITLE}}{{HEADLINE_BODY}}{{BODY_SECTIONS}}</html>",
                encoding="utf-8",
            )
            article = ArticleDraft(
                article_id="article-1",
                template="daily-intelligence",
                meta={"title": "AI 日报", "digest": "摘要", "date": "2026-03-31"},
                headline={"title": "头条", "body": ["第一段"]},
                sections=[
                    {
                        "cn": "要闻",
                        "en": "BRIEFING",
                        "blocks": [{"type": "card", "title": "新闻一", "body": ["正文"], "source": "来源"}],
                    }
                ],
                conclusion="",
                cta="",
                source_refs=[],
                target_platforms=["wechat"],
            )
            formatter = WechatHtmlFormatter(FileTemplateCatalog(template_root))

            summary = run_formatting_pipeline(article, formatter=formatter, output_dir=output_dir, dry_run=True)

            self.assertEqual(summary["html"].endswith(".html"), True)
            self.assertEqual(summary["resolved_article"].endswith(".resolved.json"), True)


if __name__ == "__main__":
    unittest.main()
