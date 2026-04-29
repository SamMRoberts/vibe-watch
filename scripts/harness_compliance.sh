#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

export GOCACHE="${GOCACHE:-$repo_root/.vibe-watch/go-build-cache}"
mkdir -p "$GOCACHE"

fail() {
  printf 'harness compliance failed: %s\n' "$1" >&2
  printf 'make another pass until the check succeeds.\n' >&2
  exit 1
}

printf '==> checking docs current-scope markers\n'
for doc in docs/*.md; do
  rg -q '^## Current Scope$' "$doc" || fail "$doc must include a Current Scope section"
done

printf '==> checking JSONL fixture boundaries\n'
unexpected_jsonl="$(
  find . \
    -path './.git' -prune -o \
    -path './.harness-validation' -prune -o \
    -path './.vibe-watch' -prune -o \
    -name '*.jsonl' ! -path './testdata/codex/*' -print
)"
if [[ -n "$unexpected_jsonl" ]]; then
  printf '%s\n' "$unexpected_jsonl" >&2
  fail 'JSONL files may only be committed under testdata/codex'
fi

printf '==> checking obvious private/session tokens in fixtures and docs\n'
if rg -n '(/Users/[^ ]+/.codex/sessions/|sk-[A-Za-z0-9]{20,}|BEGIN PRIVATE KEY)' docs testdata; then
  fail 'docs and fixtures must not contain raw local session paths, API keys, or private keys'
fi

printf '==> running go tests\n'
go test ./...

printf 'harness compliance passed\n'
