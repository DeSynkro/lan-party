#!/usr/bin/env bash
# Validate and create a LAN Party release tag.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

version=""
push=false
check_only=false
remote="${LANPARTY_RELEASE_REMOTE:-}"

usage() {
    cat >&2 <<'EOF'
Usage: ./build/tag-release.sh [--check-only] [--push] VERSION

VERSION must be a stable version such as 1.3.0. The script requires:
  - a clean main branch
  - docs/releases/vVERSION.md
  - make test and make vet to pass
  - the tag must not already exist

Without --push it creates a local annotated tag and prints the push commands.
With --push it pushes main and the tag to the github remote (or origin).
Use --check-only to run every check without creating a tag.

Override the remote with LANPARTY_RELEASE_REMOTE.
EOF
}

while [ $# -gt 0 ]; do
    case "$1" in
        --push) push=true ;;
        --check-only | --check) check_only=true ;;
        -h | --help) usage; exit 0 ;;
        -*)
            echo "Unknown option: $1" >&2
            usage
            exit 2
            ;;
        *)
            [ -z "$version" ] || { echo "Only one version is allowed." >&2; exit 2; }
            version="$1"
            ;;
    esac
    shift
done

if ! [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "Use a stable version like 1.2.0 (no leading v)." >&2
    exit 2
fi

tag="v${version}"
notes="docs/releases/${tag}.md"

if [ "$(git branch --show-current)" != "main" ]; then
    echo "Release tags are created from main." >&2
    exit 1
fi
if [ -n "$(git status --porcelain)" ]; then
    echo "The working tree is not clean. Commit or stash first." >&2
    git status --short >&2
    exit 1
fi
if [ ! -f "$notes" ]; then
    echo "Missing release notes: $notes" >&2
    exit 1
fi
if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null; then
    echo "Tag ${tag} already exists. Never move or reuse a release tag." >&2
    exit 1
fi

echo "Running release checks..."
make test
make vet

if [ "$check_only" = true ]; then
    echo "Release checks passed for ${tag}. No tag was created."
    exit 0
fi

git tag -a "$tag" -m "LAN Party ${version}"
echo "Created annotated tag ${tag}."

if [ -z "$remote" ]; then
    if git remote get-url github >/dev/null 2>&1; then
        remote="github"
    else
        remote="origin"
    fi
fi

if [ "$push" = true ]; then
    git push "$remote" main
    git push "$remote" "$tag"
    echo
    echo "Pushed main and ${tag} to ${remote}."
    echo "CI will build the release assets and create a draft release."
    echo "Open the draft on GitHub, check the notes and assets, then publish it."
else
    echo
    echo "Push the release when ready:"
    echo "  git push ${remote} main"
    echo "  git push ${remote} ${tag}"
    echo "CI will build the release assets and create a draft release."
fi
