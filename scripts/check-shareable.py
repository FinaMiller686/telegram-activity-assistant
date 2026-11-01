#!/usr/bin/env python3
"""Check shareable Git files without printing detected private values."""
from pathlib import Path
import argparse
import re
import subprocess
import sys


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--staged", action="store_true", help="inspect the entire Git index")
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    command = ["git", "ls-files", "-z", "--cached"]
    if not args.staged:
        command += ["--others", "--exclude-standard"]
    names = sorted(set(subprocess.check_output(command, cwd=root).decode().split("\0")) - {""})
    patterns = {
        "private-key": re.compile(r"-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----"),
        "api-key": re.compile(r"\bsk-(?:proj-|svcacct-)?[A-Za-z0-9_-]{24,}"),
        "github-token": re.compile(r"\b(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{40,})"),
        "bot-token": re.compile(r"\b[0-9]{7,12}:[A-Za-z0-9_-]{30,}"),
        "local-path": re.compile(r"/(?:Users|home)/[^/\s]+/|/var/folders(?:/)|\.codex(?:/)"),
    }
    blocked_parts = {".local", ".cache", ".next", "node_modules", "exports", "backups", "pgdata"}
    blocked_suffixes = {".key", ".pem", ".p12", ".pfx", ".db", ".sqlite", ".sqlite3", ".log", ".dump", ".backup", ".session", ".dmg", ".jsonl"}
    known_images = {"docs/images/home-zh.png", "docs/images/home-en.png", "web/app/icon.png", "web/app/apple-icon.png"}
    issues = []
    for name in names:
        path = root / name
        if path.is_symlink():
            issues.append((name, "symlink"))
            continue
        if (set(path.relative_to(root).parts) & blocked_parts
                or path.suffix.lower() in blocked_suffixes
                or path.name == ".env"
                or (path.name.startswith(".env.") and path.name != ".env.example")
                or ".sparsebundle" in name
                or "session-journal" in name):
            issues.append((name, "local-data-file"))
            continue
        if args.staged:
            data = subprocess.check_output(["git", "show", ":" + name], cwd=root)
        else:
            if not path.is_file():
                continue
            data = path.read_bytes()
        try:
            content = data.decode("utf-8")
        except UnicodeDecodeError:
            if name not in known_images:
                issues.append((name, "unreviewed-binary"))
            continue
        for label, pattern in patterns.items():
            if pattern.search(content):
                issues.append((name, label))
    for name, label in issues:
        print(f"BLOCKED {label}: {name}")
    print(f"Checked {len(names)} files; {len(issues)} potential issues. Detected values are never printed.")
    return bool(issues)


if __name__ == "__main__":
    sys.exit(main())
