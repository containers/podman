# Minimal makefile for Sphinx documentation
#

# You can set these variables from the command line, and also
# from the environment for the first two.
SPHINXOPTS    ?=
SPHINXBUILD   ?= sphinx-build
SOURCEDIR     = source
BUILDDIR      = build

# Put it first so that "make" without argument is like "make help".
help:
	@$(SPHINXBUILD) -M help "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)

clean:
	rm -fr build/
	rm -f source/man/podman-*.1.md

copy:
	cp -v ../podman-attach.1.md source/man/
	cp -v ../podman-build.1.md source/man/
	cp -v ../podman-commit.1.md source/man/
	# container
	cp -v ../podman-container-checkpoint.1.md  source/man/
	cp -v ../podman-container-exists.1.md source/man/
	cp -v ../podman-container-restore.1.md source/man/
	cp -v ../podman-container-cleanup.1.md source/man/
	cp -v ../podman-container-prune.1.md source/man/
	cp -v ../podman-container-runlabel.1.md source/man/
	cp -v ../podman-cp.1.md source/man/
	cp -v ../podman-create.1.md source/man/
	cp -v ../podman-diff.1.md source/man/
	cp -v ../podman-events.1.md source/man/
	cp -v ../podman-exec.1.md source/man/
	cp -v ../podman-export.1.md source/man/
	# generate
	cp -v ../podman-generate-systemd.1.md source/man/
	cp -v ../podman-generate-kube.1.md source/man/
	# healthcheck
	cp -v ../podman-healthcheck-run.1.md source/man/
	#cp -v ../podman-help.1.md source/
	cp -v ../podman-history.1.md source/man/
	# image
	cp -v ../podman-image-prune.1.md source/man/
	cp -v ../podman-image-tree.1.md source/man/
	cp -v ../podman-image-trust.1.md source/man/
	cp -v ../podman-image-exists.1.md source/man/
	cp -v ../podman-image-sign.1.md source/man/
	cp -v ../podman-images.1.md source/man/
	cp -v ../podman-import.1.md source/man/
	cp -v ../podman-info.1.md source/man/
	cp -v ../podman-init.1.md source/man/
	cp -v ../podman-inspect.1.md source/man/
	cp -v ../podman-kill.1.md source/man/
	cp -v ../podman-load.1.md source/man/
	cp -v ../podman-login.1.md source/man/
	cp -v ../podman-logout.1.md source/man/
	cp -v ../podman-logs.1.md source/man/
	cp -v ../podman-mount.1.md source/man/
	# network
	cp -v ../podman-network-create.1.md source/man/
	cp -v ../podman-network-ls.1.md source/man/
	cp -v ../podman-network-inspect.1.md source/man/
	cp -v ../podman-network-rm.1.md source/man/
	cp -v ../podman-pause.1.md source/man/
	# play
	cp -v ../podman-play-kube.1.md source/man/
	# pod
	cp -v ../podman-pod-create.1.md source/man/
	cp -v ../podman-pod-pause.1.md source/man/
	cp -v ../podman-pod-rm.1.md source/man/
	cp -v ../podman-pod-top.1.md source/man/
	cp -v ../podman-pod-exists.1.md source/man/
	cp -v ../podman-pod-prune.1.md source/man/
	cp -v ../podman-pod-start.1.md source/man/
	cp -v ../podman-pod-unpause.1.md source/man/
	cp -v ../podman-pod-inspect.1.md source/man/
	cp -v ../podman-pod-ps.1.md source/man/
	cp -v ../podman-pod-stats.1.md source/man/
	cp -v ../podman-pod-kill.1.md source/man/
	cp -v ../podman-pod-restart.1.md source/man/
	cp -v ../podman-pod-stop.1.md source/man/
	cp -v ../podman-port.1.md source/man/
	cp -v ../podman-ps.1.md source/man/
	cp -v ../podman-pull.1.md source/man/
	cp -v ../podman-push.1.md source/man/
	cp -v ../podman-restart.1.md source/man/
	cp -v ../podman-rm.1.md source/man/
	cp -v ../podman-rmi.1.md source/man/
	cp -v ../podman-run.1.md source/man/
	cp -v ../podman-save.1.md source/man/
	cp -v ../podman-search.1.md source/man/
	cp -v ../podman-start.1.md source/man/
	cp -v ../podman-stats.1.md source/man/
	cp -v ../podman-stop.1.md source/man/
	# system
	cp -v ../podman-system-migrate.1.md source/man/
	cp -v ../podman-system-renumber.1.md source/man/
	cp -v ../podman-system-df.1.md source/man/
	cp -v ../podman-system-prune.1.md source/man/
	cp -v ../podman-top.1.md source/man/
	cp -v ../podman-umount.1.md source/man/
	cp -v ../podman-unpause.1.md source/man/
	cp -v ../podman-unshare.1.md source/man/
	cp -v ../podman-varlink.1.md source/man/
	cp -v ../podman-version.1.md source/man/
	# volume
	cp -v ../podman-volume-inspect.1.md source/man/
	cp -v ../podman-volume-prune.1.md source/man/
	cp -v ../podman-volume-create.1.md source/man/
	cp -v ../podman-volume-ls.1.md source/man/
	cp -v ../podman-volume-rm.1.md source/man/
	cp -v ../podman-wait.1.md source/man/

.PHONY: help Makefile copy

html: copy
	@$(SPHINXBUILD) -M $@ "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)

# Catch-all target: route all unknown targets to Sphinx using the new
# "make mode" option.  $(O) is meant as a shortcut for $(SPHINXOPTS).
%: Makefile
	@$(SPHINXBUILD) -M $@ "$(SOURCEDIR)" "$(BUILDDIR)" $(SPHINXOPTS) $(O)
