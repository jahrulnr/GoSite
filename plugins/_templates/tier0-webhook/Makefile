PLUGIN_NAME := gosite-template-webhook
OUT         ?= dist

.PHONY: build sign install clean receiver

build:
	@mkdir -p $(OUT)
	cp manifest.json $(OUT)/manifest.json
	cd $(OUT) && zip -qr $(PLUGIN_NAME).zip manifest.json
	@echo "==> $(OUT)/$(PLUGIN_NAME).zip (manifest only)"

sign:
	@test -n "$(KEY)" || (echo "KEY=path/to/signing.key required"; exit 1)
	@test -f $(OUT)/$(PLUGIN_NAME).zip || $(MAKE) build
	go run ../_shared/scripts/sign.go \
		-artifact $(OUT)/$(PLUGIN_NAME).zip \
		-manifest manifest.json \
		-key $(KEY) \
		-keyid $(or $(KEY_ID),template-1) \
		-vendor template
	@sha256sum $(OUT)/$(PLUGIN_NAME).zip | awk '{print "upload sha256:", $$1}'

install:
	@../_shared/scripts/dev-install.sh $(OUT)/$(PLUGIN_NAME).zip

receiver:
	go run ./dev-receiver

clean:
	rm -rf $(OUT)
