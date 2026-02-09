LSP_BUNDLE_DIR := internal/lsp/bundle
LANGUAGE_SERVER_DIR := .language-server
LSP_BUILT_SENTINEL := $(LSP_BUNDLE_DIR)/.built

.PHONY: build clean

build: $(LSP_BUILT_SENTINEL)
	go build ./cmd/rwx

$(LSP_BUILT_SENTINEL):
	@if [ ! -d "$(LANGUAGE_SERVER_DIR)" ]; then \
		echo "Cloning language-server..."; \
		git clone https://github.com/rwx-cloud/language-server.git $(LANGUAGE_SERVER_DIR); \
	fi
	@if [ ! -d "$(LANGUAGE_SERVER_DIR)/node_modules" ]; then \
		echo "Installing language-server dependencies..."; \
		cd $(LANGUAGE_SERVER_DIR) && npm ci; \
	fi
	@echo "Compiling language-server..."
	@cd $(LANGUAGE_SERVER_DIR) && npm run compile
	@rm -rf $(LSP_BUNDLE_DIR)/out $(LSP_BUNDLE_DIR)/support $(LSP_BUNDLE_DIR)/node_modules
	@cp -r $(LANGUAGE_SERVER_DIR)/out $(LSP_BUNDLE_DIR)/out
	@cp -r $(LANGUAGE_SERVER_DIR)/support $(LSP_BUNDLE_DIR)/support
	@cp -r $(LANGUAGE_SERVER_DIR)/node_modules $(LSP_BUNDLE_DIR)/node_modules
	@touch $(LSP_BUILT_SENTINEL)

clean:
	rm -rf $(LANGUAGE_SERVER_DIR)
	cd $(LSP_BUNDLE_DIR) && rm -rf out support node_modules .built
