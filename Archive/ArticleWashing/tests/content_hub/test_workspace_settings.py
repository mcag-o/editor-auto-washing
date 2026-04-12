from __future__ import annotations

import os
import tempfile
import unittest
from pathlib import Path

from content_hub.application.services.config_service import ConfigService
from content_hub.domain.workspace.models import (
    ArticleProfile,
    AutomationPolicy,
    ProviderProfile,
    PublishProfile,
    ReviewPolicy,
    WorkspacePaths,
    WorkspaceSettings,
)


class WorkspaceSettingsTestCase(unittest.TestCase):
    def test_workspace_settings_has_cloud_ready_defaults(self) -> None:
        settings = WorkspaceSettings.default(name="daily-workspace")

        self.assertEqual(settings.name, "daily-workspace")
        self.assertEqual(settings.default_provider_profile, "default")
        self.assertEqual(settings.default_article_profile, "wechat-daily")
        self.assertEqual(settings.default_publish_profile, "wechat-review")
        self.assertEqual(settings.paths.incoming_dir, "incoming")
        self.assertEqual(settings.review_policy.default_mode, "review_required")
        self.assertFalse(settings.automation.auto_publish)
        self.assertFalse(settings.collector.enabled)

    def test_workspace_settings_round_trips_to_dict(self) -> None:
        settings = WorkspaceSettings(
            name="prod",
            paths=WorkspacePaths(data_dir="data", incoming_dir="incoming"),
            provider_profiles={
                "deepseek": ProviderProfile(
                    provider="deepseek",
                    model="deepseek-chat",
                    secret_ref="env.DEEPSEEK_API_KEY",
                )
            },
            article_profiles={
                "wechat-auto": ArticleProfile(
                    style="news-rewrite",
                    output_format="html",
                    template="daily-intelligence",
                    allow_auto_publish=True,
                )
            },
            publish_profiles={
                "wechat-auto": PublishProfile(
                    platform="wechat",
                    account="main",
                    secret_ref="wechat.main",
                    allow_auto_publish=True,
                )
            },
            review_policy=ReviewPolicy(default_mode="review_required", auto_publish_profiles=["wechat-auto"]),
            automation=AutomationPolicy(
                auto_import=True,
                auto_generate=True,
                auto_publish=True,
                interval_seconds=900,
            ),
            default_provider_profile="deepseek",
            default_article_profile="wechat-auto",
            default_publish_profile="wechat-auto",
        )

        payload = settings.to_dict()
        restored = WorkspaceSettings.from_dict(payload)

        self.assertEqual(restored.name, "prod")
        self.assertEqual(restored.provider_profiles["deepseek"].model, "deepseek-chat")
        self.assertTrue(restored.article_profiles["wechat-auto"].allow_auto_publish)
        self.assertTrue(restored.publish_profiles["wechat-auto"].allow_auto_publish)
        self.assertEqual(restored.automation.interval_seconds, 900)
        self.assertEqual(restored.collector.global_concurrency, 4)
        self.assertEqual(restored.to_dict(), payload)

    def test_config_service_loads_workspace_yaml_and_resolves_secrets(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir)
            (workspace_root / "workspace.yaml").write_text(
                """
name: prod-workspace
default_provider_profile: deepseek
default_article_profile: wechat-auto
default_publish_profile: wechat-auto
paths:
  data_dir: data
  incoming_dir: incoming
provider_profiles:
  deepseek:
    provider: deepseek
    model: deepseek-chat
    secret_ref: env.DEEPSEEK_API_KEY
article_profiles:
  wechat-auto:
    style: daily-rewrite
    output_format: html
    template: daily-intelligence
    allow_auto_publish: true
publish_profiles:
  wechat-auto:
    platform: wechat
    account: main
    secret_ref: wechat.main
    allow_auto_publish: true
review_policy:
  default_mode: review_required
  auto_publish_profiles: [wechat-auto]
automation:
  auto_import: true
  auto_generate: true
  auto_publish: true
  interval_seconds: 600
""".strip(),
                encoding="utf-8",
            )
            (workspace_root / "secrets.yaml").write_text(
                """
wechat:
  main: local-wechat-secret
""".strip(),
                encoding="utf-8",
            )
            original_secret = os.environ.get("DEEPSEEK_API_KEY")
            os.environ["DEEPSEEK_API_KEY"] = "env-secret"

            try:
                resolved = ConfigService(workspace_root).resolve_workspace_settings(workspace_root)
            finally:
                if original_secret is None:
                    os.environ.pop("DEEPSEEK_API_KEY", None)
                else:
                    os.environ["DEEPSEEK_API_KEY"] = original_secret

            self.assertEqual(resolved.workspace.name, "prod-workspace")
            self.assertEqual(resolved.secrets["env.DEEPSEEK_API_KEY"], "env-secret")
            self.assertEqual(resolved.secrets["wechat.main"], "local-wechat-secret")
            self.assertTrue(resolved.workspace.automation.auto_publish)

    def test_config_service_saves_and_validates_workspace_yaml(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir)
            service = ConfigService(workspace_root)
            settings = WorkspaceSettings.default(name="saved-workspace")
            (workspace_root / "secrets.yaml").write_text(
                """
wechat:
  main: test-wechat-secret
""".strip(),
                encoding="utf-8",
            )
            original_secret = os.environ.get("LLM_API_KEY")
            os.environ["LLM_API_KEY"] = "test-llm-secret"

            try:
                config_path = service.save_workspace_settings(workspace_root, settings)
                loaded = service.load_workspace_settings(workspace_root)
                errors = service.validate_workspace(workspace_root)
            finally:
                if original_secret is None:
                    os.environ.pop("LLM_API_KEY", None)
                else:
                    os.environ["LLM_API_KEY"] = original_secret

            self.assertEqual(config_path, workspace_root / "workspace.yaml")
            self.assertEqual(loaded.to_dict(), settings.to_dict())
            self.assertEqual(errors, [])

    def test_config_service_initializes_workspace_layout_with_default_directories(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            service = ConfigService(workspace_root)
            settings = WorkspaceSettings.default(name="initialized-workspace")

            config_path = service.initialize_workspace(workspace_root, settings)

            self.assertEqual(config_path, workspace_root / "workspace.yaml")
            self.assertTrue((workspace_root / "secrets.yaml").exists())
            self.assertEqual((workspace_root / "secrets.yaml").read_text(encoding="utf-8"), "")
            self.assertTrue((workspace_root / settings.paths.data_dir).is_dir())
            self.assertTrue((workspace_root / settings.paths.incoming_dir).is_dir())
            self.assertTrue((workspace_root / settings.paths.articles_dir).is_dir())
            self.assertTrue((workspace_root / settings.paths.drafts_dir).is_dir())
            self.assertTrue((workspace_root / settings.paths.rendered_dir).is_dir())
            self.assertTrue((workspace_root / settings.paths.reviews_dir).is_dir())
            self.assertTrue((workspace_root / settings.paths.publish_records_dir).is_dir())
            self.assertTrue((workspace_root / settings.paths.logs_dir).is_dir())

    def test_config_service_treats_null_and_blank_secret_sources_as_missing(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir)
            (workspace_root / "workspace.yaml").write_text(
                """
name: invalid-workspace
default_provider_profile: deepseek
default_article_profile: wechat-auto
default_publish_profile: wechat-auto
provider_profiles:
  deepseek:
    provider: deepseek
    model: deepseek-chat
    secret_ref: env.DEEPSEEK_API_KEY
article_profiles:
  wechat-auto:
    style: daily-rewrite
    output_format: html
    template: daily-intelligence
publish_profiles:
  wechat-auto:
    platform: wechat
    account: main
    secret_ref: wechat.main
""".strip(),
                encoding="utf-8",
            )
            (workspace_root / "secrets.yaml").write_text(
                """
wechat:
  main:
""".strip(),
                encoding="utf-8",
            )
            original_secret = os.environ.get("DEEPSEEK_API_KEY")
            os.environ["DEEPSEEK_API_KEY"] = ""

            try:
                service = ConfigService(workspace_root)
                resolved = service.resolve_workspace_settings(workspace_root)
                errors = service.validate_workspace(workspace_root)
            finally:
                if original_secret is None:
                    os.environ.pop("DEEPSEEK_API_KEY", None)
                else:
                    os.environ["DEEPSEEK_API_KEY"] = original_secret

            self.assertIsNone(resolved.secrets["env.DEEPSEEK_API_KEY"])
            self.assertIsNone(resolved.secrets["wechat.main"])
            self.assertEqual(
                errors,
                [
                    "missing secret for provider profile deepseek: env.DEEPSEEK_API_KEY",
                    "missing secret for publish profile wechat-auto: wechat.main",
                ],
            )


if __name__ == "__main__":
    unittest.main()
