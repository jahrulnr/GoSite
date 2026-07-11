BINARY := bin/gosite
PKG := ./...
# BASE_VERSION from latest v* tag; local builds default to X.Y.Z-dev, release builds to X.Y.Z.
BASE_VERSION ?= $(shell git describe --tags --match 'v*' --abbrev=0 2>/dev/null | sed 's/^v//' || echo 1.0.0)
ifndef VERSION
ifeq ($(RELEASE),1)
VERSION := $(BASE_VERSION)
else
VERSION := $(BASE_VERSION)-dev
endif
endif
LDFLAGS := -X github.com/jahrulnr/gosite/internal/buildinfo.Version=$(VERSION)

.PHONY: build test test-cover clean up down dev dev-api dev-fe build-fe build-docker docker-push docker-buildx dev-api-setup contract-check wiki-export bundled-plugins

IMAGE ?= ghcr.io/jahrulnr/gosite
PLATFORMS ?= linux/amd64,linux/arm64

bundled-plugins:
	mkdir -p dist/bundled-plugins
	$(MAKE) -C plugins/gosite/mcp build vet
	cp plugins/gosite/mcp/dist/gosite-mcp.zip dist/bundled-plugins/
	cp internal/service/plugin/bundled/index.json dist/bundled-plugins/bundled-plugins.json

wiki-export:
	@bash scripts/export-wiki.sh

build-fe:
	cd web && npm ci && npm run build

build: build-fe
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/gosite

DEV_STORAGE := /tmp/gosite-qa/storage

dev-api-setup:
	@bash scripts/dev-api-setup.sh

dev-api: dev-api-setup
	APP_ENV=local DEMO_SEED=true STORAGE_PATH=$(DEV_STORAGE) DB_DATABASE=$(DEV_STORAGE)/db.sqlite \
	WEB_PATH=$(DEV_STORAGE)/www \
	TEMPLATES_DIR=$(CURDIR)/config ETC_DIR=/tmp/gosite-qa/etc \
	LETSENCRYPT_DIR=/tmp/gosite-qa/etc/letsencrypt \
	PLUGIN_BUNDLED_PATH=$(CURDIR)/dist/bundled-plugins \
	AUTH_ENABLE=false SESSION_COOKIE_SECURE=false FE_EMBED=false \
	TLS_CERT=$(DEV_STORAGE)/webconfig/ssl/live/default/cert.pem \
	TLS_KEY=$(DEV_STORAGE)/webconfig/ssl/live/default/key.pem \
	LISTEN_ADDR=:8080 go run ./cmd/gosite serve

dev-fe:
	cd web && npm run dev

dev:
	@echo "Run 'make dev-api' and 'make dev-fe' in separate terminals"

test:
	go test -race -count=1 $(PKG)

contract-check:
	go test -count=1 ./internal/delivery/http/contract/...

COVERAGE_MIN ?= 65

test-cover:
	go test -race -coverprofile=coverage.out ./internal/service/... ./internal/observability/...
	@go test -cover ./internal/service/... ./internal/observability/... 2>&1 | awk -F'coverage: ' '/coverage:/ {gsub(/%.*/,"",$$2); if ($$2+0 < 80) print "WARN coverage <80%:", $$0}' || true
	@pct=$$(go tool cover -func=coverage.out | awk '/^total:/ {sub(/%/,"",$$3); print $$3}'); \
		echo "total coverage: $$pct% (required >= $(COVERAGE_MIN)%)"; \
		awk -v p="$$pct" -v min=$(COVERAGE_MIN) 'BEGIN{ if (p+0 < min+0) exit 1 }' || { echo "FAIL total coverage $$pct% < $(COVERAGE_MIN)%"; exit 1; }
	go tool cover -func=coverage.out | tail -1

up:
	$(MAKE) build-docker
	docker compose up -d

build-docker:
	docker build --network=host -t gosite:local .

docker-push:
	docker buildx build --platform $(PLATFORMS) \
		--build-arg VERSION=$(VERSION) \
		-t $(IMAGE):$(VERSION) \
		-t $(IMAGE):latest \
		--push .

down:
	docker compose down

clean:
	rm -f $(BINARY) *.out
