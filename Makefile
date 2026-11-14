.PHONY: test vet check help

help:
	@echo "Available commands"
	@echo "  make build - build project"
	@echo "  make test  - run all unit tests"
	@echo "  make vet   - run Go static checks"
	@echo "  make check - run the complete local verification suite"

build:
	go build ./...

test:
	go test ./...

vet:
	go vet ./...

check: test vet
