.PHONY: build install uninstall test clean fmt vet

BINARY  := diary
BIN_DIR := bin
INSTALL_DIR := /usr/local/bin

GOFLAGS := -trimpath

build:
	mkdir -p $(BIN_DIR)
	go build $(GOFLAGS) -o $(BIN_DIR)/$(BINARY) ./cmd/diary

install: build
	@echo "Installing to $(INSTALL_DIR)/$(BINARY) (may ask for sudo)"
	cp $(BIN_DIR)/$(BINARY) $(INSTALL_DIR)/$(BINARY)
	@which $(BINARY) >/dev/null 2>&1 && echo "Done. Try: diary" || echo "Done. Make sure $(INSTALL_DIR) is in your PATH."

uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "Removed $(INSTALL_DIR)/$(BINARY)"

test:
	go test -v ./...

test-short:
	go test -short ./...

fmt:
	gofmt -s -w .

vet:
	go vet ./...

clean:
	rm -rf $(BIN_DIR) $(DIST_DIR)

run: build
	./$(BIN_DIR)/$(BINARY)
