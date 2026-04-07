from __future__ import annotations

from pathlib import Path


class FileTemplateCatalog:
    def __init__(self, root_dir: Path):
        self.root_dir = root_dir

    def list_templates(self) -> list[str]:
        if not self.root_dir.exists():
            return []
        return sorted(path.stem for path in self.root_dir.glob("*.html"))

    def read_template(self, template_name: str) -> str:
        path = self.root_dir / f"{template_name}.html"
        return path.read_text(encoding="utf-8")
