from pathlib import Path
import tempfile
import unittest

from content_hub.infrastructure.formatters.template_catalog import FileTemplateCatalog


class TemplateCatalogTestCase(unittest.TestCase):
    def test_template_catalog_lists_all_registered_structured_templates(self) -> None:
        with tempfile.TemporaryDirectory() as tmp_dir:
            root = Path(tmp_dir)
            for name in [
                "breaking-watch",
                "daily-intelligence",
                "deep-analysis",
                "industry-radar",
                "neo-brutalism",
                "product-release",
                "studio-brief",
                "weekly-financial",
            ]:
                (root / f"{name}.html").write_text(f"<html>{name}</html>", encoding="utf-8")

            catalog = FileTemplateCatalog(root)

            self.assertEqual(
                catalog.list_templates(),
                [
                    "breaking-watch",
                    "daily-intelligence",
                    "deep-analysis",
                    "industry-radar",
                    "neo-brutalism",
                    "product-release",
                    "studio-brief",
                    "weekly-financial",
                ],
            )


if __name__ == "__main__":
    unittest.main()
