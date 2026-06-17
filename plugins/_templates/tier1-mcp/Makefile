PLUGIN_ID   := template/mcp
PLUGIN_NAME := gosite-template-mcp
MAIN_PKG    := ./plugins/_templates/tier1-mcp
MCP_PKG     := ./plugins/gosite/mcp/cmd/mcp

include ../_shared/Makefile.inc

.PHONY: build vet

build: _build-bin _build-mcp _pack-zip
	@sha256sum $(OUT)/$(PLUGIN_NAME).zip | awk '{print "sha256:", $$1}'

_build-mcp:
	@mkdir -p $(OUT)/plugin
	cd $(GOSITE_ROOT) && CGO_ENABLED=0 go build -o $(CURDIR)/$(OUT)/plugin/mcp $(MCP_PKG)

vet:
	cd $(GOSITE_ROOT) && go vet $(MAIN_PKG)
