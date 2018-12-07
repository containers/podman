PYTHON ?= $(shell command -v python3 2>/dev/null || command -v python)
DESTDIR := /
PODMAN_VERSION ?= '0.11.1.1'

.PHONY: python-pypodman
python-pypodman:
	PODMAN_VERSION=$(PODMAN_VERSION) \
	$(PYTHON) setup.py sdist bdist

.PHONY: lint
lint:
	$(PYTHON) -m pylint pypodman

.PHONY: integration
integration:
	true

.PHONY: install
install:
	PODMAN_VERSION=$(PODMAN_VERSION) \
	$(PYTHON) setup.py install --root ${DESTDIR}

.PHONY: upload
upload:
	PODMAN_VERSION=$(PODMAN_VERSION) $(PYTHON) setup.py sdist bdist_wheel
	twine upload --repository-url https://test.pypi.org/legacy/ dist/*

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
