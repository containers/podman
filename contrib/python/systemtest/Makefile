PYTHON ?= /usr/bin/python3

.PHONY: all
all: smoke system

.PHONY: lint
lint:
	# TODO: pylint sucks
	$(PYTHON) -m pylint pypodman

# Unittests for library code/modules used by the actual system tests
.PHONY: unit
unit:
	$(PYTHON) -m unittest discover --verbose --pattern 'unittest*.py'

# Simple 'basic functionality' tests
.PHONY: smoke
smoke:
	$(PYTHON) -m unittest discover --verbose --pattern 'smoketest*.py'

# Main, primary system tests
.PHONY: system
system:
	# TODO: Incorporate
	#       ../../../tests/test_podman_baseline.sh
	#       ../../../tests/test_podman_build.sh
	$(PYTHON) -m unittest discover --verbose --pattern 'systemtest*.py'

# Remove all local modifications / clear cache
.PHONY: clean
clean:
