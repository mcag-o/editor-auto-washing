from pathlib import Path
import unittest


class DocsIntegrationTestCase(unittest.TestCase):
    def test_root_readme_points_structured_article_entrypoint_to_content_hub(self) -> None:
        readme = Path(__file__).resolve().parents[3] / "README.md"
        source = readme.read_text(encoding="utf-8")

        self.assertIn("结构化文章能力已逐步并入 `文章中转站/`", source)
        self.assertIn("优先进入 `文章中转站/`", source)

    def test_content_hub_readme_mentions_python_cli_and_structured_templates(self) -> None:
        readme = Path(__file__).resolve().parents[3] / "文章中转站" / "README.md"
        source = readme.read_text(encoding="utf-8")

        self.assertIn("content_hub.interfaces.cli", source)
        self.assertIn("knowledge/structured_templates/", source)


if __name__ == "__main__":
    unittest.main()
