from pathlib import Path
import json
import subprocess
import tempfile
import unittest


class ContentHubCliTestCase(unittest.TestCase):
    def _write_article(self, root: Path) -> Path:
        article_path = root / "article.sample.json"
        article_path.write_text(
            json.dumps(
                {
                    "article_id": "article-1",
                    "template": "daily-intelligence",
                    "meta": {"title": "AI 日报测试", "digest": "这里是一段摘要", "date": "2026-03-31"},
                    "headline": {"title": "头条标题", "body": ["正文"], "source": "来源"},
                    "sections": [{"cn": "要闻", "en": "BRIEFING", "blocks": [{"type": "card", "title": "新闻标题", "body": ["正文"], "source": "来源"}]}],
                    "conclusion": "结论",
                    "cta": "欢迎留言",
                    "source_refs": [],
                    "target_platforms": ["wechat"],
                },
                ensure_ascii=False,
                indent=2,
            ),
            encoding="utf-8",
        )
        return article_path

    def test_render_command_writes_html_output(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            article_path = self._write_article(tmp_path)
            template_root = tmp_path / "templates"
            template_root.mkdir(parents=True)
            (template_root / "daily-intelligence.html").write_text(
                "<html>{{TITLE}}{{HEADLINE_TITLE}}{{HEADLINE_BODY}}{{BODY_SECTIONS}}</html>",
                encoding="utf-8",
            )
            output_path = tmp_path / ".tmp" / "article.html"

            result = subprocess.run(
                [
                    "python3",
                    "-m",
                    "content_hub.interfaces.cli",
                    "render",
                    str(article_path),
                    "-o",
                    str(output_path),
                    "--check",
                    "--template-root",
                    str(template_root),
                ],
                cwd=Path(__file__).resolve().parents[3],
                env={"PYTHONPATH": str(Path(__file__).resolve().parents[2] / "src")},
                capture_output=True,
                text=True,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertTrue(output_path.exists())
            self.assertIn("头条标题", output_path.read_text(encoding="utf-8"))

    def test_validate_command_prints_ok_for_valid_article(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            article_path = self._write_article(tmp_path)
            template_root = tmp_path / "templates"
            template_root.mkdir(parents=True)
            (template_root / "daily-intelligence.html").write_text(
                "<html>{{TITLE}}{{HEADLINE_TITLE}}{{HEADLINE_BODY}}{{BODY_SECTIONS}}</html>",
                encoding="utf-8",
            )

            result = subprocess.run(
                [
                    "python3",
                    "-m",
                    "content_hub.interfaces.cli",
                    "validate",
                    str(article_path),
                    "--template-root",
                    str(template_root),
                ],
                cwd=Path(__file__).resolve().parents[3],
                env={"PYTHONPATH": str(Path(__file__).resolve().parents[2] / "src")},
                capture_output=True,
                text=True,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn('"ok": true', result.stdout)

    def test_pipeline_command_prints_summary_json_in_dry_run_mode(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            tmp_path = Path(tmp_dir)
            article_path = self._write_article(tmp_path)
            template_root = tmp_path / "templates"
            template_root.mkdir(parents=True)
            (template_root / "daily-intelligence.html").write_text(
                "<html>{{TITLE}}{{HEADLINE_TITLE}}{{HEADLINE_BODY}}{{BODY_SECTIONS}}</html>",
                encoding="utf-8",
            )
            output_dir = tmp_path / ".tmp" / "cli-pipeline"

            result = subprocess.run(
                [
                    "python3",
                    "-m",
                    "content_hub.interfaces.cli",
                    "pipeline",
                    str(article_path),
                    "--output-dir",
                    str(output_dir),
                    "--dry-run",
                    "--template-root",
                    str(template_root),
                ],
                cwd=Path(__file__).resolve().parents[3],
                env={"PYTHONPATH": str(Path(__file__).resolve().parents[2] / "src")},
                capture_output=True,
                text=True,
            )

            self.assertEqual(result.returncode, 0, result.stderr)
            self.assertIn('"html"', result.stdout)
            self.assertIn('"resolved_article"', result.stdout)


if __name__ == "__main__":
    unittest.main()
