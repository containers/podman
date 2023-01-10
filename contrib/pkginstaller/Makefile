SHELL := bash

ARCH ?= aarch64
ifeq ($(ARCH), aarch64)
	GOARCH:=arm64
else
	GOARCH:=$(ARCH)
endif
GVPROXY_VERSION ?= 0.5.0
QEMU_VERSION ?= 7.1.0-1
GVPROXY_RELEASE_URL ?= https://github.com/containers/gvisor-tap-vsock/releases/download/v$(GVPROXY_VERSION)/gvproxy-darwin
QEMU_RELEASE_URL ?= https://github.com/containers/podman-machine-qemu/releases/download/v$(QEMU_VERSION)/podman-machine-qemu-$(ARCH)-$(QEMU_VERSION).tar.xz
PACKAGE_DIR ?= out/packaging
TMP_DOWNLOAD ?= tmp-download
PACKAGE_ROOT ?= root
PKG_NAME := podman-installer-macos-$(GOARCH).pkg

default: pkginstaller

podman_version:
	make -C ../../ test/version/version

$(TMP_DOWNLOAD)/gvproxy:
	mkdir -p $(TMP_DOWNLOAD)
	cd $(TMP_DOWNLOAD) && curl -sLo gvproxy $(GVPROXY_RELEASE_URL)

$(TMP_DOWNLOAD)/podman-machine-qemu-$(ARCH)-$(QEMU_VERSION).tar.xz:
	mkdir -p $(TMP_DOWNLOAD)
	cd $(TMP_DOWNLOAD) && curl -sLO $(QEMU_RELEASE_URL)

packagedir: podman_version package_root Distribution welcome.html
	mkdir -p $(PACKAGE_DIR)
	cp -r Resources $(PACKAGE_DIR)/
	cp welcome.html $(PACKAGE_DIR)/Resources/
	cp Distribution $(PACKAGE_DIR)/
	cp -r scripts $(PACKAGE_DIR)/
	cp -r $(PACKAGE_ROOT) $(PACKAGE_DIR)/
	cp package.sh $(PACKAGE_DIR)/
	cd $(PACKAGE_DIR) && pkgbuild --analyze --root ./root component.plist
	../../test/version/version > $(PACKAGE_DIR)/VERSION
	echo -n $(ARCH) > $(PACKAGE_DIR)/ARCH
	cp ../../LICENSE $(PACKAGE_DIR)/Resources/LICENSE.txt
	cp hvf.entitlements $(PACKAGE_DIR)/

package_root: clean-pkgroot $(TMP_DOWNLOAD)/podman-machine-qemu-$(ARCH)-$(QEMU_VERSION).tar.xz $(TMP_DOWNLOAD)/gvproxy
	mkdir -p $(PACKAGE_ROOT)/podman/bin $(PACKAGE_ROOT)/podman/qemu
	tar -C $(PACKAGE_ROOT)/podman/qemu -xf $(TMP_DOWNLOAD)/podman-machine-qemu-$(ARCH)-$(QEMU_VERSION).tar.xz
	cp $(TMP_DOWNLOAD)/gvproxy $(PACKAGE_ROOT)/podman/bin/
	chmod a+x $(PACKAGE_ROOT)/podman/bin/*

%: %.in podman_version
	@sed -e 's/__VERSION__/'$(shell ../../test/version/version)'/g' $< >$@

pkginstaller: packagedir
	cd $(PACKAGE_DIR) && ./package.sh ..

_notarize: pkginstaller
	xcrun notarytool submit --apple-id $(NOTARIZE_USERNAME) --password $(NOTARIZE_PASSWORD) --team-id=$(NOTARIZE_TEAM) -f json --wait out/$(PKG_NAME)

notarize: _notarize
	xcrun stapler staple out/$(PKG_NAME)

.PHONY: clean clean-pkgroot
clean:
	rm -rf $(TMP_DOWNLOAD) $(PACKAGE_ROOT) $(PACKAGE_DIR) Distribution welcome.html ../../test/version/version

clean-pkgroot:
	rm -rf $(PACKAGE_ROOT) $(PACKAGE_DIR) Distribution welcome.html
