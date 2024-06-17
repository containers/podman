# Define the directories for installation
PREFIX ?= /usr/
BINDIR = $(PREFIX)/bin/
SHAREDIR = $(PREFIX)/share/podman-stressor
SHAREDIR_DOC = $(PREFIX)/share/doc/podman-stressor
CONFIGDIR = $(HOME)/.config/podman-stressor

# Define the list of scripts and files
SCRIPTS = cgroup \
	memory \
	podman \
	network \
	podman-operations \
	processes \
	volume \
	stress \
	systemd \
	system \
	date \
	rpm \
	common \
	selinux

BIN_FILE = podman-stressor

DOCS = README.md LICENSE SECURITY.md NOTICE

CONFIG_FILE = constants

# Default target
all:
	@echo "Available targets: install, uninstall"

# Install target
install:
	@install -d $(DESTDIR)$(SHAREDIR)
	@install -d $(DESTDIR)$(SHAREDIR_DOC)
	@install -d $(DESTDIR)$(BINDIR)
	@install -d $(DESTDIR)$(CONFIGDIR)
	@for script in $(SCRIPTS); do \
                install -m 755 $$script $(DESTDIR)$(SHAREDIR); \
        done
	@for doc in $(DOCS); do \
                install -m 644 $$doc $(DESTDIR)$(SHAREDIR_DOC); \
        done
	@install -m 755 $(BIN_FILE) $(DESTDIR)$(BINDIR)
	@install -m 644 $(CONFIG_FILE) $(DESTDIR)$(CONFIGDIR)/$(CONFIG_FILE)
	@if ! grep -q '^SHARE_DIR=$(SHAREDIR)' $(DESTDIR)$(CONFIGDIR)/$(CONFIG_FILE); then \
                echo 'SHARE_DIR=$(SHAREDIR)' >> $(DESTDIR)$(CONFIGDIR)/$(CONFIG_FILE); \
        fi
	@echo "Installation complete."

# Uninstall target
uninstall:
	@for script in $(SCRIPTS); do \
                rm -f $(DESTDIR)$(SHAREDIR)/$$script; \
        done
	rm -rf $(DESTDIR)$(SHAREDIR)
	rm -rf $(DESTDIR)$(SHAREDIR_DOC)
	rm -rf $(DESTDIR)$(CONFIGDIR)
	rm -f $(DESTDIR)$(BINDIR)/$(BIN_FILE)
	@echo "Uninstallation complete."

.PHONY: all install uninstall
