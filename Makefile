.PHONY: build install test clean run help

build:
	@go build -o sstart ./cmd/sstart

install:
	@go install ./cmd/sstart

test:
	@go test ./...

clean:
	@rm -f sstart

run: build
	@./sstart $(ARGS)

help:
	@echo "Available targets:"
	@echo "  build    - Build the sstart binary"
	@echo "  install  - Install sstart to GOPATH/bin"
	@echo "  test     - Run tests"
	@echo "  clean    - Remove build artifacts"
	@echo "  run      - Build and run sstart with ARGS"
	@echo "            Example: make run ARGS='--help'"

