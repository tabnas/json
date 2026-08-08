#!/bin/sh
# Fetch the external JSON conformance suite (nst/JSONTestSuite) into
# test/jsontestsuite/, where both runtimes' conformance tests find it.
#
# The suite is NOT vendored (it is .gitignore'd) because it is a separate
# MIT-licensed repository of ~318 parsing cases; run this once to enable
# ts/test/conformance.test.js and go/conformance_test.go. Without it both
# tests skip, and the RFC 8259 claim in AGENTS.md is unverified.
#
#   sh test/fetch-jsontestsuite.sh      # or: make json-test-suite

set -e

DIR="$(cd "$(dirname "$0")" && pwd)/jsontestsuite"

if [ -d "$DIR/test_parsing" ]; then
  echo "JSONTestSuite already present: $DIR"
  exit 0
fi

echo "Cloning nst/JSONTestSuite into $DIR ..."
git clone --depth 1 https://github.com/nst/JSONTestSuite.git "$DIR"
echo "Done: $(ls "$DIR/test_parsing" | wc -l) parsing cases."
