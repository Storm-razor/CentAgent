.PHONY: help all fmt test vet tidy build run clean lint

GO ?= go
APP ?= centagent
MAIN ?= ./cmd/centagent
BIN_DIR ?= bin

GOOS := $(shell $(GO) env GOOS)
BIN_EXT :=
ifeq ($(GOOS),windows)
BIN_EXT := .exe
endif
BIN := $(BIN_DIR)/$(APP)$(BIN_EXT)

MKDIR_BIN :=
RM_BIN_DIR :=
GOFMT_ALL :=
ifeq ($(GOOS),windows)
MKDIR_BIN = powershell -NoProfile -Command "New-Item -ItemType Directory -Force -Path '$(BIN_DIR)' | Out-Null"
RM_BIN_DIR = powershell -NoProfile -Command "if (Test-Path -LiteralPath '$(BIN_DIR)') { Remove-Item -LiteralPath '$(BIN_DIR)' -Recurse -Force }"
GOFMT_ALL = powershell -NoProfile -Command "$$files = Get-ChildItem -Path . -Recurse -Filter *.go | Where-Object { $$_.FullName -notmatch '\\\\vendor\\\\' } | ForEach-Object { $$_.FullName }; if ($$files.Count -gt 0) { gofmt -w $$files }"
else
MKDIR_BIN = mkdir -p $(BIN_DIR)
RM_BIN_DIR = rm -rf $(BIN_DIR)
GOFMT_ALL = gofmt -w $$(find . -name '*.go' -not -path './vendor/*')
endif

help:
	@echo "Targets:"
	@echo "  make fmt    - gofmt all Go files"
	@echo "  make test   - go test ./..."
	@echo "  make vet    - go vet ./..."
	@echo "  make tidy   - go mod tidy"
	@echo "  make build  - build binary to $(BIN)"
	@echo "  make run    - go run $(MAIN)"
	@echo "  make lint   - golangci-lint run ./..."
	@echo "  make clean  - remove $(BIN_DIR)"

all: fmt test build

fmt:
	@$(GO) fmt ./...
	@$(GOFMT_ALL)

test:
	@$(GO) test ./...

vet:
	@$(GO) vet ./...

tidy:
	@$(GO) mod tidy

build:
	@$(MKDIR_BIN)
	@$(GO) build -o $(BIN) $(MAIN)
	@echo "built: $(BIN)"

run:
	@$(GO) run $(MAIN)


lint:
	@golangci-lint run ./...

clean:
	@$(RM_BIN_DIR)
