PYTHON ?= /usr/bin/python3

.PHONY: python-podman
python-podman:
	$(PYTHON) setup.py bdist

.PHONY: integration
integration:
	test/test_runner.sh

.PHONY: clean
clean:
	$(PYTHON) setup.py clean --all
	rm -rf podman.egg-info dist
	find . -depth -name __pycache__ -exec rm -rf {} \;
