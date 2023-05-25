.PHONY: rpm
rpm:
	$(shell /usr/bin/bash ./update-spec-version.sh)
	spectool -g podman.spec
	rpmbuild -ba \
		--define '_sourcedir $(shell pwd)' \
		--define '_rpmdir %{_sourcedir}/RPMS' \
		--define '_srcrpmdir %{_sourcedir}/SRPMS' \
		--define '_builddir %{_sourcedir}/BUILD' \
		podman.spec
