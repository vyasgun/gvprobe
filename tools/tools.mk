# Use abspath so the path is valid before tools/bin exists (realpath can be empty then).
TOOLS_BINDIR := $(abspath $(TOOLS_DIR)/bin)

$(TOOLS_BINDIR)/golangci-lint: $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && GOBIN="$(TOOLS_BINDIR)" go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.11.4

$(TOOLS_BINDIR)/goimports: $(TOOLS_DIR)/go.mod
	cd $(TOOLS_DIR) && GOBIN="$(TOOLS_BINDIR)" go install golang.org/x/tools/cmd/goimports@latest
