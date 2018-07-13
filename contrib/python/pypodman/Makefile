PYTHON ?= /usr/bin/python3

.PHONY: python-pypodman
python-pypodman:
	$(PYTHON) setup.py bdist

.PHONY: integration
integration:
	true

.PHONY: install
install:
	$(PYTHON) setup.py install --user

.PHONY: clean
clean:
	$(PYTHON) setup.py clean --all
	pip3 uninstall pypodman ||:
	rm -rf pypodman.egg-info dist
	find . -depth -name __pycache__ -exec rm -rf {} \;
	find . -depth -name \*.pyc -exec rm -f {} \;
