PYTHON ?= /usr/bin/python3
DESTDIR ?= /
PODMAN_VERSION ?= '0.0.4'

.PHONY: python-podman
python-podman:
	$(PYTHON) setup.py sdist bdist

.PHONY: lint
lint:
	$(PYTHON) -m pylint podman

.PHONY: integration
integration:
	test/test_runner.sh -v

.PHONY: install
install:
	$(PYTHON) setup.py install --root ${DESTDIR}

.PHONY: upload
upload:
	$(PODMAN_VERSION) $(PYTHON) setup.py sdist bdist_wheel
	twine upload --repository-url https://test.pypi.org/legacy/ dist/*

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
