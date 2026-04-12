from pathlib import Path
import tempfile
import unittest

try:
    from fastapi.testclient import TestClient
except ModuleNotFoundError:  # pragma: no cover - environment dependent
    TestClient = None

if TestClient is not None:
    from content_hub.bootstrap.settings import HubSettings, LLMSettings, PublishSettings, RewriteSettings, StorageSettings, TemplateSettings, WorkflowSettings
    from content_hub.interfaces.api.main import create_app
else:  # pragma: no cover - environment dependent
    HubSettings = None
    LLMSettings = None
    PublishSettings = None
    RewriteSettings = None
    StorageSettings = None
    TemplateSettings = None
    WorkflowSettings = None
    create_app = None


@unittest.skipIf(TestClient is None, "fastapi is not installed in the current environment")
class FormattingApiTestCase(unittest.TestCase):
    def test_create_render_review_and_publish_flow(self) -> None:
        self.assertIsNotNone(TestClient)
        self.assertIsNotNone(create_app)
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            template_root = tmp_path / "templates"
            template_root.mkdir(parents=True)
            (template_root / "daily-intelligence.html").write_text(
                (
                    "<html><body>"
                    "<header>{{TITLE}}|{{DIGEST}}|{{AUTHOR}}</header>"
                    "<section>{{HEADLINE_BODY}}</section>"
                    "<main>{{BODY_SECTIONS}}</main>"
                    "<footer>{{CONCLUSION}}|{{CTA}}</footer>"
                    "</body></html>"
                ),
                encoding="utf-8",
            )
            settings = HubSettings(
                llm=LLMSettings(provider="stub", model="stub-model"),
                workflow=WorkflowSettings(publish_platform="wechat", article_format="markdown", auto_publish=False),
                rewrite=RewriteSettings(enabled=False),
                template=TemplateSettings(root_dir=template_root),
                storage=StorageSettings(root_dir=tmp_path / "storage"),
                publish=PublishSettings(wechat_credentials=[]),
            )
            client = TestClient(create_app(project_root=tmp_path, settings_override=settings))

            create_response = client.post(
                "/drafts",
                json={
                    "template": "daily-intelligence",
                    "meta": {"title": "AI 日报", "digest": "摘要", "author": "editor"},
                    "headline": {"title": "头条", "body": ["第一段"]},
                    "sections": [
                        {
                            "cn": "要闻",
                            "en": "BRIEFING",
                            "blocks": [{"type": "card", "title": "新闻标题", "body": ["正文"], "source": "来源"}],
                        }
                    ],
                    "conclusion": "结论",
                    "cta": "行动",
                    "target_platforms": ["wechat"],
                },
            )
            self.assertEqual(create_response.status_code, 200)
            article_id = create_response.json()["article_id"]

            get_response = client.get(f"/drafts/{article_id}")
            self.assertEqual(get_response.status_code, 200)
            self.assertEqual(get_response.json()["template"], "daily-intelligence")

            render_response = client.post(
                "/formatting/render",
                json={
                    "article_id": article_id,
                    "targets": [{"platform": "wechat", "template": "daily-intelligence", "output_format": "html"}],
                },
            )
            self.assertEqual(render_response.status_code, 200)
            assets = render_response.json()["assets"]
            self.assertEqual(len(assets), 1)
            asset_id = assets[0]["asset_id"]

            asset_response = client.get(f"/formatting/assets/{asset_id}")
            self.assertEqual(asset_response.status_code, 200)
            self.assertIn("AI 日报", asset_response.json()["content"])

            review_response = client.post(
                "/reviews",
                json={"article_id": article_id, "asset_ids": [asset_id], "reviewer": "alice", "notes": "待审"},
            )
            self.assertEqual(review_response.status_code, 200)
            review_id = review_response.json()["review_id"]

            approve_response = client.post(
                f"/reviews/{review_id}/approve",
                json={"reviewer": "bob", "notes": "通过"},
            )
            self.assertEqual(approve_response.status_code, 200)
            self.assertEqual(approve_response.json()["status"], "approved")

            publish_response = client.post(f"/reviews/{review_id}/publish", json={})
            self.assertEqual(publish_response.status_code, 200)
            self.assertEqual(publish_response.json()["results"][0]["platform"], "wechat")
            self.assertFalse(publish_response.json()["results"][0]["success"])
            self.assertEqual(
                publish_response.json()["results"][0]["metadata"]["error_code"],
                "MISSING_CREDENTIALS",
            )


if __name__ == "__main__":
    unittest.main()
