#!/usr/bin/env bash
# Fetch the authoritative third-party JSON conformance corpus:
# nst/JSONTestSuite -- the corpus behind "Parsing JSON is a Minefield"
# (http://seriot.ch/projects/parsing_json.html), the most-cited RFC 8259
# / ECMA-404 test set.
#
# UPSTREAM (pinned):
#   https://github.com/nst/JSONTestSuite
#   commit 1ef36fa01286573e846ac449e8683f8833c5b26a
#
# The commit SHA -- not a branch -- is what is pinned, so the conformance
# numbers recorded in this repository always refer to one exact corpus.
#
# The corpus is owned by Nicolas Seriot and is NOT redistributed as part
# of this repository: test/jsontestsuite/ is gitignored and must never be
# committed. Running this script is an explicit opt-in to download it.
#
# The script is idempotent: if the pinned corpus is already present it
# exits 0 without touching the network. Pass --force to re-clone.
#
# Usage:
#   scripts/fetch-jsontestsuite.sh            # default location
#   scripts/fetch-jsontestsuite.sh --force    # re-download
#   scripts/fetch-jsontestsuite.sh /some/dir  # custom destination
#
# The conformance tests run against it automatically:
#   ts/  -> the `pretest` npm script runs this, then test/jsontestsuite.test.js
#   go/  -> go/jsontestsuite_test.go
# If the corpus is missing the tests FAIL LOUDLY. They never skip: a
# conformance suite that quietly does not run is worse than no suite at
# all, because the green tick is a lie.
set -euo pipefail

URL="https://github.com/nst/JSONTestSuite"
SHA="1ef36fa01286573e846ac449e8683f8833c5b26a"

# Exact case counts at the pinned commit. The test runners re-assert these
# before grading, so a narrowed corpus goes red instead of quietly
# inflating the pass rate.
EXPECT_Y=95    # y_ : MUST be accepted
EXPECT_N=188   # n_ : MUST be rejected
EXPECT_I=35    # i_ : implementation-defined
EXPECT_TOTAL=318

FORCE=0
DEST=""
for arg in "$@"; do
  case "$arg" in
    --force) FORCE=1 ;;
    *) DEST="$arg" ;;
  esac
done

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"
DEST="${DEST:-$REPO_ROOT/test/jsontestsuite}"

count() { find "$DEST/test_parsing" -maxdepth 1 -name "$1*.json" | wc -l | tr -d ' '; }

corpus_ok() {
  [ -d "$DEST/test_parsing" ] || return 1
  [ "$(git -C "$DEST" rev-parse HEAD 2>/dev/null || true)" = "$SHA" ] || return 1
  [ "$(count y_)" = "$EXPECT_Y" ] || return 1
  [ "$(count n_)" = "$EXPECT_N" ] || return 1
  [ "$(count i_)" = "$EXPECT_I" ] || return 1
  return 0
}

if [ "$FORCE" = "1" ]; then
  rm -rf "$DEST"
elif corpus_ok; then
  echo "nst/JSONTestSuite @ $SHA already present at $DEST (use --force to re-clone)."
  exit 0
else
  # Partial/stale/wrong-commit checkout: start clean rather than merge.
  rm -rf "$DEST"
fi

echo "Cloning $URL @ $SHA ..."
mkdir -p "$(dirname "$DEST")"
git clone --quiet --no-checkout --filter=blob:none "$URL" "$DEST"
git -C "$DEST" checkout --quiet "$SHA"

if [ "$(git -C "$DEST" rev-parse HEAD)" != "$SHA" ]; then
  echo "ERROR: $DEST is not at the pinned commit $SHA" >&2
  exit 1
fi

y="$(count y_)"; n="$(count n_)"; i="$(count i_)"
total=$((y + n + i))
if [ "$y" != "$EXPECT_Y" ] || [ "$n" != "$EXPECT_N" ] || [ "$i" != "$EXPECT_I" ] ||
   [ "$total" != "$EXPECT_TOTAL" ]; then
  echo "ERROR: unexpected corpus shape at $SHA" >&2
  echo "  got      y_=$y n_=$n i_=$i total=$total" >&2
  echo "  expected y_=$EXPECT_Y n_=$EXPECT_N i_=$EXPECT_I total=$EXPECT_TOTAL" >&2
  exit 1
fi

echo "Done. $DEST/test_parsing: $y y_ (must accept), $n n_ (must reject), $i i_ (implementation-defined) = $total."
