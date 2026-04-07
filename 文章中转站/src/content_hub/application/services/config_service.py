from __future__ import annotations

import os
from pathlib import Path
from typing import Any

import yaml

from content_hub.bootstrap.settings import HubSettings
from content_hub.domain.workspace.models import ResolvedWorkspaceSettings
from content_hub.domain.workspace.models import WorkspaceSettings


class ConfigService:
    def __init__(self, project_root: Path):
        self.project_root = project_root

    def load_legacy_settings(self) -> HubSettings:
        config_path = self.project_root / "src" / "ai_write_x" / "config" / "config.yaml"
        config_data = yaml.safe_load(config_path.read_text(encoding="utf-8")) or {}
        return HubSettings.from_legacy_config(config_data, self.project_root)

    def save_hub_settings(self, settings: HubSettings, output_path: Path) -> Path:
        output_path.parent.mkdir(parents=True, exist_ok=True)
        output_path.write_text(
            yaml.safe_dump(settings.to_dict(), allow_unicode=True, sort_keys=False),
            encoding="utf-8",
        )
        return output_path

    def workspace_config_path(self, workspace_root: Path) -> Path:
        return workspace_root / "workspace.yaml"

    def workspace_secrets_path(self, workspace_root: Path) -> Path:
        return workspace_root / "secrets.yaml"

    def initialize_workspace(self, workspace_root: Path, settings: WorkspaceSettings) -> Path:
        workspace_root.mkdir(parents=True, exist_ok=True)
        self._ensure_workspace_directories(workspace_root, settings)

        secrets_path = self.workspace_secrets_path(workspace_root)
        if not secrets_path.exists():
            secrets_path.write_text("", encoding="utf-8")

        return self.save_workspace_settings(workspace_root, settings)

    def load_workspace_settings(self, workspace_root: Path) -> WorkspaceSettings:
        config_path = self.workspace_config_path(workspace_root)
        raw = yaml.safe_load(config_path.read_text(encoding="utf-8")) or {}
        return WorkspaceSettings.from_dict(raw)

    def save_workspace_settings(self, workspace_root: Path, settings: WorkspaceSettings) -> Path:
        config_path = self.workspace_config_path(workspace_root)
        config_path.parent.mkdir(parents=True, exist_ok=True)
        config_path.write_text(
            yaml.safe_dump(settings.to_dict(), allow_unicode=True, sort_keys=False),
            encoding="utf-8",
        )
        return config_path

    def load_workspace_secrets(self, workspace_root: Path) -> dict[str, str | None]:
        secrets_path = self.workspace_secrets_path(workspace_root)
        if not secrets_path.exists():
            return {}
        raw = yaml.safe_load(secrets_path.read_text(encoding="utf-8")) or {}
        return self._flatten_secrets(raw)

    def resolve_workspace_settings(self, workspace_root: Path) -> ResolvedWorkspaceSettings:
        settings = self.load_workspace_settings(workspace_root)
        secrets = self.load_workspace_secrets(workspace_root)
        refs: set[str] = set()
        refs.update(
            profile.secret_ref
            for profile in settings.provider_profiles.values()
            if profile.secret_ref
        )
        refs.update(
            profile.secret_ref
            for profile in settings.publish_profiles.values()
            if profile.secret_ref
        )
        for ref in refs:
            if ref.startswith("env."):
                secrets[ref] = self._normalize_secret_value(
                    os.environ.get(ref.removeprefix("env."))
                )
        return ResolvedWorkspaceSettings(workspace=settings, secrets=secrets)

    def validate_workspace(self, workspace_root: Path) -> list[str]:
        errors: list[str] = []
        config_path = self.workspace_config_path(workspace_root)
        if not config_path.exists():
            return [f"missing workspace config: {config_path}"]

        resolved = self.resolve_workspace_settings(workspace_root)
        workspace = resolved.workspace

        if workspace.default_provider_profile not in workspace.provider_profiles:
            errors.append(
                f"missing default provider profile: {workspace.default_provider_profile}"
            )
        if workspace.default_article_profile not in workspace.article_profiles:
            errors.append(
                f"missing default article profile: {workspace.default_article_profile}"
            )
        if workspace.default_publish_profile not in workspace.publish_profiles:
            errors.append(
                f"missing default publish profile: {workspace.default_publish_profile}"
            )

        for profile_name, profile in workspace.provider_profiles.items():
            if profile.secret_ref and not resolved.secrets.get(profile.secret_ref):
                errors.append(
                    f"missing secret for provider profile {profile_name}: {profile.secret_ref}"
                )

        for profile_name, profile in workspace.publish_profiles.items():
            if profile.secret_ref and not resolved.secrets.get(profile.secret_ref):
                errors.append(
                    f"missing secret for publish profile {profile_name}: {profile.secret_ref}"
                )

        return errors

    def _flatten_secrets(
        self, raw: dict[str, Any], prefix: str = ""
    ) -> dict[str, str | None]:
        flattened: dict[str, str | None] = {}
        for key, value in raw.items():
            full_key = f"{prefix}.{key}" if prefix else str(key)
            if isinstance(value, dict):
                flattened.update(self._flatten_secrets(value, full_key))
                continue
            flattened[full_key] = self._normalize_secret_value(value)
        return flattened

    def _ensure_workspace_directories(
        self, workspace_root: Path, settings: WorkspaceSettings
    ) -> None:
        for directory in (
            settings.paths.data_dir,
            settings.paths.incoming_dir,
            settings.paths.articles_dir,
            settings.paths.drafts_dir,
            settings.paths.rendered_dir,
            settings.paths.reviews_dir,
            settings.paths.publish_records_dir,
            settings.paths.logs_dir,
        ):
            (workspace_root / directory).mkdir(parents=True, exist_ok=True)

    def _normalize_secret_value(self, value: Any) -> str | None:
        if value is None:
            return None

        normalized = str(value).strip()
        if not normalized:
            return None

        return normalized
