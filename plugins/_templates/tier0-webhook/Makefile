PLUGIN_NAME := gosite-template-webhook
OUT         ?= dist
GOSITE_ROOT ?= $(abspath $(dir $(lastword $(MAKEFILE_LIST)))/../../..)

.PHONY: build sign install clean receiver

build:
	@mkdir -p $(OUT)
	cp manifest.json $(OUT)/manifest.json
	cd $(OUT) && zip -qr $(PLUGIN_NAME).zip manifest.json
	@echo "==> $(OUT)/$(PLUGIN_NAME).zip (manifest only)"

sign:
	@test -n "$(KEY)" || (echo "KEY=path/to/signing.key required"; exit 1)
	@test -f $(OUT)/$(PLUGIN_NAME).zip || $(MAKE) build
	cd $(GOSITE_ROOT) && go run $(GOSITE_ROOT)/plugins/_templates/_shared/scripts/sign.go \
		-artifact $(CURDIR)/$(OUT)/$(PLUGIN_NAME).zip \
		-manifest $(CURDIR)/manifest.json \
		-key $(KEY) \
		-keyid $(or $(KEY_ID),template-1) \
		-vendor template
	@sha256sum $(OUT)/$(PLUGIN_NAME).zip | awk '{print "upload sha256:", $$1}'

install:
	@../_shared/scripts/dev-install.sh $(OUT)/$(PLUGIN_NAME).zip

receiver:
	cd $(CURDIR) && go run ./dev-receiver

vet:
	cd $(CURDIR) && go vet ./...

clean:
	rm -rf $(OUT)
