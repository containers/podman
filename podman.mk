################################################################################
#
# podman
#
################################################################################

PODMAN_VERSION = 5.0.3

PODMAN_SITE = https://github.com/containers/podman/releases/v$(PODMAN_VERSION)
PODMAN_SOURCE = podman-$(PODMAN_VERSION).tar.gz

PODMAN_LICENSE = Apache-2.0
PODMAN_LICENSE_FILES = LICENSE

PODMAN_DEPENDENCIES = libseccomp libgpg-error libassuan libgpgme

define PODMAN_BUILD_CMDS
    $(MAKE) -C $(@D) \
        prefix=$(TARGET_DIR) \
        install
endef

$(eval $(generic-package))
