SHELL := bash

ARCH ?= aarch64
PODMAN_VERSION ?= 4.1.0
GVPROXY_VERSION ?= 0.4.0
QEMU_VERSION ?= 7.0.0-2
GVPROXY_RELEASE_URL ?= https://github.com/containers/gvisor-tap-vsock/releases/download/v$(GVPROXY_VERSION)/gvproxy-darwin
QEMU_RELEASE_URL ?= https://github.com/containers/podman-machine-qemu/releases/download/v$(QEMU_VERSION)/podman-machine-qemu-$(ARCH)-$(QEMU_VERSION).tar.xz
PACKAGE_DIR ?= out/packaging
TMP_DOWNLOAD ?= tmp-download
PACKAGE_ROOT ?= root

default: pkginstaller

get_gvproxy:
	mkdir -p $(TMP_DOWNLOAD)
	cd $(TMP_DOWNLOAD) && curl -sLo gvproxy $(GVPROXY_RELEASE_URL)

get_qemu:
	mkdir -p $(TMP_DOWNLOAD)
	cd $(TMP_DOWNLOAD) && curl -sLO $(QEMU_RELEASE_URL)

packagedir: package_root Distribution welcome.html
	mkdir -p $(PACKAGE_DIR)
	cp -r Resources $(PACKAGE_DIR)/
	cp welcome.html $(PACKAGE_DIR)/Resources/
	cp Distribution $(PACKAGE_DIR)/
	cp -r scripts $(PACKAGE_DIR)/
	cp -r $(PACKAGE_ROOT) $(PACKAGE_DIR)/
	cp package.sh $(PACKAGE_DIR)/
	cd $(PACKAGE_DIR) && pkgbuild --analyze --root ./root component.plist
	echo -n $(PODMAN_VERSION) > $(PACKAGE_DIR)/VERSION
	echo -n $(ARCH) > $(PACKAGE_DIR)/ARCH
	cp ../../LICENSE $(PACKAGE_DIR)/Resources/LICENSE.txt
	cp hvf.entitlements $(PACKAGE_DIR)/

package_root: get_gvproxy get_qemu
	mkdir -p $(PACKAGE_ROOT)/podman/bin $(PACKAGE_ROOT)/podman/qemu
	tar -C $(PACKAGE_ROOT)/podman/qemu -xf $(TMP_DOWNLOAD)/podman-machine-qemu-$(ARCH)-$(QEMU_VERSION).tar.xz
	cp $(TMP_DOWNLOAD)/gvproxy $(PACKAGE_ROOT)/podman/bin/
	chmod a+x $(PACKAGE_ROOT)/podman/bin/*

%: %.in
	@sed -e 's/__VERSION__/'$(PODMAN_VERSION)'/g' $< >$@

pkginstaller: packagedir
	cd $(PACKAGE_DIR) && ./package.sh ..

.PHONY: clean
clean:
	rm -rf $(TMP_DOWNLOAD) $(PACKAGE_ROOT) $(PACKAGE_DIR) Distribution welcome.html
