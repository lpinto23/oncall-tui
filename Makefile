BINARY     := oncall-tui
INSTALL_DIR := /usr/local/bin
MODULE     := github.com/lpinto23/oncall-tui

GOOS   ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

.PHONY: all build install uninstall upgrade clean fmt vet

all: build

build:
	go build -o $(BINARY) .

install: build
	sudo cp $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed to $(INSTALL_DIR)/$(BINARY)"

upgrade:
	git pull && $(MAKE) install

uninstall:
	sudo rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "Removed $(INSTALL_DIR)/$(BINARY)"

clean:
	rm -f $(BINARY)

fmt:
	gofmt -w .

vet:
	go vet ./...
