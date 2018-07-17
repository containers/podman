PYTHON ?= /usr/bin/python3

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
	$(PYTHON) setup.py install

.PHONY: clobber
clobber: uninstall clean

.PHONY: uninstall
	$(PYTHON) -m pip uninstall --yes pypodman ||:

.PHONY: clean
clean:
	$(PYTHON) setup.py clean --all
	rm -rf pypodman.egg-info dist
	find . -depth -name __pycache__ -exec rm -rf {} \;
	find . -depth -name \*.pyc -exec rm -f {} \;
