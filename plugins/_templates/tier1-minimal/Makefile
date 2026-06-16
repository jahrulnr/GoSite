PLUGIN_ID   := template/minimal
PLUGIN_NAME := gosite-template-minimal
MAIN_PKG    := ./plugins/_templates/tier1-minimal

include ../_shared/Makefile.inc

.PHONY: build vet

build: _build-bin _pack-zip
	@sha256sum $(OUT)/$(PLUGIN_NAME).zip | awk '{print "sha256:", $$1}'

vet:
	cd $(GOSITE_ROOT) && go vet $(MAIN_PKG)
