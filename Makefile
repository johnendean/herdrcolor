REPO   := $(shell pwd)
PYTHON := $(REPO)/.venv/bin/python
HERDRCOLOR := $(REPO)/bin/herdrcolor

.PHONY: help venv test sync list snippet doctor link unlink

help:
	@echo "make venv      create .venv (only needed to run the tests)"
	@echo "make test      run the test suite (no running Herdr needed)"
	@echo "make sync      colour every agent now"
	@echo "make list      show assigned and reported colours"
	@echo "make snippet   print the config.toml block for the palette"
	@echo "make doctor    check Herdr, the sidebar config, and the colours"
	@echo "make link      link this checkout into Herdr as a plugin"
	@echo "make unlink    remove it again"

venv:
	python3 -m venv .venv
	$(REPO)/.venv/bin/pip install -q --upgrade pip
	$(REPO)/.venv/bin/pip install -q -e '.[dev]'

test:
	$(PYTHON) -m pytest -q

sync:
	$(HERDRCOLOR) sync --verbose

list:
	$(HERDRCOLOR) list

snippet:
	@$(HERDRCOLOR) snippet

doctor:
	-$(HERDRCOLOR) doctor

link:
	herdr plugin link $(REPO)

unlink:
	herdr plugin unlink herdrcolor
