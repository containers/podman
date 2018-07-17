PYTHON ?= /usr/bin/python3

.PHONY: python-podman
python-podman:
	$(PYTHON) setup.py sdist bdist

.PHONY: lint
lint:
	$(PYTHON) -m pylint podman

.PHONY: integration
integration:
	test/test_runner.sh

.PHONY: install
install:
	$(PYTHON) setup.py install

.PHONY: clobber
clobber: uninstall clean

.PHONY: uninstall
uninstall:
	$(PYTHON) -m pip uninstall --yes podman ||:

.PHONY: clean
clean:
	$(PYTHON) setup.py clean --all
	rm -rf podman.egg-info dist
	find . -depth -name __pycache__ -exec rm -rf {} \;
	find . -depth -name \*.pyc -exec rm -f {} \;
