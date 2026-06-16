SHELL := /bin/bash

OUT ?= dist
NAME := gosite-hello-hook

.PHONY: build validate clean

build:
	@mkdir -p $(OUT)
	@echo "Building $(NAME) (host)..."
	@cd ../../../.. && CGO_ENABLED=0 go build -o $(OUT)/plugin/gosite ./examples/plugins/hello-hook
	@cp manifest/manifest.json $(OUT)/manifest.json
	@cd $(OUT) && zip -r $(NAME).zip plugin/gosite manifest.json
	@echo "Artifact: $(OUT)/$(NAME).zip"

validate:
	@cd ../../../.. && go vet ./examples/plugins/hello-hook/...

clean:
	@rm -rf $(OUT)
