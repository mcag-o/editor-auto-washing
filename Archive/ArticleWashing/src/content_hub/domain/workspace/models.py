from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any


@dataclass
class WorkspacePaths:
    data_dir: str = "workspace_data"
    incoming_dir: str = "incoming"
    articles_dir: str = "articles"
    drafts_dir: str = "drafts"
    rendered_dir: str = "rendered"
    reviews_dir: str = "reviews"
    publish_records_dir: str = "publish_records"
    logs_dir: str = "logs"

    @classmethod
    def from_dict(cls, raw: dict[str, Any] | None) -> "WorkspacePaths":
        raw = raw or {}
        defaults = cls()
        return cls(**{key: raw.get(key, getattr(defaults, key)) for key in cls.__dataclass_fields__})

    def to_dict(self) -> dict[str, Any]:
        return {key: getattr(self, key) for key in self.__dataclass_fields__}


@dataclass
class ProviderProfile:
    provider: str
    model: str
    secret_ref: str
    base_url: str = ""
    temperature: float = 0.7
    max_tokens: int = 4096
    enabled: bool = True

    @classmethod
    def from_dict(cls, raw: dict[str, Any]) -> "ProviderProfile":
        return cls(
            provider=raw.get("provider", ""),
            model=raw.get("model", ""),
            secret_ref=raw.get("secret_ref", ""),
            base_url=raw.get("base_url", ""),
            temperature=float(raw.get("temperature", 0.7)),
            max_tokens=int(raw.get("max_tokens", 4096)),
            enabled=bool(raw.get("enabled", True)),
        )

    def to_dict(self) -> dict[str, Any]:
        return {
            "provider": self.provider,
            "model": self.model,
            "secret_ref": self.secret_ref,
            "base_url": self.base_url,
            "temperature": self.temperature,
            "max_tokens": self.max_tokens,
            "enabled": self.enabled,
        }


@dataclass
class ArticleProfile:
    style: str
    output_format: str
    template: str
    length: str = "medium"
    image_policy: str = "none"
    allow_auto_publish: bool = False

    @classmethod
    def from_dict(cls, raw: dict[str, Any]) -> "ArticleProfile":
        return cls(
            style=raw.get("style", "news-rewrite"),
            output_format=raw.get("output_format", "html"),
            template=raw.get("template", "daily-intelligence"),
            length=raw.get("length", "medium"),
            image_policy=raw.get("image_policy", "none"),
            allow_auto_publish=bool(raw.get("allow_auto_publish", False)),
        )

    def to_dict(self) -> dict[str, Any]:
        return {
            "style": self.style,
            "output_format": self.output_format,
            "template": self.template,
            "length": self.length,
            "image_policy": self.image_policy,
            "allow_auto_publish": self.allow_auto_publish,
        }


@dataclass
class PublishProfile:
    platform: str
    account: str
    secret_ref: str
    allow_auto_publish: bool = False
    retry_count: int = 1
    fallback_to_review: bool = True

    @classmethod
    def from_dict(cls, raw: dict[str, Any]) -> "PublishProfile":
        return cls(
            platform=raw.get("platform", "wechat"),
            account=raw.get("account", "main"),
            secret_ref=raw.get("secret_ref", ""),
            allow_auto_publish=bool(raw.get("allow_auto_publish", False)),
            retry_count=int(raw.get("retry_count", 1)),
            fallback_to_review=bool(raw.get("fallback_to_review", True)),
        )

    def to_dict(self) -> dict[str, Any]:
        return {
            "platform": self.platform,
            "account": self.account,
            "secret_ref": self.secret_ref,
            "allow_auto_publish": self.allow_auto_publish,
            "retry_count": self.retry_count,
            "fallback_to_review": self.fallback_to_review,
        }


@dataclass
class ReviewPolicy:
    default_mode: str = "review_required"
    auto_publish_profiles: list[str] = field(default_factory=list)
    blocking_errors: list[str] = field(
        default_factory=lambda: ["missing_secret", "render_failed", "validation_failed"]
    )

    @classmethod
    def from_dict(cls, raw: dict[str, Any] | None) -> "ReviewPolicy":
        raw = raw or {}
        return cls(
            default_mode=raw.get("default_mode", "review_required"),
            auto_publish_profiles=list(raw.get("auto_publish_profiles", [])),
            blocking_errors=list(
                raw.get(
                    "blocking_errors",
                    ["missing_secret", "render_failed", "validation_failed"],
                )
            ),
        )

    def to_dict(self) -> dict[str, Any]:
        return {
            "default_mode": self.default_mode,
            "auto_publish_profiles": self.auto_publish_profiles,
            "blocking_errors": self.blocking_errors,
        }


@dataclass
class AutomationPolicy:
    auto_import: bool = False
    auto_generate: bool = False
    auto_publish: bool = False
    interval_seconds: int = 1800
    incoming_dir: str | None = None
    processed_dir: str | None = None
    failed_dir: str | None = None
    alert_warning_threshold: int | None = None
    alert_critical_threshold: int | None = None
    alert_webhook_cooldown_seconds: int | None = None

    @classmethod
    def from_dict(cls, raw: dict[str, Any] | None) -> "AutomationPolicy":
        raw = raw or {}
        return cls(
            auto_import=bool(raw.get("auto_import", False)),
            auto_generate=bool(raw.get("auto_generate", False)),
            auto_publish=bool(raw.get("auto_publish", False)),
            interval_seconds=int(raw.get("interval_seconds", 1800)),
            incoming_dir=(
                str(raw["incoming_dir"])
                if raw.get("incoming_dir") is not None
                else None
            ),
            processed_dir=(
                str(raw["processed_dir"])
                if raw.get("processed_dir") is not None
                else None
            ),
            failed_dir=(
                str(raw["failed_dir"])
                if raw.get("failed_dir") is not None
                else None
            ),
            alert_warning_threshold=(
                int(raw["alert_warning_threshold"])
                if raw.get("alert_warning_threshold") is not None
                else None
            ),
            alert_critical_threshold=(
                int(raw["alert_critical_threshold"])
                if raw.get("alert_critical_threshold") is not None
                else None
            ),
            alert_webhook_cooldown_seconds=(
                int(raw["alert_webhook_cooldown_seconds"])
                if raw.get("alert_webhook_cooldown_seconds") is not None
                else None
            ),
        )

    def to_dict(self) -> dict[str, Any]:
        return {
            "auto_import": self.auto_import,
            "auto_generate": self.auto_generate,
            "auto_publish": self.auto_publish,
            "interval_seconds": self.interval_seconds,
            "incoming_dir": self.incoming_dir,
            "processed_dir": self.processed_dir,
            "failed_dir": self.failed_dir,
            "alert_warning_threshold": self.alert_warning_threshold,
            "alert_critical_threshold": self.alert_critical_threshold,
            "alert_webhook_cooldown_seconds": self.alert_webhook_cooldown_seconds,
        }


@dataclass
class CollectorPolicy:
    enabled: bool = False
    command: str = ""
    working_dir: str = "DataCollection"
    bundle_out_pattern: str = "incoming/bundle-{timestamp}.json"
    platforms: list[str] = field(default_factory=list)
    global_concurrency: int = 4
    http_timeout_ms: int = 10000
    http_retry_count: int = 2
    http_retry_base_ms: int = 250
    default_user_agent: str = ""
    http_proxy: str = ""
    https_proxy: str = ""
    weibo_cookie: str = ""
    xueqiu_cookie: str = ""
    enable_browser_fallback: bool = False

    @classmethod
    def from_dict(cls, raw: dict[str, Any] | None) -> "CollectorPolicy":
        raw = raw or {}
        return cls(
            enabled=bool(raw.get("enabled", False)),
            command=raw.get("command", ""),
            working_dir=raw.get("working_dir", "DataCollection"),
            bundle_out_pattern=raw.get(
                "bundle_out_pattern",
                "incoming/bundle-{timestamp}.json",
            ),
            platforms=list(raw.get("platforms", [])),
            global_concurrency=int(raw.get("global_concurrency", 4)),
            http_timeout_ms=int(raw.get("http_timeout_ms", 10000)),
            http_retry_count=int(raw.get("http_retry_count", 2)),
            http_retry_base_ms=int(raw.get("http_retry_base_ms", 250)),
            default_user_agent=str(raw.get("default_user_agent", "")),
            http_proxy=str(raw.get("http_proxy", "")),
            https_proxy=str(raw.get("https_proxy", "")),
            weibo_cookie=str(raw.get("weibo_cookie", "")),
            xueqiu_cookie=str(raw.get("xueqiu_cookie", "")),
            enable_browser_fallback=bool(raw.get("enable_browser_fallback", False)),
        )

    def to_dict(self) -> dict[str, Any]:
        return {
            "enabled": self.enabled,
            "command": self.command,
            "working_dir": self.working_dir,
            "bundle_out_pattern": self.bundle_out_pattern,
            "platforms": self.platforms,
            "global_concurrency": self.global_concurrency,
            "http_timeout_ms": self.http_timeout_ms,
            "http_retry_count": self.http_retry_count,
            "http_retry_base_ms": self.http_retry_base_ms,
            "default_user_agent": self.default_user_agent,
            "http_proxy": self.http_proxy,
            "https_proxy": self.https_proxy,
            "weibo_cookie": self.weibo_cookie,
            "xueqiu_cookie": self.xueqiu_cookie,
            "enable_browser_fallback": self.enable_browser_fallback,
        }


@dataclass
class WorkspaceSettings:
    name: str
    paths: WorkspacePaths
    provider_profiles: dict[str, ProviderProfile]
    article_profiles: dict[str, ArticleProfile]
    publish_profiles: dict[str, PublishProfile]
    review_policy: ReviewPolicy
    automation: AutomationPolicy
    default_provider_profile: str
    default_article_profile: str
    default_publish_profile: str
    collector: CollectorPolicy = field(default_factory=CollectorPolicy)

    @classmethod
    def default(cls, name: str = "content-workspace") -> "WorkspaceSettings":
        return cls(
            name=name,
            paths=WorkspacePaths(),
            provider_profiles={
                "default": ProviderProfile(
                    provider="openai-compatible",
                    model="",
                    secret_ref="env.LLM_API_KEY",
                )
            },
            article_profiles={
                "wechat-daily": ArticleProfile(
                    style="news-rewrite",
                    output_format="html",
                    template="daily-intelligence",
                )
            },
            publish_profiles={
                "wechat-review": PublishProfile(
                    platform="wechat",
                    account="main",
                    secret_ref="wechat.main",
                )
            },
            review_policy=ReviewPolicy(),
            automation=AutomationPolicy(),
            collector=CollectorPolicy(),
            default_provider_profile="default",
            default_article_profile="wechat-daily",
            default_publish_profile="wechat-review",
        )

    @classmethod
    def from_dict(cls, raw: dict[str, Any]) -> "WorkspaceSettings":
        return cls(
            name=raw.get("name", "content-workspace"),
            paths=WorkspacePaths.from_dict(raw.get("paths")),
            provider_profiles={
                key: ProviderProfile.from_dict(value)
                for key, value in raw.get("provider_profiles", {}).items()
            },
            article_profiles={
                key: ArticleProfile.from_dict(value)
                for key, value in raw.get("article_profiles", {}).items()
            },
            publish_profiles={
                key: PublishProfile.from_dict(value)
                for key, value in raw.get("publish_profiles", {}).items()
            },
            review_policy=ReviewPolicy.from_dict(raw.get("review_policy")),
            automation=AutomationPolicy.from_dict(raw.get("automation")),
            collector=CollectorPolicy.from_dict(raw.get("collector")),
            default_provider_profile=raw.get("default_provider_profile", "default"),
            default_article_profile=raw.get("default_article_profile", "wechat-daily"),
            default_publish_profile=raw.get("default_publish_profile", "wechat-review"),
        )

    def to_dict(self) -> dict[str, Any]:
        return {
            "name": self.name,
            "paths": self.paths.to_dict(),
            "default_provider_profile": self.default_provider_profile,
            "default_article_profile": self.default_article_profile,
            "default_publish_profile": self.default_publish_profile,
            "provider_profiles": {
                key: value.to_dict() for key, value in self.provider_profiles.items()
            },
            "article_profiles": {
                key: value.to_dict() for key, value in self.article_profiles.items()
            },
            "publish_profiles": {
                key: value.to_dict() for key, value in self.publish_profiles.items()
            },
            "review_policy": self.review_policy.to_dict(),
            "automation": self.automation.to_dict(),
            "collector": self.collector.to_dict(),
        }


@dataclass
class ResolvedWorkspaceSettings:
    workspace: WorkspaceSettings
    secrets: dict[str, str | None]
