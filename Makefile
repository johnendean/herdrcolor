REPO := $(shell pwd)
EXE  := $(if $(filter Windows_NT,$(OS)),.exe,)
BIN  := $(REPO)/bin/herdrcolor$(EXE)

.PHONY: help build test sync list snippet doctor link unlink release

help:
	@echo "make build     build bin/herdrcolor for this machine (needs Go)"
	@echo "make test      run the test suite (no running Herdr needed)"
	@echo "make sync      colour every agent now"
	@echo "make list      show assigned and reported colours"
	@echo "make snippet   print the sidebar config sections"
	@echo "make doctor    check Herdr, the sidebar config, and the colours"
	@echo "make link      build, then link this checkout into Herdr"
	@echo "make unlink    remove it again"
	@echo "make release   build every target and rewrite checksums.txt"

build:
	go build -o $(BIN) ./cmd/herdrcolor

test:
	go vet ./...
	go test ./...

sync: build
	$(BIN) sync --verbose

list: build
	$(BIN) list

snippet: build
	@$(BIN) snippet

doctor: build
	-$(BIN) doctor

link: build
	herdr plugin link $(REPO)

unlink:
	herdr plugin unlink herdrcolor

release:
	go run ./tools/release
