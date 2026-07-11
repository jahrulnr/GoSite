# Shared Make targets for plugin templates.
#
# Include from a template Makefile:
#   include ../_shared/Makefile.inc
#
# Required variables in the parent Makefile:
#   PLUGIN_ID   — e.g. acme/hello-minimal
#   PLUGIN_NAME — zip base name, e.g. gosite-hello-minimal
#   MAIN_PKG    — go package path relative to GOSITE_ROOT

GOSITE_ROOT ?= $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/../../..)
GO_MOD_DIR  ?= $(CURDIR)
OUT         ?= dist
KEY         ?=
KEY_ID      ?=
VENDOR      ?= $(word 1,$(subst /, ,$(PLUGIN_ID)))
GOSITE_URL  ?= http://127.0.0.1:8080

.PHONY: _build-bin _pack-zip _sha

_build-bin:
	@mkdir -p $(OUT)/plugin
	cd $(GO_MOD_DIR) && CGO_ENABLED=0 go build -o $(CURDIR)/$(OUT)/plugin/gosite $(MAIN_PKG)
	cp $(OUT)/plugin/gosite $(OUT)/plugin/validate

_pack-zip: manifest.json
	@mkdir -p $(OUT)
	cp manifest.json $(OUT)/manifest.json
	cd $(OUT) && zip -qr $(PLUGIN_NAME).zip manifest.json plugin/
	@echo "==> $(OUT)/$(PLUGIN_NAME).zip"

_sha:
	@sha256sum $(OUT)/$(PLUGIN_NAME).zip | awk '{print $$1}'

sign:
	@test -n "$(KEY)" || (echo "KEY=path/to/signing.key required"; exit 1)
	@test -f $(OUT)/$(PLUGIN_NAME).zip || $(MAKE) build
	cd $(GOSITE_ROOT) && go run $(GOSITE_ROOT)/plugins/_templates/_shared/scripts/sign.go \
		-artifact $(CURDIR)/$(OUT)/$(PLUGIN_NAME).zip \
		-manifest $(CURDIR)/manifest.json \
		-key $(KEY) \
		-keyid $(or $(KEY_ID),$(VENDOR)-1) \
		-vendor $(VENDOR)
	@sha256sum $(OUT)/$(PLUGIN_NAME).zip | awk '{print "upload sha256:", $$1}'

install:
	@test -f $(OUT)/$(PLUGIN_NAME).zip || $(MAKE) build
	@SHA=$$(sha256sum $(OUT)/$(PLUGIN_NAME).zip | awk '{print $$1}'); \
	curl -sf -X POST "$(GOSITE_URL)/api/v1/plugins/install" \
		-F "artifact=@$(OUT)/$(PLUGIN_NAME).zip" \
		-F "sha256=$$SHA" \
		-u "$${AUTH_USER:-admin}:$${AUTH_PASS:-123456}"

vet:
	cd $(GO_MOD_DIR) && go vet ./...

clean:
	rm -rf $(OUT)
