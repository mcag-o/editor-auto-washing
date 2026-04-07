from __future__ import annotations

import json
import os
import subprocess
import tempfile
import unittest
from pathlib import Path


class WorkspaceCliTestCase(unittest.TestCase):
    def _run_cli(
        self,
        args: list[str],
        extra_env: dict[str, str] | None = None,
    ) -> subprocess.CompletedProcess[str]:
        env = os.environ.copy()
        env["PYTHONPATH"] = str(Path(__file__).resolve().parents[2] / "src")
        if extra_env:
            env.update(extra_env)
        return subprocess.run(
            ["python3", "-m", "content_hub.interfaces.cli", *args],
            cwd=Path(__file__).resolve().parents[3],
            env=env,
            capture_output=True,
            text=True,
        )

    def test_workspace_init_creates_workspace_yaml_and_directories(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"

            result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertTrue((workspace_root / "workspace.yaml").exists())
            self.assertTrue((workspace_root / "incoming").is_dir())
            self.assertIn("workspace.yaml", result.stdout)

    def test_workspace_show_config_outputs_json(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            result = self._run_cli(["workspace", "show-config", str(workspace_root)])

            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout)
            self.assertEqual(payload["name"], "cloud-writer")

    def test_workspace_doctor_reports_missing_provider_secret(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            result = self._run_cli(["workspace", "doctor", str(workspace_root)])

            self.assertEqual(result.returncode, 1)
            payload = json.loads(result.stdout)
            self.assertFalse(payload["ok"])
            self.assertTrue(any("missing secret" in message for message in payload["errors"]))

    def test_workspace_resolve_config_reports_resolved_and_missing_secret_refs(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)
            (workspace_root / "secrets.yaml").write_text(
                "wechat:\n  main: local-wechat-secret\n",
                encoding="utf-8",
            )

            result = self._run_cli(
                ["workspace", "resolve-config", str(workspace_root)],
                extra_env={"LLM_API_KEY": "env-llm-secret"},
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout)
            self.assertEqual(payload["name"], "cloud-writer")
            self.assertEqual(payload["resolved_secret_refs"], ["env.LLM_API_KEY", "wechat.main"])
            self.assertEqual(payload["missing_secret_refs"], [])

    def test_workspace_resolve_config_reports_missing_file_secret_refs(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            result = self._run_cli(["workspace", "resolve-config", str(workspace_root)])

            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout)
            self.assertEqual(payload["resolved_secret_refs"], [])
            self.assertEqual(
                payload["missing_secret_refs"],
                ["env.LLM_API_KEY", "wechat.main"],
            )

    def test_workspace_init_requires_force_to_overwrite_existing_config(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            first_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "first-name"]
            )
            self.assertEqual(first_result.returncode, 0, first_result.stderr)

            second_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "second-name"]
            )

            self.assertEqual(second_result.returncode, 1)
            payload = json.loads(second_result.stdout)
            self.assertFalse(payload["ok"])
            self.assertTrue(any("Use --force to overwrite" in message for message in payload["errors"]))

            forced_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "second-name", "--force"]
            )

            self.assertEqual(forced_result.returncode, 0, forced_result.stderr)
            show_result = self._run_cli(["workspace", "show-config", str(workspace_root)])
            self.assertEqual(show_result.returncode, 0, show_result.stderr)
            payload = json.loads(show_result.stdout)
            self.assertEqual(payload["name"], "second-name")

    def test_automation_run_once_imports_workspace_incoming_with_default_profiles(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            workspace_config_path = workspace_root / "workspace.yaml"
            workspace_config = workspace_config_path.read_text(encoding="utf-8")
            workspace_config_path.write_text(
                workspace_config.replace("incoming_dir: incoming", "incoming_dir: queued"),
                encoding="utf-8",
            )

            incoming_dir = workspace_root / "queued"
            incoming_dir.mkdir(parents=True, exist_ok=True)
            (incoming_dir / "bundle.json").write_text(
                json.dumps(
                    {
                        "items": [
                            {"url": "https://example.com/1", "title": "Topic 1"},
                            {"url": "https://example.com/2", "title": "Topic 2"},
                        ]
                    }
                ),
                encoding="utf-8",
            )

            result = self._run_cli(
                [
                    "automation",
                    "run-once",
                    str(workspace_root),
                    "--project-root",
                    str(workspace_root),
                ]
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout)
            self.assertEqual(payload["scanned_files"], 1)
            self.assertEqual(payload["imported_files"], 1)
            self.assertEqual(payload["failed_files"], 0)
            self.assertEqual(payload["total_imported_items"], 2)
            self.assertTrue((incoming_dir / "processed" / "bundle.json").exists())
            state_payload = json.loads((incoming_dir / "automation_state.json").read_text(encoding="utf-8"))
            self.assertEqual(state_payload["command"], "run-once")

    def test_automation_run_once_routes_success_and_failed_files_to_default_directories(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            incoming_dir = workspace_root / "incoming"
            (incoming_dir / "ok.json").write_text(
                json.dumps({"items": [{"url": "https://example.com/ok", "title": "OK"}]}),
                encoding="utf-8",
            )
            (incoming_dir / "bad.json").write_text("{not-valid-json", encoding="utf-8")

            result = self._run_cli(
                [
                    "automation",
                    "run-once",
                    str(workspace_root),
                    "--project-root",
                    str(workspace_root),
                ]
            )

            self.assertEqual(result.returncode, 1)
            payload = json.loads(result.stdout)
            self.assertEqual(payload["scanned_files"], 2)
            self.assertEqual(payload["imported_files"], 1)
            self.assertEqual(payload["failed_files"], 1)
            self.assertTrue((incoming_dir / "processed" / "ok.json").exists())
            self.assertTrue((incoming_dir / "failed" / "bad.json").exists())
            self.assertFalse((incoming_dir / "ok.json").exists())
            self.assertFalse((incoming_dir / "bad.json").exists())

    def test_automation_run_once_returns_non_zero_for_malformed_bundle(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            incoming_dir = workspace_root / "incoming"
            (incoming_dir / "ok.json").write_text(
                json.dumps({"items": [{"url": "https://example.com/ok", "title": "OK"}]}),
                encoding="utf-8",
            )
            (incoming_dir / "bad.json").write_text("{not-valid-json", encoding="utf-8")

            result = self._run_cli(
                [
                    "automation",
                    "run-once",
                    str(workspace_root),
                    "--project-root",
                    str(workspace_root),
                ]
            )

            self.assertEqual(result.returncode, 1)
            payload = json.loads(result.stdout)
            self.assertEqual(payload["scanned_files"], 2)
            self.assertEqual(payload["imported_files"], 1)
            self.assertEqual(payload["failed_files"], 1)

    def test_automation_run_daemon_executes_one_successful_run(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            incoming_dir = workspace_root / "incoming"
            (incoming_dir / "bundle.json").write_text(
                json.dumps({"items": [{"url": "https://example.com/ok", "title": "OK"}]}),
                encoding="utf-8",
            )

            result = self._run_cli(
                [
                    "automation",
                    "run-daemon",
                    str(workspace_root),
                    "--project-root",
                    str(workspace_root),
                    "--interval-seconds",
                    "0",
                    "--max-runs",
                    "1",
                ]
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout)
            self.assertEqual(payload["runs_executed"], 1)
            self.assertFalse(payload["had_failures"])
            self.assertEqual(payload["last_summary"]["scanned_files"], 1)
            self.assertEqual(payload["last_summary"]["imported_files"], 1)
            self.assertEqual(payload["last_summary"]["failed_files"], 0)

    def test_automation_run_daemon_returns_non_zero_when_any_run_has_failures(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            incoming_dir = workspace_root / "incoming"
            (incoming_dir / "bad.json").write_text("{not-valid-json", encoding="utf-8")

            result = self._run_cli(
                [
                    "automation",
                    "run-daemon",
                    str(workspace_root),
                    "--project-root",
                    str(workspace_root),
                    "--interval-seconds",
                    "0",
                    "--max-runs",
                    "1",
                ]
            )

            self.assertEqual(result.returncode, 1)
            payload = json.loads(result.stdout)
            self.assertEqual(payload["runs_executed"], 1)
            self.assertTrue(payload["had_failures"])
            self.assertEqual(payload["last_summary"]["failed_files"], 1)

    def test_automation_run_daemon_returns_non_zero_when_lock_file_exists(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            incoming_dir = workspace_root / "incoming"
            lock_path = incoming_dir / "automation_daemon.lock"
            lock_path.write_text("already-running", encoding="utf-8")

            result = self._run_cli(
                [
                    "automation",
                    "run-daemon",
                    str(workspace_root),
                    "--project-root",
                    str(workspace_root),
                    "--interval-seconds",
                    "0",
                    "--max-runs",
                    "1",
                ]
            )

            self.assertEqual(result.returncode, 1)
            payload = json.loads(result.stdout)
            self.assertFalse(payload["ok"])
            self.assertIn("already running", payload["error"])

    def test_automation_stop_creates_stop_signal_file(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            result = self._run_cli(["automation", "stop", str(workspace_root)])

            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout)
            self.assertTrue((workspace_root / "incoming" / "automation_stop.signal").exists())
            self.assertEqual(
                payload["signal_path"],
                str(workspace_root / "incoming" / "automation_stop.signal"),
            )

    def test_automation_run_daemon_consumes_existing_stop_signal_and_exits(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            incoming_dir = workspace_root / "incoming"
            stop_signal_path = incoming_dir / "automation_stop.signal"
            stop_signal_path.write_text("stop", encoding="utf-8")

            result = self._run_cli(
                [
                    "automation",
                    "run-daemon",
                    str(workspace_root),
                    "--project-root",
                    str(workspace_root),
                    "--interval-seconds",
                    "0",
                    "--max-runs",
                    "5",
                ]
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout)
            self.assertTrue(payload["stopped_by_signal"])
            self.assertEqual(payload["runs_executed"], 0)
            self.assertFalse(stop_signal_path.exists())

    def test_automation_retry_failed_retries_failed_directory_and_archives_successful_files(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            failed_dir = workspace_root / "incoming" / "failed"
            failed_dir.mkdir(parents=True, exist_ok=True)
            (failed_dir / "retry.json").write_text(
                json.dumps({"items": [{"url": "https://example.com/retry", "title": "Retry"}]}),
                encoding="utf-8",
            )

            result = self._run_cli(
                [
                    "automation",
                    "retry-failed",
                    str(workspace_root),
                    "--project-root",
                    str(workspace_root),
                ]
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            payload = json.loads(result.stdout)
            self.assertEqual(payload["scanned_files"], 1)
            self.assertEqual(payload["imported_files"], 1)
            self.assertEqual(payload["failed_files"], 0)
            self.assertTrue((workspace_root / "incoming" / "processed" / "retry.json").exists())
            self.assertFalse((failed_dir / "retry.json").exists())
            state_payload = json.loads(
                (workspace_root / "incoming" / "automation_state.json").read_text(encoding="utf-8")
            )
            self.assertEqual(state_payload["command"], "retry-failed")

    def test_automation_status_prints_latest_snapshot_after_run_once(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            incoming_dir = workspace_root / "incoming"
            (incoming_dir / "bundle.json").write_text(
                json.dumps({"items": [{"url": "https://example.com/ok", "title": "OK"}]}),
                encoding="utf-8",
            )

            run_result = self._run_cli(
                [
                    "automation",
                    "run-once",
                    str(workspace_root),
                    "--project-root",
                    str(workspace_root),
                ]
            )
            self.assertEqual(run_result.returncode, 0, run_result.stderr)

            status_result = self._run_cli(["automation", "status", str(workspace_root)])

            self.assertEqual(status_result.returncode, 0, status_result.stderr)
            payload = json.loads(status_result.stdout)
            self.assertEqual(payload["command"], "run-once")
            self.assertEqual(payload["summary"]["imported_files"], 1)
            self.assertEqual(payload["runs_total"], 1)
            self.assertEqual(payload["failure_runs_total"], 0)
            self.assertEqual(payload["consecutive_failure_runs"], 0)
            self.assertEqual(len(payload["recent_runs"]), 1)

    def test_automation_state_tracks_failure_streak_and_total_runs(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            incoming_dir = workspace_root / "incoming"

            (incoming_dir / "bad-1.json").write_text("{not-valid-json", encoding="utf-8")
            first = self._run_cli(
                [
                    "automation",
                    "run-once",
                    str(workspace_root),
                    "--project-root",
                    str(workspace_root),
                ]
            )
            self.assertEqual(first.returncode, 1)

            (incoming_dir / "failed" / "bad-1.json").write_text(
                json.dumps({"items": [{"url": "https://example.com/recover", "title": "Recover"}]}),
                encoding="utf-8",
            )
            second = self._run_cli(
                [
                    "automation",
                    "retry-failed",
                    str(workspace_root),
                    "--project-root",
                    str(workspace_root),
                ]
            )
            self.assertEqual(second.returncode, 0, second.stderr)

            status_result = self._run_cli(["automation", "status", str(workspace_root)])
            self.assertEqual(status_result.returncode, 0, status_result.stderr)
            payload = json.loads(status_result.stdout)
            self.assertEqual(payload["runs_total"], 2)
            self.assertEqual(payload["failure_runs_total"], 1)
            self.assertEqual(payload["consecutive_failure_runs"], 0)
            self.assertEqual(len(payload["recent_runs"]), 2)

    def test_automation_alert_file_created_after_consecutive_failures_and_cleared_on_success(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            incoming_dir = workspace_root / "incoming"
            alert_path = incoming_dir / "automation_alert.json"

            for index in range(3):
                (incoming_dir / f"bad-{index}.json").write_text(
                    "{not-valid-json",
                    encoding="utf-8",
                )
                run_result = self._run_cli(
                    [
                        "automation",
                        "run-once",
                        str(workspace_root),
                        "--project-root",
                        str(workspace_root),
                    ]
                )
                self.assertEqual(run_result.returncode, 1)

            self.assertTrue(alert_path.exists())
            alert_payload = json.loads(alert_path.read_text(encoding="utf-8"))
            self.assertEqual(alert_payload["reason"], "consecutive_failures_threshold_reached")
            self.assertGreaterEqual(alert_payload["consecutive_failure_runs"], 3)

            (incoming_dir / "recovery.json").write_text(
                json.dumps(
                    {
                        "items": [
                            {
                                "url": "https://example.com/recovery",
                                "title": "Recovery",
                            }
                        ]
                    }
                ),
                encoding="utf-8",
            )
            recover_result = self._run_cli(
                [
                    "automation",
                    "run-once",
                    str(workspace_root),
                    "--project-root",
                    str(workspace_root),
                ]
            )
            self.assertEqual(recover_result.returncode, 0, recover_result.stderr)
            self.assertFalse(alert_path.exists())

    def test_automation_alert_webhook_file_receives_raise_and_recover_events(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            incoming_dir = workspace_root / "incoming"
            webhook_log = workspace_root / "webhook-events.ndjson"
            extra_env = {
                "CONTENT_HUB_AUTOMATION_ALERT_WEBHOOK_URL": f"file://{webhook_log}",
                "CONTENT_HUB_AUTOMATION_ALERT_WEBHOOK_COOLDOWN_SECONDS": "0",
            }

            for index in range(3):
                (incoming_dir / f"bad-webhook-{index}.json").write_text(
                    "{not-valid-json",
                    encoding="utf-8",
                )
                run_result = self._run_cli(
                    [
                        "automation",
                        "run-once",
                        str(workspace_root),
                        "--project-root",
                        str(workspace_root),
                    ],
                    extra_env=extra_env,
                )
                self.assertEqual(run_result.returncode, 1)

            self.assertTrue(webhook_log.exists())
            lines = [line for line in webhook_log.read_text(encoding="utf-8").splitlines() if line]
            self.assertGreaterEqual(len(lines), 1)
            first_event = json.loads(lines[0])
            self.assertEqual(first_event["event_type"], "alert_raised_warning")

            (incoming_dir / "recover-webhook.json").write_text(
                json.dumps(
                    {
                        "items": [
                            {
                                "url": "https://example.com/recover-webhook",
                                "title": "Recover Webhook",
                            }
                        ]
                    }
                ),
                encoding="utf-8",
            )
            recover_result = self._run_cli(
                [
                    "automation",
                    "run-once",
                    str(workspace_root),
                    "--project-root",
                    str(workspace_root),
                ],
                extra_env=extra_env,
            )
            self.assertEqual(recover_result.returncode, 0, recover_result.stderr)

            final_lines = [line for line in webhook_log.read_text(encoding="utf-8").splitlines() if line]
            self.assertGreaterEqual(len(final_lines), 2)
            last_event = json.loads(final_lines[-1])
            self.assertEqual(last_event["event_type"], "alert_recovered")

    def test_automation_webhook_cooldown_is_scoped_per_event_type(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            incoming_dir = workspace_root / "incoming"
            webhook_log = workspace_root / "webhook-cooldown.ndjson"
            extra_env = {
                "CONTENT_HUB_AUTOMATION_ALERT_WEBHOOK_URL": f"file://{webhook_log}",
                "CONTENT_HUB_AUTOMATION_ALERT_WEBHOOK_COOLDOWN_SECONDS": "600",
            }

            for index in range(3):
                (incoming_dir / f"bad-cooldown-{index}.json").write_text(
                    "{not-valid-json",
                    encoding="utf-8",
                )
                run_result = self._run_cli(
                    [
                        "automation",
                        "run-once",
                        str(workspace_root),
                        "--project-root",
                        str(workspace_root),
                    ],
                    extra_env=extra_env,
                )
                self.assertEqual(run_result.returncode, 1)

            (incoming_dir / "recovery-cooldown.json").write_text(
                json.dumps(
                    {
                        "items": [
                            {
                                "url": "https://example.com/recovery-cooldown",
                                "title": "Recovery Cooldown",
                            }
                        ]
                    }
                ),
                encoding="utf-8",
            )
            recover_result = self._run_cli(
                [
                    "automation",
                    "run-once",
                    str(workspace_root),
                    "--project-root",
                    str(workspace_root),
                ],
                extra_env=extra_env,
            )
            self.assertEqual(recover_result.returncode, 0, recover_result.stderr)

            lines = [line for line in webhook_log.read_text(encoding="utf-8").splitlines() if line]
            event_types = [json.loads(line)["event_type"] for line in lines]
            self.assertEqual(event_types, ["alert_raised_warning", "alert_recovered"])

            status_result = self._run_cli(["automation", "status", str(workspace_root)])
            self.assertEqual(status_result.returncode, 0, status_result.stderr)
            status_payload = json.loads(status_result.stdout)
            self.assertEqual(status_payload["last_webhook_event_suppressed_reason"], "")

    def test_automation_alert_escalates_to_critical_after_six_consecutive_failures(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            incoming_dir = workspace_root / "incoming"
            webhook_log = workspace_root / "webhook-critical.ndjson"
            extra_env = {
                "CONTENT_HUB_AUTOMATION_ALERT_WEBHOOK_URL": f"file://{webhook_log}",
                "CONTENT_HUB_AUTOMATION_ALERT_WEBHOOK_COOLDOWN_SECONDS": "0",
            }

            for index in range(6):
                (incoming_dir / f"bad-critical-{index}.json").write_text(
                    "{not-valid-json",
                    encoding="utf-8",
                )
                run_result = self._run_cli(
                    [
                        "automation",
                        "run-once",
                        str(workspace_root),
                        "--project-root",
                        str(workspace_root),
                    ],
                    extra_env=extra_env,
                )
                self.assertEqual(run_result.returncode, 1)

            lines = [line for line in webhook_log.read_text(encoding="utf-8").splitlines() if line]
            event_types = [json.loads(line)["event_type"] for line in lines]
            self.assertIn("alert_raised_warning", event_types)
            self.assertIn("alert_escalated_critical", event_types)

            health_result = self._run_cli(["automation", "health", str(workspace_root)])
            self.assertEqual(health_result.returncode, 1)
            payload = json.loads(health_result.stdout)
            self.assertEqual(payload["status"], "critical")
            self.assertEqual(payload["alert_severity"], "critical")

    def test_automation_alert_thresholds_can_be_overridden_by_workspace_config(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            config_path = workspace_root / "workspace.yaml"
            config_payload = json.loads(
                self._run_cli(["workspace", "show-config", str(workspace_root)]).stdout
            )
            config_payload["automation"]["alert_warning_threshold"] = 2
            config_payload["automation"]["alert_critical_threshold"] = 4
            config_payload["automation"]["alert_webhook_cooldown_seconds"] = 1

            import yaml

            config_path.write_text(
                yaml.safe_dump(config_payload, allow_unicode=True, sort_keys=False),
                encoding="utf-8",
            )

            incoming_dir = workspace_root / "incoming"
            for index in range(2):
                (incoming_dir / f"bad-override-{index}.json").write_text(
                    "{not-valid-json",
                    encoding="utf-8",
                )
                run_result = self._run_cli(
                    [
                        "automation",
                        "run-once",
                        str(workspace_root),
                        "--project-root",
                        str(workspace_root),
                    ]
                )
                self.assertEqual(run_result.returncode, 1)

            health_result = self._run_cli(["automation", "health", str(workspace_root)])
            self.assertEqual(health_result.returncode, 1)
            payload = json.loads(health_result.stdout)
            self.assertEqual(payload["status"], "warning")
            self.assertEqual(payload["consecutive_failure_runs"], 2)

            status_result = self._run_cli(["automation", "status", str(workspace_root)])
            self.assertEqual(status_result.returncode, 0)
            status_payload = json.loads(status_result.stdout)
            self.assertEqual(status_payload["failure_alert_threshold"], 2)
            self.assertEqual(status_payload["critical_alert_threshold"], 4)
            self.assertEqual(status_payload["webhook_cooldown_seconds"], 1)

    def test_automation_status_returns_non_zero_when_snapshot_missing(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            result = self._run_cli(["automation", "status", str(workspace_root)])

            self.assertEqual(result.returncode, 1)
            payload = json.loads(result.stdout)
            self.assertFalse(payload["ok"])
            self.assertIn("automation state snapshot not found", payload["error"])

    def test_automation_health_reports_healthy_after_successful_run(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            incoming_dir = workspace_root / "incoming"
            (incoming_dir / "bundle.json").write_text(
                json.dumps({"items": [{"url": "https://example.com/ok", "title": "OK"}]}),
                encoding="utf-8",
            )

            run_result = self._run_cli(
                [
                    "automation",
                    "run-once",
                    str(workspace_root),
                    "--project-root",
                    str(workspace_root),
                ]
            )
            self.assertEqual(run_result.returncode, 0, run_result.stderr)

            health_result = self._run_cli(["automation", "health", str(workspace_root)])
            self.assertEqual(health_result.returncode, 0, health_result.stderr)
            payload = json.loads(health_result.stdout)
            self.assertTrue(payload["ok"])
            self.assertEqual(payload["status"], "healthy")
            self.assertFalse(payload["alert_active"])

    def test_automation_health_reports_warning_when_alert_file_exists(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            incoming_dir = workspace_root / "incoming"
            for index in range(3):
                (incoming_dir / f"bad-{index}.json").write_text(
                    "{not-valid-json",
                    encoding="utf-8",
                )
                run_result = self._run_cli(
                    [
                        "automation",
                        "run-once",
                        str(workspace_root),
                        "--project-root",
                        str(workspace_root),
                    ]
                )
                self.assertEqual(run_result.returncode, 1)

            health_result = self._run_cli(["automation", "health", str(workspace_root)])
            self.assertEqual(health_result.returncode, 1)
            payload = json.loads(health_result.stdout)
            self.assertFalse(payload["ok"])
            self.assertEqual(payload["status"], "warning")
            self.assertTrue(payload["alert_active"])
            self.assertTrue(payload["alert_file_exists"])

    def test_automation_health_returns_non_zero_when_state_missing(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            workspace_root = Path(tmp_dir) / "workspace"
            init_result = self._run_cli(
                ["workspace", "init", str(workspace_root), "--name", "cloud-writer"]
            )
            self.assertEqual(init_result.returncode, 0, init_result.stderr)

            result = self._run_cli(["automation", "health", str(workspace_root)])
            self.assertEqual(result.returncode, 1)
            payload = json.loads(result.stdout)
            self.assertFalse(payload["ok"])
            self.assertEqual(payload["status"], "missing_state")


if __name__ == "__main__":
    unittest.main()
