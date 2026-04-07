import unittest

from content_hub.application.formatting.image_plan import attach_missing_image_plans
from content_hub.domain.formatting.models import ArticleDraft


class ImagePlanParityTestCase(unittest.TestCase):
    def test_attach_missing_image_plans_adds_cover_and_content_image_plans(self) -> None:
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

        updated = attach_missing_image_plans(article, output_dir="build/images", max_content_images=3)

        self.assertIn("prompt", updated.meta["cover_image"])
        self.assertTrue(updated.meta["cover_image"]["local_path"].endswith("cover-2026-03-31.png"))
        self.assertIn("prompt", updated.headline["image"])
        self.assertIn("prompt", updated.sections[0]["image"])
        self.assertEqual(len(updated.meta["image_plans"]), 3)


if __name__ == "__main__":
    unittest.main()
