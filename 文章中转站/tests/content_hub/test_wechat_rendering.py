from pathlib import Path
import tempfile
import unittest

from content_hub.application.formatting.article_normalizer import ensure_article_defaults
from content_hub.domain.formatting.models import ArticleDraft, FormatTarget
from content_hub.infrastructure.formatters.template_catalog import FileTemplateCatalog
from content_hub.infrastructure.formatters.wechat_html_formatter import WechatHtmlFormatter


class WechatRenderingTestCase(unittest.TestCase):
    def test_article_normalizer_fills_default_meta_fields(self) -> None:
        article = ArticleDraft(
            article_id="article-1",
            template="daily-intelligence",
            meta={"title": "AI 日报", "digest": "摘要"},
            headline={"title": "头条", "body": ["第一段"]},
            sections=[{"cn": "要闻", "en": "BRIEFING", "blocks": []}],
            conclusion="结论",
            cta="行动",
            source_refs=[],
            target_platforms=["wechat"],
        )

        normalized = ensure_article_defaults(article)

        self.assertEqual(normalized.meta["author"], "39Claw")
        self.assertIn("date", normalized.meta)
        self.assertIn("date_cn", normalized.meta)
        self.assertIn("news_count", normalized.meta)

    def test_wechat_html_formatter_validates_missing_required_fields(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            template_root = Path(tmp_dir)
            (template_root / "daily-intelligence.html").write_text(
                "<html>{{TITLE}}{{HEADLINE_BODY}}{{BODY_SECTIONS}}{{CONCLUSION}}{{CTA}}</html>",
                encoding="utf-8",
            )
            formatter = WechatHtmlFormatter(FileTemplateCatalog(template_root))
            invalid_article = ArticleDraft(
                article_id="article-1",
                template="daily-intelligence",
                meta={"title": "", "digest": ""},
                headline={"title": "", "body": []},
                sections=[],
                conclusion="",
                cta="",
                source_refs=[],
                target_platforms=["wechat"],
            )

            errors = formatter.validate(invalid_article, FormatTarget(platform="wechat", template="daily-intelligence", output_format="html"))

            self.assertIn("meta.title is required", errors)
            self.assertIn("meta.digest is required", errors)
            self.assertIn("headline.title is required", errors)
            self.assertIn("sections must be a non-empty array", errors)

    def test_wechat_html_formatter_renders_html_from_structured_article(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            template_root = Path(tmp_dir)
            (template_root / "daily-intelligence.html").write_text(
                """
                <html>
                <body>
                <header>{{TITLE}}|{{DIGEST}}|{{AUTHOR}}</header>
                <section>{{HEADLINE_BODY}}</section>
                <main>{{BODY_SECTIONS}}</main>
                <footer>{{CONCLUSION}}|{{CTA}}</footer>
                </body>
                </html>
                """.strip(),
                encoding="utf-8",
            )
            formatter = WechatHtmlFormatter(FileTemplateCatalog(template_root))
            article = ArticleDraft(
                article_id="article-1",
                template="daily-intelligence",
                meta={"title": "AI 日报", "digest": "摘要", "author": "editor"},
                headline={"title": "头条", "body": ["第一段", "第二段"], "source": "Example Source"},
                sections=[
                    {
                        "cn": "要闻",
                        "en": "BRIEFING",
                        "blocks": [
                            {
                                "type": "card",
                                "number": "1",
                                "title": "新闻标题",
                                "body": ["新闻正文"],
                                "source": "Example Source",
                            }
                        ],
                    }
                ],
                conclusion="本期重点在于模型和应用的同步推进。",
                cta="欢迎留言讨论。",
                source_refs=[],
                target_platforms=["wechat"],
            )

            asset = formatter.render(article, FormatTarget(platform="wechat", template="daily-intelligence", output_format="html"))

            self.assertEqual(asset.platform, "wechat")
            self.assertEqual(asset.output_format, "html")
            self.assertIn("AI 日报", asset.content)
            self.assertIn("新闻标题", asset.content)
            self.assertIn("欢迎留言讨论", asset.content)
            self.assertNotIn("{{", asset.content)


if __name__ == "__main__":
    unittest.main()
