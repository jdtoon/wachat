#!/usr/bin/env bash
# Idempotent installer for the local pre-commit hook.
# Copies (rather than symlinks) because Windows hates symlinks.

set -euo pipefail

repo_root=$(git rev-parse --show-toplevel)
src="$repo_root/scripts/pre-commit"
dst="$repo_root/.git/hooks/pre-commit"

if [ ! -f "$src" ]; then
    echo "install-hooks: $src not found" >&2
    exit 1
fi

mkdir -p "$repo_root/.git/hooks"
cp "$src" "$dst"
chmod +x "$dst" 2>/dev/null || true   # chmod is a no-op on NTFS but harmless

echo "Installed pre-commit hook -> $dst"
