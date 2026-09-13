"""Generator boundary tests; compilation/install evidence is checked separately."""
import argparse
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import tempfile
import unittest
import sys

sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("create_plugin", Path(__file__).with_name("create-plugin.py"))
generator = importlib.util.module_from_spec(spec)
spec.loader.exec_module(generator)


class ProjectCreation(unittest.TestCase):
    def args(self, output, panel=False):
        return argparse.Namespace(output=str(output), module="example.test/my-plugin", namespace="https://plugins.example.test/author", slug="my-plugin", name='中文 "插件"', sdk_version="v0.0.0-local.h1234", with_panel=panel, panel_port=18760)

    def create(self, args):
        with contextlib.redirect_stdout(io.StringIO()):
            generator.create(args)

    def test_node_and_panel_are_independent_modules(self):
        with tempfile.TemporaryDirectory() as temp:
            for panel in [False, True]:
                output = Path(temp) / str(panel)
                self.create(self.args(output, panel))
                metadata = json.loads((output / "plugin.json").read_text(encoding="utf-8"))
                self.assertEqual(metadata["name"], '中文 "插件"')
                self.assertEqual(metadata["panel"], panel)
                self.assertEqual((output / "cmd/companion/main.go").exists(), panel)
                self.assertNotIn("replace ", (output / "go.mod").read_text())
                for source in output.rglob("*.go"):
                    text = source.read_text(encoding="utf-8")
                    self.assertNotIn("@@", text)
                    self.assertNotIn('"github.com/yottaapp/yotta/internal/', text)

    def test_refuses_existing_project_without_modification(self):
        with tempfile.TemporaryDirectory() as temp:
            sentinel = Path(temp) / "keep.txt"
            sentinel.write_text("user work")
            with self.assertRaises(ValueError):
                self.create(self.args(temp))
            self.assertEqual(sentinel.read_text(), "user work")
            self.assertEqual(len(list(Path(temp).iterdir())), 1)

    def test_invalid_inputs_do_not_create_partial_project(self):
        with tempfile.TemporaryDirectory() as temp:
            for field, value in [("module", 'bad"/module'), ("slug", "../escape"), ("namespace", "not-a-namespace"), ("sdk_version", "latest"), ("panel_port", 80)]:
                output = Path(temp) / field
                args = self.args(output)
                setattr(args, field, value)
                with self.assertRaises(ValueError):
                    self.create(args)
                self.assertFalse(output.exists())


if __name__ == "__main__":
    unittest.main()
