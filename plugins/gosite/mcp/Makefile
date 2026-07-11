PLUGIN_ID   := gosite/mcp
PLUGIN_NAME := gosite-mcp
MAIN_PKG    := .
MCP_PKG     := ./cmd/mcp

include ../../_templates/_shared/Makefile.inc

.PHONY: build

build: _build-bin _build-mcp _pack-zip
	@sha256sum $(OUT)/$(PLUGIN_NAME).zip | awk '{print "sha256:", $$1}'

_build-mcp:
	@mkdir -p $(OUT)/plugin
	cd $(CURDIR) && CGO_ENABLED=0 go build -o $(CURDIR)/$(OUT)/plugin/mcp $(MCP_PKG)
