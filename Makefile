# Release binaries are built by CI (.github/workflows/release.yml) on `v*` tags.
# `make bin` reproduces them locally in bin/ (gitignored).
PLATFORMS = darwin/arm64 darwin/amd64 linux/amd64 linux/arm64
VERSION ?= dev

bin:
	for t in $(PLATFORMS); do \
	  GOOS=$${t%/*} GOARCH=$${t#*/} CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -buildid= -X main.version=$(VERSION)" -o bin/delegate-$${t%/*}-$${t#*/} ./cmd/delegate || exit 1; done
	cd bin && sha256sum delegate-* > checksums.txt

test:
	go vet ./... && go test ./...

.PHONY: bin test
