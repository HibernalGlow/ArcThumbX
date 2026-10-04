#!/usr/bin/env bash
# Rebuild the panel's embedded font subsets in assets/fonts/.
#
#   tools/fonts/build.sh              # fetch what's missing, then subset
#   tools/fonts/build.sh --refresh    # re-download the upstream families
#
# Everything except this launcher lives in tools/fonts/subset.py: the sources
# come from google/fonts over HTTPS, the subsetting and renaming run under
# fontTools, and woff2 compression needs brotli -- so both are injected by `uv
# run` rather than installed globally.
#
# The derived woff2 files are committed, so a normal `cargo build` touches
# neither network nor Python.
set -euo pipefail

REPO="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# The proxy is only used when the caller has not set one. It buys real speed on
# raw.githubusercontent from this machine.
export HTTPS_PROXY="${HTTPS_PROXY:-${https_proxy:-http://127.0.0.1:7890}}"

exec uv run --quiet --with fonttools --with brotli python3 "$REPO/tools/fonts/subset.py" "$@"
