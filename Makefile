# Build, test and publish both the TypeScript (ts/) and Go (go/)
# implementations. ts/ is canonical; go/ tracks it.
#
# Local build/test resolve the unpublished @tabnas siblings via the
# repo-set go.work + node_modules symlinks (admin/scripts/link.sh).

.PHONY: all build test clean build-ts build-go test-ts test-go \
        clean-ts clean-go publish-ts publish-go tags-go reset \
        json-test-suite

all: build test

build: build-ts build-go

test: test-ts test-go

clean: clean-ts clean-go

# --- External conformance suite ---
# Fetch nst/JSONTestSuite at its pinned commit into test/jsontestsuite
# (gitignored). Both runtimes fetch it themselves before grading — `pretest`
# in ts/, TestMain in go/ — so this target is only for fetching it by hand;
# it is not a prerequisite of `make test`. When the corpus is missing the
# conformance tests fail, they never skip.
json-test-suite:
	sh test/fetch-jsontestsuite.sh

# --- TypeScript (package in ts/) ---
build-ts:
	cd ts && npm run build

test-ts:
	cd ts && npm test

clean-ts:
	rm -rf ts/dist ts/dist-test

# Publish the TypeScript package at its current package.json version.
publish-ts: test-ts
	cd ts && npm publish --access public

# --- Go (module in go/) ---
build-go:
	cd go && go build ./...

test-go:
	cd go && go test -v ./...

clean-go:
	cd go && go clean

# Publish the Go module: make publish-go V=x.y.z
# Injects V into the Go `VERSION` const, commits, tags go/vX.Y.Z, and
# (when gh is available) creates a GitHub release.
#
# NOTE: this rewrites go/json.go ONLY. It does NOT touch ts/src/json.ts or
# ts/package.json — keeping the two runtimes in sync is the release
# orchestrator's job (admin/publish.sh), and the version tests in both
# runtimes fail the build if they ever drift.
publish-go: test-go
	@test -n "$(V)" || (echo "Usage: make publish-go V=x.y.z" && exit 1)
	sed -i.bak 's/^const VERSION = ".*"/const VERSION = "$(V)"/' go/json.go
	rm -f go/json.go.bak
	git add go/json.go
	git commit -m "go: v$(V)"
	git tag go/v$(V)
	git push origin main go/v$(V)
	@command -v gh >/dev/null 2>&1 && gh release create go/v$(V) --title "go/v$(V)" --notes "Go module release v$(V)" || true

# List published Go module tags, newest first.
tags-go:
	git tag -l 'go/v*' --sort=-version:refname

reset:
	cd ts && npm run reset
	cd go && go clean -cache && go build ./... && go test -v ./...
