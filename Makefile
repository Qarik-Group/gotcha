DESTDIR      ?= /usr/local
RELEASE_ROOT ?= release
TARGETS      ?= linux/amd64 darwin/amd64

GITHUB_USER := jhunt
GITHUB_REPO := gotcha
GO_LDFLAGS := -ldflags="-X main.Version=$(VERSION)"

build:
	godep restore
	go build $(GO_LDFLAGS) .
	./gotcha -v

test:
	ginkgo *

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html coverage.out
	rm coverage.out

release: build
	@test -n "$(VERSION)" || echo "No VERSION specified! (use \`make VERSION=x.y.z release\`)"
	@test -n "$(VERSION)" || exit 1
	mkdir -p $(RELEASE_ROOT)
	@go get github.com/mitchellh/gox
	gox -osarch="$(TARGETS)" --output="$(RELEASE_ROOT)/artifacts/gotcha-{{.OS}}-{{.Arch}}" $(GO_LDFLAGS) .

final: release
	@go get github.com/aktau/github-release
	@test -n "$(GITHUB_TOKEN)" || echo "No GITHUB_TOKEN specified..."
	@test -n "$(GITHUB_TOKEN)" || exit 1
	github-release release -u $(GITHUB_USER) -r $(GITHUB_REPO) -n "Gotcha v$(VERSION)" -t "v$(VERSION)" -d "$$(cat ci/release_notes.md || "no release notes...")"
	@cd release/artifacts && \
		for f in *; do \
			github-release upload -u $(GITHUB_USER) -r $(GITHUB_REPO) -t "v$(VERSION)" -n $$f -f $$f; \
		done && \
		cd ../..

install: build
	mkdir -p $(DESTDIR)/bin
	cp gotcha $(DESTDIR)/bin
