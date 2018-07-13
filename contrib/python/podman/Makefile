PYTHON ?= /usr/bin/python3

.PHONY: python-podman
python-podman:
	$(PYTHON) setup.py bdist

.PHONY: integration
integration:
	test/test_runner.sh

.PHONY: install
install:
	$(PYTHON) setup.py install --user

.PHONY: clean
clean:
	$(PYTHON) setup.py clean --all
	pip3 uninstall podman ||:
	rm -rf podman.egg-info dist
	find . -depth -name __pycache__ -exec rm -rf {} \;
	find . -depth -name \*.pyc -exec rm -f {} \;
