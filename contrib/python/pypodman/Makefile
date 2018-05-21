PYTHON ?= $(shell command -v python3 2>/dev/null || command -v python)
DESTDIR := /

.PHONY: python-pypodman
python-pypodman:
	$(PYTHON) setup.py sdist bdist

.PHONY: lint
lint:
	$(PYTHON) -m pylint pypodman

.PHONY: integration
integration:
	true

.PHONY: install
install:
	$(PYTHON) setup.py install --root ${DESTDIR}

.PHONY: clobber
clobber: uninstall clean

.PHONY: uninstall
	$(PYTHON) -m pip uninstall --yes pypodman ||:

.PHONY: clean
clean:
	rm -rf pypodman.egg-info dist
	find . -depth -name __pycache__ -exec rm -rf {} \;
	find . -depth -name \*.pyc -exec rm -f {} \;
	$(PYTHON) ./setup.py clean --all
