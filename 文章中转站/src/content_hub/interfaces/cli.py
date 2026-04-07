from __future__ import annotations

import argparse
import json
import os
import time
import urllib.request
from datetime import datetime, timezone
from pathlib import Path

from content_hub.bootstrap.container import ServiceContainer
from content_hub.bootstrap.container import build_container
from content_hub.bootstrap.settings import HubSettings
from content_hub.bootstrap.settings import LLMSettings
from content_hub.bootstrap.settings import PublishSettings
from content_hub.bootstrap.settings import RewriteSettings
from content_hub.bootstrap.settings import StorageSettings
from content_hub.bootstrap.settings import TemplateSettings
from content_hub.bootstrap.settings import WorkflowSettings
from content_hub.application.services.config_service import ConfigService
from content_hub.application.formatting.pipeline import run_formatting_pipeline
from content_hub.domain.formatting.models import ArticleDraft, FormatTarget
from content_hub.domain.workspace.models import WorkspaceSettings
from content_hub.infrastructure.formatters.template_catalog import FileTemplateCatalog
from content_hub.infrastructure.formatters.wechat_html_formatter import WechatHtmlFormatter


AUTOMATION_FAILURE_ALERT_THRESHOLD = 3
AUTOMATION_CRITICAL_ALERT_THRESHOLD = 6
AUTOMATION_ALERT_WEBHOOK_ENV = "CONTENT_HUB_AUTOMATION_ALERT_WEBHOOK_URL"
AUTOMATION_ALERT_WEBHOOK_COOLDOWN_ENV = "CONTENT_HUB_AUTOMATION_ALERT_WEBHOOK_COOLDOWN_SECONDS"


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


def handle_workspace_init(args: argparse.Namespace) -> int:
    workspace_root = Path(args.workspace)
    config_path = workspace_root / "workspace.yaml"
    if config_path.exists() and not args.force:
        print(
            json.dumps(
                {
                    "ok": False,
                    "errors": [
                        f"workspace already initialized: {config_path}. Use --force to overwrite."
                    ],
                },
                ensure_ascii=False,
                indent=2,
            )
        )
        return 1

    settings = WorkspaceSettings.default(name=args.name)
    config_path = ConfigService(workspace_root).initialize_workspace(
        workspace_root, settings
    )
    print(config_path)
    return 0


def handle_workspace_show_config(args: argparse.Namespace) -> int:
    workspace_root = Path(args.workspace)
    settings = ConfigService(workspace_root).load_workspace_settings(workspace_root)
    print(json.dumps(settings.to_dict(), ensure_ascii=False, indent=2))
    return 0


def handle_workspace_resolve_config(args: argparse.Namespace) -> int:
    workspace_root = Path(args.workspace)
    service = ConfigService(workspace_root)
    resolved = service.resolve_workspace_settings(workspace_root)
    payload = resolved.workspace.to_dict()
    secret_refs = {
        profile.secret_ref
        for profile in resolved.workspace.provider_profiles.values()
        if profile.secret_ref
    }
    secret_refs.update(
        profile.secret_ref
        for profile in resolved.workspace.publish_profiles.values()
        if profile.secret_ref
    )
    secrets = {ref: resolved.secrets.get(ref) for ref in secret_refs}
    payload["resolved_secret_refs"] = sorted(
        key for key, value in secrets.items() if value
    )
    payload["missing_secret_refs"] = sorted(
        key for key, value in secrets.items() if not value
    )
    print(json.dumps(payload, ensure_ascii=False, indent=2))
    return 0


def handle_workspace_doctor(args: argparse.Namespace) -> int:
    workspace_root = Path(args.workspace)
    errors = ConfigService(workspace_root).validate_workspace(workspace_root)
    result = {"ok": not errors, "errors": errors}
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 0 if result["ok"] else 1


def _default_cli_project_root() -> Path:
    return Path(__file__).resolve().parents[3]


def _build_cli_container(project_root: Path) -> ServiceContainer:
    try:
        return build_container(project_root)
    except FileNotFoundError:
        fallback_settings = HubSettings(
            llm=LLMSettings(provider="", model=""),
            workflow=WorkflowSettings(publish_platform="record-only"),
            rewrite=RewriteSettings(enabled=False),
            template=TemplateSettings(root_dir=project_root / "knowledge" / "templates"),
            storage=StorageSettings(root_dir=project_root / "output"),
            publish=PublishSettings(),
        )
        return build_container(project_root, fallback_settings)


def handle_ingestion_import_bundle(args: argparse.Namespace) -> int:
    bundle = json.loads(Path(args.bundle_path).read_text(encoding="utf-8"))
    container = _build_cli_container(Path(args.project_root))
    summary = container.ingestion_service.import_content_hub_bundle(
        bundle=bundle,
        provider_profile=args.provider_profile,
        article_profile=args.article_profile,
        publish_profile=args.publish_profile,
    )
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 0


def _automation_state_path(workspace_root: Path, incoming_dir: Path) -> Path:
    _ = workspace_root
    return incoming_dir / "automation_state.json"


def _automation_daemon_lock_path(incoming_dir: Path) -> Path:
    return incoming_dir / "automation_daemon.lock"


def _automation_stop_signal_path(incoming_dir: Path) -> Path:
    return incoming_dir / "automation_stop.signal"


def _automation_alert_path(incoming_dir: Path) -> Path:
    return incoming_dir / "automation_alert.json"


def _consume_stop_signal_if_present(incoming_dir: Path) -> bool:
    stop_path = _automation_stop_signal_path(incoming_dir)
    if not stop_path.exists():
        return False
    stop_path.unlink()
    return True


def _dispatch_automation_alert_webhook(
    *,
    event_type: str,
    incoming_dir: Path,
    webhook_url: str,
    payload: dict,
) -> None:
    normalized_webhook_url = webhook_url.strip()
    if not normalized_webhook_url:
        return

    event_payload = {
        "event_type": event_type,
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "incoming_dir": str(incoming_dir),
        "payload": payload,
    }

    if normalized_webhook_url.startswith("file://"):
        destination = Path(normalized_webhook_url.removeprefix("file://"))
        destination.parent.mkdir(parents=True, exist_ok=True)
        with destination.open("a", encoding="utf-8") as handle:
            handle.write(json.dumps(event_payload, ensure_ascii=False) + "\n")
        return

    request = urllib.request.Request(
        normalized_webhook_url,
        data=json.dumps(event_payload, ensure_ascii=False).encode("utf-8"),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    try:
        with urllib.request.urlopen(request, timeout=5):
            return
    except Exception:
        return


def _automation_webhook_cooldown_seconds(global_default: int | None = None) -> int:
    raw = os.environ.get(AUTOMATION_ALERT_WEBHOOK_COOLDOWN_ENV)
    if raw is None:
        return global_default if global_default is not None else 300
    try:
        return max(int(raw), 0)
    except ValueError:
        return global_default if global_default is not None else 300


def _write_automation_state_snapshot(
    *,
    command: str,
    workspace_root: Path,
    incoming_dir: Path,
    processed_dir: Path,
    failed_dir: Path,
    provider_profile: str,
    article_profile: str,
    publish_profile: str,
    alert_warning_threshold: int,
    alert_critical_threshold: int,
    alert_webhook_cooldown_seconds: int,
    summary: dict,
) -> None:
    snapshot_path = _automation_state_path(workspace_root, incoming_dir)
    existing: dict = {}
    if snapshot_path.exists():
        try:
            loaded = json.loads(snapshot_path.read_text(encoding="utf-8"))
            if isinstance(loaded, dict):
                existing = loaded
        except json.JSONDecodeError:
            existing = {}

    failed_files = int(summary.get("failed_files", 0))
    had_failures = failed_files > 0
    previous_runs_total = int(existing.get("runs_total", 0))
    previous_failure_runs_total = int(existing.get("failure_runs_total", 0))
    previous_consecutive_failures = int(existing.get("consecutive_failure_runs", 0))
    previous_alert_active = bool(existing.get("alert_active", False))
    previous_alert_severity = str(existing.get("alert_severity", "none"))
    previous_webhook_events_raw = existing.get("last_webhook_events", {})
    previous_webhook_events: dict[str, float] = {}
    if isinstance(previous_webhook_events_raw, dict):
        for key, value in previous_webhook_events_raw.items():
            try:
                previous_webhook_events[str(key)] = float(value)
            except (TypeError, ValueError):
                continue
    webhook_url = str(os.environ.get(AUTOMATION_ALERT_WEBHOOK_ENV, "")).strip()
    webhook_cooldown_seconds = _automation_webhook_cooldown_seconds(
        global_default=alert_webhook_cooldown_seconds
    )

    consecutive_failure_runs = (
        previous_consecutive_failures + 1 if had_failures else 0
    )
    alert_active = consecutive_failure_runs >= alert_warning_threshold
    if consecutive_failure_runs >= alert_critical_threshold:
        alert_severity = "critical"
    elif alert_active:
        alert_severity = "warning"
    else:
        alert_severity = "none"

    previous_recent_runs = existing.get("recent_runs", [])
    if not isinstance(previous_recent_runs, list):
        previous_recent_runs = []
    run_entry = {
        "timestamp": datetime.now(timezone.utc).isoformat(),
        "command": command,
        "scanned_files": int(summary.get("scanned_files", 0)),
        "imported_files": int(summary.get("imported_files", 0)),
        "failed_files": failed_files,
        "had_failures": had_failures,
    }
    recent_runs = [*previous_recent_runs, run_entry][-20:]

    next_webhook_event_type = ""
    if not previous_alert_active and alert_active:
        next_webhook_event_type = (
            "alert_raised_critical" if alert_severity == "critical" else "alert_raised_warning"
        )
    elif previous_alert_active and not alert_active:
        next_webhook_event_type = "alert_recovered"
    elif previous_alert_active and alert_active and previous_alert_severity != alert_severity:
        if alert_severity == "critical":
            next_webhook_event_type = "alert_escalated_critical"
        elif previous_alert_severity == "critical" and alert_severity == "warning":
            next_webhook_event_type = "alert_deescalated_warning"

    now_ts = time.time()
    webhook_dispatched = False
    webhook_suppressed_reason = ""
    current_webhook_events = dict(previous_webhook_events)
    current_webhook_event_type = ""
    current_webhook_event_at = 0.0

    if next_webhook_event_type:
        last_event_time = current_webhook_events.get(next_webhook_event_type, 0.0)
        elapsed = now_ts - last_event_time
        if elapsed >= webhook_cooldown_seconds:
            _dispatch_automation_alert_webhook(
                event_type=next_webhook_event_type,
                incoming_dir=incoming_dir,
                webhook_url=webhook_url,
                payload={
                    "workspace": str(workspace_root),
                    "incoming_dir": str(incoming_dir),
                    "provider_profile": provider_profile,
                    "article_profile": article_profile,
                    "publish_profile": publish_profile,
                    "summary": summary,
                    "consecutive_failure_runs": consecutive_failure_runs,
                    "alert_active": alert_active,
                    "alert_severity": alert_severity,
                },
            )
            webhook_dispatched = bool(webhook_url)
            current_webhook_event_type = next_webhook_event_type
            current_webhook_event_at = now_ts
            current_webhook_events[next_webhook_event_type] = now_ts
        else:
            webhook_suppressed_reason = "cooldown_active"

    payload = {
        "command": command,
        "workspace": str(workspace_root),
        "incoming_dir": str(incoming_dir),
        "processed_dir": str(processed_dir),
        "failed_dir": str(failed_dir),
        "provider_profile": provider_profile,
        "article_profile": article_profile,
        "publish_profile": publish_profile,
        "updated_at": datetime.now(timezone.utc).isoformat(),
        "runs_total": previous_runs_total + 1,
        "failure_runs_total": previous_failure_runs_total + (1 if had_failures else 0),
        "consecutive_failure_runs": consecutive_failure_runs,
        "failure_alert_threshold": alert_warning_threshold,
        "critical_alert_threshold": alert_critical_threshold,
        "alert_active": alert_active,
        "alert_severity": alert_severity,
        "webhook_enabled": bool(webhook_url),
        "webhook_cooldown_seconds": webhook_cooldown_seconds,
        "last_webhook_event_type": current_webhook_event_type,
        "last_webhook_event_at": current_webhook_event_at,
        "last_webhook_events": current_webhook_events,
        "last_webhook_event_dispatched": webhook_dispatched,
        "last_webhook_event_suppressed_reason": webhook_suppressed_reason,
        "recent_runs": recent_runs,
        "summary": summary,
    }
    snapshot_path.parent.mkdir(parents=True, exist_ok=True)
    snapshot_path.write_text(
        json.dumps(payload, ensure_ascii=False, indent=2),
        encoding="utf-8",
    )

    alert_path = _automation_alert_path(incoming_dir)
    if payload["alert_active"]:
        alert_payload = {
            "ok": False,
            "severity": alert_severity,
            "reason": "consecutive_failures_threshold_reached",
            "threshold": alert_warning_threshold,
            "critical_threshold": alert_critical_threshold,
            "consecutive_failure_runs": consecutive_failure_runs,
            "workspace": str(workspace_root),
            "incoming_dir": str(incoming_dir),
            "updated_at": payload["updated_at"],
            "last_summary": summary,
        }
        alert_path.write_text(
            json.dumps(alert_payload, ensure_ascii=False, indent=2),
            encoding="utf-8",
        )
    elif alert_path.exists():
        alert_path.unlink()

    return


def _resolve_automation_run_once_context(args: argparse.Namespace) -> dict:
    workspace_root = Path(args.workspace)
    workspace_settings = ConfigService(workspace_root).load_workspace_settings(workspace_root)
    provider_profile = args.provider_profile or workspace_settings.default_provider_profile
    article_profile = args.article_profile or workspace_settings.default_article_profile
    publish_profile = args.publish_profile or workspace_settings.default_publish_profile
    incoming_dir = (
        Path(args.incoming_dir)
        if args.incoming_dir
        else workspace_root / workspace_settings.paths.incoming_dir
    )
    processed_dir = incoming_dir / "processed"
    failed_dir = incoming_dir / "failed"
    automation_policy = workspace_settings.automation
    alert_warning_threshold = (
        int(automation_policy.alert_warning_threshold)
        if automation_policy.alert_warning_threshold is not None
        else AUTOMATION_FAILURE_ALERT_THRESHOLD
    )
    alert_critical_threshold = (
        int(automation_policy.alert_critical_threshold)
        if automation_policy.alert_critical_threshold is not None
        else AUTOMATION_CRITICAL_ALERT_THRESHOLD
    )
    if alert_critical_threshold < alert_warning_threshold:
        alert_critical_threshold = alert_warning_threshold
    alert_webhook_cooldown_seconds = (
        int(automation_policy.alert_webhook_cooldown_seconds)
        if automation_policy.alert_webhook_cooldown_seconds is not None
        else 300
    )
    return {
        "workspace_root": workspace_root,
        "workspace_settings": workspace_settings,
        "provider_profile": provider_profile,
        "article_profile": article_profile,
        "publish_profile": publish_profile,
        "incoming_dir": incoming_dir,
        "processed_dir": processed_dir,
        "failed_dir": failed_dir,
        "alert_warning_threshold": alert_warning_threshold,
        "alert_critical_threshold": alert_critical_threshold,
        "alert_webhook_cooldown_seconds": alert_webhook_cooldown_seconds,
    }


def _run_automation_import_once(
    *,
    command: str,
    project_root: Path,
    workspace_root: Path,
    incoming_dir: Path,
    processed_dir: Path,
    failed_dir: Path,
    provider_profile: str,
    article_profile: str,
    publish_profile: str,
    alert_warning_threshold: int,
    alert_critical_threshold: int,
    alert_webhook_cooldown_seconds: int,
) -> dict:
    container = _build_cli_container(project_root)
    summary = container.ingestion_service.import_content_hub_bundles_from_directory(
        incoming_dir=incoming_dir,
        provider_profile=provider_profile,
        article_profile=article_profile,
        publish_profile=publish_profile,
        archive_dir=processed_dir,
        failed_dir=failed_dir,
    )
    _write_automation_state_snapshot(
        command=command,
        workspace_root=workspace_root,
        incoming_dir=incoming_dir,
        processed_dir=processed_dir,
        failed_dir=failed_dir,
        provider_profile=provider_profile,
        article_profile=article_profile,
        publish_profile=publish_profile,
        alert_warning_threshold=alert_warning_threshold,
        alert_critical_threshold=alert_critical_threshold,
        alert_webhook_cooldown_seconds=alert_webhook_cooldown_seconds,
        summary=summary,
    )
    return summary


def handle_automation_run_once(args: argparse.Namespace) -> int:
    context = _resolve_automation_run_once_context(args)
    summary = _run_automation_import_once(
        command="run-once",
        project_root=Path(args.project_root),
        workspace_root=context["workspace_root"],
        incoming_dir=context["incoming_dir"],
        processed_dir=context["processed_dir"],
        failed_dir=context["failed_dir"],
        provider_profile=context["provider_profile"],
        article_profile=context["article_profile"],
        publish_profile=context["publish_profile"],
        alert_warning_threshold=context["alert_warning_threshold"],
        alert_critical_threshold=context["alert_critical_threshold"],
        alert_webhook_cooldown_seconds=context["alert_webhook_cooldown_seconds"],
    )
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 1 if int(summary.get("failed_files", 0)) > 0 else 0


def handle_automation_run_daemon(args: argparse.Namespace) -> int:
    context = _resolve_automation_run_once_context(args)
    policy_interval_seconds = getattr(
        context["workspace_settings"].automation,
        "interval_seconds",
        None,
    )
    interval_seconds = (
        int(args.interval_seconds)
        if args.interval_seconds is not None
        else int(policy_interval_seconds) if policy_interval_seconds is not None else 60
    )

    runs_executed = 0
    had_failures = False
    stopped_by_signal = False
    last_summary: dict = {}
    lock_path = _automation_daemon_lock_path(context["incoming_dir"])
    if lock_path.exists():
        print(
            json.dumps(
                {
                    "ok": False,
                    "error": f"automation daemon already running: {lock_path}",
                },
                ensure_ascii=False,
                indent=2,
            )
        )
        return 1

    lock_path.parent.mkdir(parents=True, exist_ok=True)
    lock_path.write_text(str(time.time()), encoding="utf-8")
    try:
        while args.max_runs is None or runs_executed < args.max_runs:
            if _consume_stop_signal_if_present(context["incoming_dir"]):
                stopped_by_signal = True
                break

            summary = _run_automation_import_once(
                command="run-daemon",
                project_root=Path(args.project_root),
                workspace_root=context["workspace_root"],
                incoming_dir=context["incoming_dir"],
                processed_dir=context["processed_dir"],
                failed_dir=context["failed_dir"],
                provider_profile=context["provider_profile"],
                article_profile=context["article_profile"],
                publish_profile=context["publish_profile"],
                alert_warning_threshold=context["alert_warning_threshold"],
                alert_critical_threshold=context["alert_critical_threshold"],
                alert_webhook_cooldown_seconds=context["alert_webhook_cooldown_seconds"],
            )
            runs_executed += 1
            last_summary = summary
            if int(summary.get("failed_files", 0)) > 0:
                had_failures = True

            if args.max_runs is not None and runs_executed >= args.max_runs:
                break
            time.sleep(max(interval_seconds, 0))
            if _consume_stop_signal_if_present(context["incoming_dir"]):
                stopped_by_signal = True
                break
    finally:
        if lock_path.exists():
            lock_path.unlink()

    result = {
        "runs_executed": runs_executed,
        "had_failures": had_failures,
        "last_summary": last_summary,
        "stopped_by_signal": stopped_by_signal,
    }
    print(json.dumps(result, ensure_ascii=False, indent=2))
    return 1 if had_failures else 0


def handle_automation_stop(args: argparse.Namespace) -> int:
    workspace_root = Path(args.workspace)
    workspace_settings = ConfigService(workspace_root).load_workspace_settings(workspace_root)
    incoming_dir = (
        Path(args.incoming_dir)
        if args.incoming_dir
        else workspace_root / workspace_settings.paths.incoming_dir
    )
    signal_path = _automation_stop_signal_path(incoming_dir)
    signal_path.parent.mkdir(parents=True, exist_ok=True)
    signal_path.write_text("stop", encoding="utf-8")
    print(
        json.dumps(
            {
                "ok": True,
                "signal_path": str(signal_path),
            },
            ensure_ascii=False,
            indent=2,
        )
    )
    return 0


def handle_automation_retry_failed(args: argparse.Namespace) -> int:
    workspace_root = Path(args.workspace)
    workspace_settings = ConfigService(workspace_root).load_workspace_settings(workspace_root)

    provider_profile = args.provider_profile or workspace_settings.default_provider_profile
    article_profile = args.article_profile or workspace_settings.default_article_profile
    publish_profile = args.publish_profile or workspace_settings.default_publish_profile

    incoming_dir = workspace_root / workspace_settings.paths.incoming_dir
    failed_dir = Path(args.failed_dir) if args.failed_dir else incoming_dir / "failed"
    processed_dir = incoming_dir / "processed"

    container = _build_cli_container(Path(args.project_root))
    summary = container.ingestion_service.import_content_hub_bundles_from_directory(
        incoming_dir=failed_dir,
        provider_profile=provider_profile,
        article_profile=article_profile,
        publish_profile=publish_profile,
        archive_dir=processed_dir,
    )
    _write_automation_state_snapshot(
        command="retry-failed",
        workspace_root=workspace_root,
        incoming_dir=incoming_dir,
        processed_dir=processed_dir,
        failed_dir=failed_dir,
        provider_profile=provider_profile,
        article_profile=article_profile,
        publish_profile=publish_profile,
        alert_warning_threshold=max(
            int(workspace_settings.automation.alert_warning_threshold)
            if workspace_settings.automation.alert_warning_threshold is not None
            else AUTOMATION_FAILURE_ALERT_THRESHOLD,
            1,
        ),
        alert_critical_threshold=max(
            int(workspace_settings.automation.alert_critical_threshold)
            if workspace_settings.automation.alert_critical_threshold is not None
            else AUTOMATION_CRITICAL_ALERT_THRESHOLD,
            int(workspace_settings.automation.alert_warning_threshold)
            if workspace_settings.automation.alert_warning_threshold is not None
            else AUTOMATION_FAILURE_ALERT_THRESHOLD,
        ),
        alert_webhook_cooldown_seconds=(
            int(workspace_settings.automation.alert_webhook_cooldown_seconds)
            if workspace_settings.automation.alert_webhook_cooldown_seconds is not None
            else 300
        ),
        summary=summary,
    )
    print(json.dumps(summary, ensure_ascii=False, indent=2))
    return 1 if int(summary.get("failed_files", 0)) > 0 else 0


def handle_automation_status(args: argparse.Namespace) -> int:
    workspace_root = Path(args.workspace)
    workspace_settings = ConfigService(workspace_root).load_workspace_settings(workspace_root)
    incoming_dir = (
        Path(args.incoming_dir)
        if args.incoming_dir
        else workspace_root / workspace_settings.paths.incoming_dir
    )
    snapshot_path = _automation_state_path(workspace_root, incoming_dir)
    if not snapshot_path.exists():
        print(
            json.dumps(
                {
                    "ok": False,
                    "error": f"automation state snapshot not found: {snapshot_path}",
                },
                ensure_ascii=False,
                indent=2,
            )
        )
        return 1

    payload = json.loads(snapshot_path.read_text(encoding="utf-8"))
    print(json.dumps(payload, ensure_ascii=False, indent=2))
    return 0


def handle_automation_health(args: argparse.Namespace) -> int:
    workspace_root = Path(args.workspace)
    workspace_settings = ConfigService(workspace_root).load_workspace_settings(workspace_root)
    incoming_dir = (
        Path(args.incoming_dir)
        if args.incoming_dir
        else workspace_root / workspace_settings.paths.incoming_dir
    )
    snapshot_path = _automation_state_path(workspace_root, incoming_dir)
    alert_path = _automation_alert_path(incoming_dir)

    if not snapshot_path.exists():
        payload = {
            "ok": False,
            "status": "missing_state",
            "error": f"automation state snapshot not found: {snapshot_path}",
            "workspace": str(workspace_root),
            "incoming_dir": str(incoming_dir),
            "alert_file_exists": alert_path.exists(),
        }
        print(json.dumps(payload, ensure_ascii=False, indent=2))
        return 1

    snapshot = json.loads(snapshot_path.read_text(encoding="utf-8"))
    recent_runs = snapshot.get("recent_runs", [])
    if not isinstance(recent_runs, list):
        recent_runs = []

    has_alert_file = alert_path.exists()
    alert_payload = None
    if has_alert_file:
        try:
            parsed_alert = json.loads(alert_path.read_text(encoding="utf-8"))
            alert_payload = parsed_alert if isinstance(parsed_alert, dict) else None
        except json.JSONDecodeError:
            alert_payload = {"ok": False, "error": "invalid alert file"}

    updated_at = snapshot.get("updated_at")
    next_run_eta_seconds = None
    if isinstance(updated_at, str):
        try:
            updated_at_dt = datetime.fromisoformat(updated_at)
            now = datetime.now(timezone.utc)
            interval_seconds = (
                int(args.interval_seconds)
                if args.interval_seconds is not None
                else int(getattr(workspace_settings.automation, "interval_seconds", 60) or 60)
            )
            elapsed_seconds = max(int((now - updated_at_dt).total_seconds()), 0)
            next_run_eta_seconds = max(interval_seconds - elapsed_seconds, 0)
        except ValueError:
            next_run_eta_seconds = None

    status = "healthy"
    alert_severity = str(snapshot.get("alert_severity", "none"))
    if has_alert_file or bool(snapshot.get("alert_active", False)):
        status = "warning"
    if alert_severity == "critical":
        status = "critical"

    payload = {
        "ok": status == "healthy",
        "status": status,
        "workspace": str(workspace_root),
        "incoming_dir": str(incoming_dir),
        "updated_at": updated_at,
        "runs_total": int(snapshot.get("runs_total", 0)),
        "failure_runs_total": int(snapshot.get("failure_runs_total", 0)),
        "consecutive_failure_runs": int(snapshot.get("consecutive_failure_runs", 0)),
        "alert_severity": alert_severity,
        "recent_runs_count": len(recent_runs),
        "recent_failures_count": sum(
            1 for item in recent_runs if isinstance(item, dict) and bool(item.get("had_failures"))
        ),
        "alert_active": bool(snapshot.get("alert_active", False)) or has_alert_file,
        "alert_file_exists": has_alert_file,
        "alert": alert_payload,
        "next_run_eta_seconds": next_run_eta_seconds,
    }
    print(json.dumps(payload, ensure_ascii=False, indent=2))
    return 0 if payload["ok"] else 1


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

    workspace_parser = subparsers.add_parser("workspace")
    workspace_subparsers = workspace_parser.add_subparsers(
        dest="workspace_command", required=True
    )

    workspace_init_parser = workspace_subparsers.add_parser("init")
    workspace_init_parser.add_argument("workspace")
    workspace_init_parser.add_argument("--name", default="content-workspace")
    workspace_init_parser.add_argument("--force", action="store_true")
    workspace_init_parser.set_defaults(handler=handle_workspace_init)

    workspace_doctor_parser = workspace_subparsers.add_parser("doctor")
    workspace_doctor_parser.add_argument("workspace")
    workspace_doctor_parser.set_defaults(handler=handle_workspace_doctor)

    workspace_show_parser = workspace_subparsers.add_parser("show-config")
    workspace_show_parser.add_argument("workspace")
    workspace_show_parser.set_defaults(handler=handle_workspace_show_config)

    workspace_resolve_parser = workspace_subparsers.add_parser("resolve-config")
    workspace_resolve_parser.add_argument("workspace")
    workspace_resolve_parser.set_defaults(handler=handle_workspace_resolve_config)

    ingestion_parser = subparsers.add_parser("ingestion")
    ingestion_subparsers = ingestion_parser.add_subparsers(
        dest="ingestion_command", required=True
    )

    ingestion_import_bundle_parser = ingestion_subparsers.add_parser("import-bundle")
    ingestion_import_bundle_parser.add_argument("bundle_path")
    ingestion_import_bundle_parser.add_argument("--provider-profile", required=True)
    ingestion_import_bundle_parser.add_argument("--article-profile", required=True)
    ingestion_import_bundle_parser.add_argument("--publish-profile", required=True)
    ingestion_import_bundle_parser.add_argument(
        "--project-root",
        default=str(_default_cli_project_root()),
    )
    ingestion_import_bundle_parser.set_defaults(handler=handle_ingestion_import_bundle)

    automation_parser = subparsers.add_parser("automation")
    automation_subparsers = automation_parser.add_subparsers(
        dest="automation_command", required=True
    )

    automation_run_once_parser = automation_subparsers.add_parser("run-once")
    automation_run_once_parser.add_argument("workspace")
    automation_run_once_parser.add_argument(
        "--project-root",
        default=str(_default_cli_project_root()),
    )
    automation_run_once_parser.add_argument("--incoming-dir")
    automation_run_once_parser.add_argument("--provider-profile")
    automation_run_once_parser.add_argument("--article-profile")
    automation_run_once_parser.add_argument("--publish-profile")
    automation_run_once_parser.set_defaults(handler=handle_automation_run_once)

    automation_run_daemon_parser = automation_subparsers.add_parser("run-daemon")
    automation_run_daemon_parser.add_argument("workspace")
    automation_run_daemon_parser.add_argument(
        "--project-root",
        default=str(_default_cli_project_root()),
    )
    automation_run_daemon_parser.add_argument("--incoming-dir")
    automation_run_daemon_parser.add_argument("--provider-profile")
    automation_run_daemon_parser.add_argument("--article-profile")
    automation_run_daemon_parser.add_argument("--publish-profile")
    automation_run_daemon_parser.add_argument("--interval-seconds", type=int)
    automation_run_daemon_parser.add_argument("--max-runs", type=int)
    automation_run_daemon_parser.set_defaults(handler=handle_automation_run_daemon)

    automation_retry_failed_parser = automation_subparsers.add_parser("retry-failed")
    automation_retry_failed_parser.add_argument("workspace")
    automation_retry_failed_parser.add_argument(
        "--project-root",
        default=str(_default_cli_project_root()),
    )
    automation_retry_failed_parser.add_argument("--failed-dir")
    automation_retry_failed_parser.add_argument("--provider-profile")
    automation_retry_failed_parser.add_argument("--article-profile")
    automation_retry_failed_parser.add_argument("--publish-profile")
    automation_retry_failed_parser.set_defaults(handler=handle_automation_retry_failed)

    automation_status_parser = automation_subparsers.add_parser("status")
    automation_status_parser.add_argument("workspace")
    automation_status_parser.add_argument("--incoming-dir")
    automation_status_parser.set_defaults(handler=handle_automation_status)

    automation_health_parser = automation_subparsers.add_parser("health")
    automation_health_parser.add_argument("workspace")
    automation_health_parser.add_argument("--incoming-dir")
    automation_health_parser.add_argument("--interval-seconds", type=int)
    automation_health_parser.set_defaults(handler=handle_automation_health)

    automation_stop_parser = automation_subparsers.add_parser("stop")
    automation_stop_parser.add_argument("workspace")
    automation_stop_parser.add_argument("--incoming-dir")
    automation_stop_parser.set_defaults(handler=handle_automation_stop)

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    return args.handler(args)


if __name__ == "__main__":
    raise SystemExit(main())
