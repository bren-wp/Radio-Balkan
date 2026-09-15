#!/usr/bin/env python3
from __future__ import annotations

import subprocess
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def main() -> int:
    result = subprocess.run(
        ["git", "status", "--porcelain", "--untracked-files=all"],
        cwd=ROOT,
        check=False,
        capture_output=True,
        text=True,
    )
    if result.returncode != 0:
        sys.stderr.write(result.stderr or "git status failed\n")
        return result.returncode or 1

    changes = [line for line in result.stdout.splitlines() if line.strip()]
    if changes:
        print("Repository workspace is not clean after build:", file=sys.stderr)
        for line in changes:
            print(f"  {line}", file=sys.stderr)
        return 1

    print("Repository workspace clean after build")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
