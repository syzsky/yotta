"""Create an independent plugin project from the public SDK templates."""
import argparse
import json
from pathlib import Path
import re
from urllib.parse import urlsplit


def create(args):
    if not re.fullmatch(r"[a-z][a-z0-9-]{0,47}", args.slug):
        raise ValueError("slug must contain lowercase letters, digits or hyphens")
    if not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._/-]+", args.module) or any(
        part in ("", ".", "..") for part in args.module.split("/")
    ):
        raise ValueError("module must be a Go module path")
    namespace = urlsplit(args.namespace)
    if namespace.scheme != "https" or not namespace.netloc or namespace.query or namespace.fragment or namespace.username:
        raise ValueError("namespace must be the publisher's absolute HTTPS namespace")
    if not re.fullmatch(r"v\d+\.\d+\.\d+(?:-[A-Za-z0-9.-]+)?", args.sdk_version):
        raise ValueError("sdk-version must be an explicit Go module version")
    if not args.name.strip() or len(args.name) > 64:
        raise ValueError("name must contain 1 to 64 characters")
    if not 1024 <= args.panel_port <= 65535:
        raise ValueError("panel-port must be between 1024 and 65535")
    output = Path(args.output).resolve()
    if output.exists():
        raise ValueError("output already exists; choose a new project directory")
    root = Path(__file__).resolve().parent.parent
    go_version = re.search(r"^go (\S+)$", (root / "go.mod").read_text(), re.M).group(1)
    values = {"@@MODULE@@": args.module, "@@SLUG@@": args.slug}
    files = {}
    template = root / "sdk/plugin/templates"
    modes = ["process", "panel"] if args.with_panel else ["process"]
    for mode in modes:
        for source in (template / mode).rglob("*.tmpl"):
            text = source.read_text(encoding="utf-8")
            for token, value in values.items():
                text = text.replace(token, value)
            files[source.relative_to(template / mode).with_suffix("")] = text
    files[Path("go.mod")] = f"module {args.module}\n\ngo {go_version}\n\nrequire github.com/yottaapp/yotta {args.sdk_version}\n"
    files[Path("plugin.json")] = json.dumps({
        "namespace": args.namespace.rstrip("/"), "slug": args.slug, "name": args.name,
        "description": "Replace with your plugin description.", "version": "1.0.0",
        "nodeVersion": "1.0.0", "panel": args.with_panel, "panelPort": args.panel_port,
    }, ensure_ascii=False, indent=2) + "\n"
    # Render and validate all inputs before creating anything; never merge into an existing project.
    output.mkdir(parents=True)
    for relative, text in files.items():
        target = output / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text, encoding="utf-8", newline="\n")
    print(f"Created {output}\nNext: configure GOPROXY for your pinned SDK, then go mod tidy and ./build.ps1 -Action Check")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", required=True)
    parser.add_argument("--module", required=True)
    parser.add_argument("--namespace", required=True)
    parser.add_argument("--slug", required=True)
    parser.add_argument("--name", required=True)
    parser.add_argument("--sdk-version", required=True)
    parser.add_argument("--with-panel", action="store_true")
    parser.add_argument("--panel-port", type=int, default=18760)
    try:
        create(parser.parse_args())
    except (ValueError, OSError) as error:
        parser.exit(1, f"{error}\n")
