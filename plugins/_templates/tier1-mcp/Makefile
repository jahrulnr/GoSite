PLUGIN_ID   := template/mcp
PLUGIN_NAME := gosite-template-mcp
MAIN_PKG    := .

include ../_shared/Makefile.inc

.PHONY: build

build: _build-bin _pack-zip
	@sha256sum $(OUT)/$(PLUGIN_NAME).zip | awk '{print "sha256:", $$1}'
