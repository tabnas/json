#!/bin/sh
# Fetch the external JSON conformance corpus (nst/JSONTestSuite) into
# test/jsontestsuite/, where both runtimes' conformance tests find it.
#
# UPSTREAM (pinned):
#   https://github.com/nst/JSONTestSuite
#   commit 1ef36fa01286573e846ac449e8683f8833c5b26a
#
# The commit SHA — not a branch — is what is pinned, so "the conformance
# suite passes" always refers to one exact corpus. An unpinned clone of the
# default branch can silently change what is being measured between runs.
#
# The corpus is third-party (Nicolas Seriot, MIT) and is NOT vendored:
# test/jsontestsuite/ is .gitignore'd and must never be committed.
#
# Idempotent: if the pinned corpus is already present with the expected
# shape, this exits 0 without touching the network. Pass --force to
# re-clone.
#
#   sh test/fetch-jsontestsuite.sh            # default location
#   sh test/fetch-jsontestsuite.sh --force    # re-download
#   make json-test-suite                      # same thing
#
# It runs automatically before the conformance tests — the `pretest` npm
# script in ts/, and TestMain in go/ — so a plain `npm test` or
# `go test ./...`, in CI as well as locally, always grades against it.
# When the corpus is absent the conformance tests FAIL rather than skip.

set -e

URL="https://github.com/nst/JSONTestSuite.git"
SHA="1ef36fa01286573e846ac449e8683f8833c5b26a"

# Exact case counts at the pinned commit. Both test runners re-assert these
# before grading, so a narrowed or half-cloned corpus goes red instead of
# quietly inflating the pass rate.
EXPECT_Y=95    # y_ : MUST be accepted
EXPECT_N=188   # n_ : MUST be rejected
EXPECT_I=35    # i_ : implementation-defined
EXPECT_TOTAL=318

FORCE=0
DIR=""
for arg in "$@"; do
  case "$arg" in
    --force) FORCE=1 ;;
    *) DIR="$arg" ;;
  esac
done

: "${DIR:=$(cd "$(dirname "$0")" && pwd)/jsontestsuite}"

count() {
  find "$DIR/test_parsing" -maxdepth 1 -name "$1*.json" | wc -l | tr -d ' '
}

corpus_ok() {
  [ -d "$DIR/test_parsing" ] || return 1
  [ "$(git -C "$DIR" rev-parse HEAD 2>/dev/null || true)" = "$SHA" ] || return 1
  [ "$(count y_)" = "$EXPECT_Y" ] || return 1
  [ "$(count n_)" = "$EXPECT_N" ] || return 1
  [ "$(count i_)" = "$EXPECT_I" ] || return 1
  return 0
}

if [ "$FORCE" = "0" ] && corpus_ok; then
  echo "nst/JSONTestSuite @ $SHA already present: $DIR (use --force to re-clone)"
  exit 0
fi

# Absent, partial, or at the wrong commit: start clean rather than merge.
rm -rf "$DIR"

echo "Cloning nst/JSONTestSuite @ $SHA into $DIR ..."
mkdir -p "$(dirname "$DIR")"
# A blobless partial clone is far smaller than full history; fall back to a
# plain clone where the server or the local git does not support it.
if ! git clone --quiet --no-checkout --filter=blob:none "$URL" "$DIR" 2>/dev/null; then
  rm -rf "$DIR"
  git clone --quiet --no-checkout "$URL" "$DIR"
fi
git -C "$DIR" checkout --quiet "$SHA"

HEAD_SHA="$(git -C "$DIR" rev-parse HEAD)"
if [ "$HEAD_SHA" != "$SHA" ]; then
  echo "ERROR: $DIR is at $HEAD_SHA, not the pinned commit $SHA" >&2
  exit 1
fi

y="$(count y_)"
n="$(count n_)"
i="$(count i_)"
total=$((y + n + i))
if [ "$y" != "$EXPECT_Y" ] || [ "$n" != "$EXPECT_N" ] ||
   [ "$i" != "$EXPECT_I" ] || [ "$total" != "$EXPECT_TOTAL" ]; then
  echo "ERROR: unexpected corpus shape at $SHA" >&2
  echo "  got      y_=$y n_=$n i_=$i total=$total" >&2
  echo "  expected y_=$EXPECT_Y n_=$EXPECT_N i_=$EXPECT_I total=$EXPECT_TOTAL" >&2
  exit 1
fi

echo "Done: $y must-accept, $n must-reject, $i implementation-defined = $total cases."
