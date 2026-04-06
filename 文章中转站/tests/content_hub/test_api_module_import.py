import importlib.util
from pathlib import Path
import unittest

try:
    import fastapi  # noqa: F401
except ModuleNotFoundError:  # pragma: no cover - environment dependent
    fastapi = None


@unittest.skipIf(fastapi is None, "fastapi is not installed in the current environment")
class ApiModuleImportTestCase(unittest.TestCase):
    def test_api_module_can_be_imported_without_triggering_default_container_load(self) -> None:
        module_path = Path(__file__).resolve().parents[2] / "src" / "content_hub" / "interfaces" / "api" / "main.py"
        spec = importlib.util.spec_from_file_location("content_hub_api_main_test", module_path)
        self.assertIsNotNone(spec)
        module = importlib.util.module_from_spec(spec)
        loader = spec.loader
        self.assertIsNotNone(loader)
        loader.exec_module(module)

        self.assertTrue(hasattr(module, "create_app"))
        self.assertTrue(callable(module.create_app))


if __name__ == "__main__":
    unittest.main()
