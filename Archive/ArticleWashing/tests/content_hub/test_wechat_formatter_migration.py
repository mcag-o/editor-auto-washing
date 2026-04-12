from pathlib import Path
import tempfile
import unittest

from content_hub.domain.formatting.models import ArticleDraft, FormatTarget
from content_hub.infrastructure.formatters.template_catalog import FileTemplateCatalog
from content_hub.infrastructure.formatters.wechat_html_formatter import WechatHtmlFormatter


class WechatFormatterMigrationTestCase(unittest.TestCase):
    def test_formatter_supports_opinion_week_ahead_quote_takeaways_image_and_paragraph_blocks(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            template_root = Path(tmp_dir)
            (template_root / "weekly-financial.html").write_text(
                "<html>{{TITLE}}{{HEADLINE_BODY}}{{BODY_SECTIONS}}{{CONCLUSION}}{{CTA}}</html>",
                encoding="utf-8",
            )
            formatter = WechatHtmlFormatter(FileTemplateCatalog(template_root))
            article = ArticleDraft(
                article_id="article-1",
                template="weekly-financial",
                meta={"title": "财经周报", "digest": "摘要", "date": "2026-03-31"},
                headline={"title": "本周头条", "body": ["头条第一段"], "source": "Bloomberg"},
                sections=[
                    {
                        "cn": "全球市场",
                        "en": "GLOBAL MARKETS",
                        "blocks": [
                            {"type": "opinion", "number": 1, "title": "观点标题", "body": ["观点正文"]},
                            {
                                "type": "week-ahead",
                                "number": 2,
                                "title": "下周前瞻",
                                "days": [{"label": "周一", "events": "CPI"}, {"label": "周二", "events": "财报"}],
                                "source": "日历",
                            },
                            {"type": "quote", "text": "市场正在重估。", "attribution": "分析师"},
                            {"type": "takeaways", "title": "核心结论", "items": ["第一点", "第二点"]},
                            {"type": "image", "url": "https://mmbiz.qpic.cn/example", "caption": "配图说明"},
                            {"type": "paragraph", "text": "补充段落"},
                        ],
                    }
                ],
                conclusion="结论",
                cta="行动号召",
                source_refs=[],
                target_platforms=["wechat"],
            )

            asset = formatter.render(article, FormatTarget(platform="wechat", template="weekly-financial", output_format="html"))

            self.assertIn("编辑观点：观点标题", asset.content)
            self.assertIn("下周前瞻", asset.content)
            self.assertIn("市场正在重估", asset.content)
            self.assertIn("核心结论", asset.content)
            self.assertIn("配图说明", asset.content)
            self.assertIn("补充段落", asset.content)

    def test_formatter_supports_studio_brief_and_neo_brutalism_variants(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            template_root = Path(tmp_dir)
            (template_root / "studio-brief.html").write_text(
                "<html>Studio Brief {{HEADLINE_IMAGE}}{{BODY_SECTIONS}}</html>",
                encoding="utf-8",
            )
            (template_root / "neo-brutalism.html").write_text(
                "<html>NEO BRUTALISM ISSUE {{HEADLINE_IMAGE}}{{BODY_SECTIONS}}</html>",
                encoding="utf-8",
            )
            formatter = WechatHtmlFormatter(FileTemplateCatalog(template_root))
            base = {
                "meta": {"title": "风格测试", "digest": "摘要", "date": "2026-03-31"},
                "headline": {"title": "头条", "body": ["正文"], "source": "来源"},
                "sections": [
                    {
                        "cn": "要闻",
                        "en": "BRIEFING",
                        "image": {"url": "https://mmbiz.qpic.cn/example", "caption": "配图"},
                        "blocks": [{"type": "card", "number": 1, "title": "标题", "body": ["正文"], "source": "来源"}],
                    }
                ],
                "conclusion": "结论",
                "cta": "行动号召",
                "source_refs": [],
                "target_platforms": ["wechat"],
            }
            studio = formatter.render(ArticleDraft(article_id="a1", template="studio-brief", **base), FormatTarget(platform="wechat", template="studio-brief", output_format="html"))
            brutal = formatter.render(ArticleDraft(article_id="a2", template="neo-brutalism", **base), FormatTarget(platform="wechat", template="neo-brutalism", output_format="html"))

            self.assertIn("Studio Brief", studio.content)
            self.assertIn("border-radius: 14px", studio.content)
            self.assertIn("NEO BRUTALISM ISSUE", brutal.content)
            self.assertIn("box-shadow: 8px 8px 0", brutal.content)


if __name__ == "__main__":
    unittest.main()
