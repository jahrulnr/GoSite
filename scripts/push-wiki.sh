#!/usr/bin/env bash
# Push docs/wiki-export/ to GitHub Wiki for GoSite.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
EXPORT="$ROOT/docs/wiki-export"
REPO="${GOSITE_REPO:-jahrulnr/GoSite}"

if [[ ! -d "$EXPORT" ]] || [[ -z "$(ls -A "$EXPORT"/*.md 2>/dev/null)" ]]; then
  echo "ERROR: wiki-export is empty. Run: make wiki-export" >&2
  exit 1
fi

TOKEN="${WIKI_TOKEN:-${GITHUB_TOKEN:-}}"
if [[ -z "$TOKEN" ]]; then
  echo "ERROR: set WIKI_TOKEN (recommended) or GITHUB_TOKEN" >&2
  exit 1
fi

WIKI_PUSH_URL="https://x-access-token:${TOKEN}@github.com/${REPO}.wiki.git"
WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

echo "Cloning wiki repository for ${REPO}..."
if ! git clone --depth=1 "$WIKI_PUSH_URL" "$WORKDIR" 2>/dev/null; then
  echo "Wiki git repo not found. Initializing (enable Wiki in repo Settings if this fails on push)."
  git -C "$WORKDIR" init -q
  git -C "$WORKDIR" remote add origin "$WIKI_PUSH_URL"
  git -C "$WORKDIR" checkout -b master
else
  echo "Wiki clone OK."
fi

find "$WORKDIR" -maxdepth 1 -name '*.md' -delete
cp -a "$EXPORT"/*.md "$WORKDIR"/

git -C "$WORKDIR" add -A
if git -C "$WORKDIR" diff --staged --quiet; then
  echo "Wiki unchanged — nothing to push."
  exit 0
fi

git -C "$WORKDIR" -c user.name="gosite wiki bot" -c user.email="gosite-wiki-bot@users.noreply.github.com" \
  commit -m "Sync wiki from ${REPO}@${GITHUB_SHA:-local}"

BRANCH="$(git -C "$WORKDIR" branch --show-current)"
if [[ -z "$BRANCH" ]]; then BRANCH=master; fi

echo "Pushing to wiki (${BRANCH})..."
if ! git -C "$WORKDIR" push -u origin "$BRANCH"; then
  echo "" >&2
  echo "ERROR: wiki push failed." >&2
  echo "  1. Enable Wiki: repo Settings → Features → Wikis" >&2
  echo "  2. Create one manual wiki page (e.g. Home) so .wiki git exists" >&2
  echo "  3. Add repo secret WIKI_TOKEN (PAT with repo scope) if GITHUB_TOKEN is rejected" >&2
  exit 1
fi

echo "Pushed $(find "$EXPORT" -maxdepth 1 -name '*.md' | wc -l) pages to ${REPO}.wiki"
