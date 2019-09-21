
# N/B: PACKER_BUILDS variable is required.  Should contain CSV of
# builder name(s) from applicable YAML file,
# e.g for names see libpod_images.yml

PACKER_VER ?= 1.4.2
GOARCH=$(shell go env GOARCH)
ARCH=$(uname -m)
PACKER_DIST_FILENAME := packer_${PACKER_VER}_linux_${GOARCH}.zip

# Only needed for libpod_base_images target
TIMESTAMP := $(shell date +%s)
GOSRC ?= $(shell realpath "./../../../")
PACKER_BASE ?= contrib/cirrus/packer
SCRIPT_BASE ?= contrib/cirrus
POST_MERGE_BUCKET_SUFFIX ?=

UBUNTU_BASE_IMAGE = $(shell source ../lib.sh && echo "$$UBUNTU_BASE_IMAGE")
PRIOR_UBUNTU_BASE_IMAGE = $(shell source ../lib.sh && echo "$$PRIOR_UBUNTU_BASE_IMAGE")

# For debugging nested-virt, use
#TTYDEV := $(shell tty)
TTYDEV := /dev/null

.PHONY: all
all: libpod_images

# Utility target for checking required parameters
.PHONY: guard-%
guard-%:
	@if [[ -z "$($*)" ]]; then \
		echo "Missing or empty required make variable '$*'."; \
		exit 1; \
	fi;

%.json: %.yml
	@python3 -c 'import json,yaml; json.dump( yaml.load(open("$<").read()), open("$@","w"), indent=2);'

${PACKER_DIST_FILENAME}:
	@curl -L --silent --show-error \
		-O https://releases.hashicorp.com/packer/${PACKER_VER}/${PACKER_DIST_FILENAME}

packer: ${PACKER_DIST_FILENAME}
	@curl -L --silent --show-error \
		https://releases.hashicorp.com/packer/${PACKER_VER}/packer_${PACKER_VER}_SHA256SUMS \
		| grep linux_${GOARCH} > /tmp/packer_sha256sums
	@sha256sum --check /tmp/packer_sha256sums
	@unzip -o ${PACKER_DIST_FILENAME}
	@touch --reference=Makefile ${PACKER_DIST_FILENAME}

.PHONY: test
test: libpod_base_images.json libpod_images.json packer
	./packer inspect libpod_base_images.json > /dev/null
	./packer inspect libpod_images.json > /dev/null
	@echo "All good"

.PHONY: libpod_images
libpod_images: guard-PACKER_BUILDS libpod_images.json packer
	./packer build -only=${PACKER_BUILDS} \
		-force \
		-var GOSRC=$(GOSRC) \
		-var PACKER_BASE=$(PACKER_BASE) \
		-var SCRIPT_BASE=$(SCRIPT_BASE) \
		libpod_images.json

cidata.ssh:
	ssh-keygen -f $@ -P "" -q

cidata.ssh.pub: cidata.ssh
	touch $@

meta-data:
	echo "local-hostname: localhost.localdomain" > $@

user-data: cidata.ssh.pub
	bash make-user-data.sh

cidata.iso: user-data meta-data
	genisoimage -output cidata.iso -volid cidata -input-charset utf-8 -joliet -rock user-data meta-data

# This is intended to be run by a human, with admin access to the libpod GCE project.
.PHONY: libpod_base_images
libpod_base_images: guard-GCP_PROJECT_ID guard-GOOGLE_APPLICATION_CREDENTIALS libpod_base_images.json cidata.iso cidata.ssh packer
	PACKER_CACHE_DIR=/tmp ./packer build \
		-force \
		-var TIMESTAMP=$(TIMESTAMP) \
		-var TTYDEV=$(TTYDEV) \
		-var GCP_PROJECT_ID=$(GCP_PROJECT_ID) \
		-var GOOGLE_APPLICATION_CREDENTIALS=$(GOOGLE_APPLICATION_CREDENTIALS) \
		-var GOSRC=$(GOSRC) \
		-var PACKER_BASE=$(PACKER_BASE) \
		-var SCRIPT_BASE=$(SCRIPT_BASE) \
		-var UBUNTU_BASE_IMAGE=$(UBUNTU_BASE_IMAGE) \
		-var PRIOR_UBUNTU_BASE_IMAGE=$(PRIOR_UBUNTU_BASE_IMAGE) \
		libpod_base_images.json
