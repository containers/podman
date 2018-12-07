
# N/B: PACKER_BUILDS variable is required.  Should contain CSV of
# builder name(s) from applicable YAML file,
# e.g for names see libpod_images.yml

PACKER_VER ?= 1.3.1
PACKER_DIST_FILENAME := packer_${PACKER_VER}_linux_amd64.zip

# Only needed for libpod_base_images target
TIMESTAMP := $(shell date +%s)
GOSRC ?= $(shell realpath "./../../../")
PACKER_BASE ?= contrib/cirrus/packer
SCRIPT_BASE ?= contrib/cirrus

# For debugging nested-virt, use
#TTYDEV := $(shell tty)
TTYDEV := /dev/null

.PHONY: all
all: libpod_images

%.json: %.yml
	@python3 -c 'import json,yaml; json.dump( yaml.load(open("$<").read()), open("$@","w"), indent=2);'

${PACKER_DIST_FILENAME}:
	@curl -L --silent --show-error \
		-O https://releases.hashicorp.com/packer/${PACKER_VER}/${PACKER_DIST_FILENAME}

packer: ${PACKER_DIST_FILENAME}
	@curl -L --silent --show-error \
		https://releases.hashicorp.com/packer/${PACKER_VER}/packer_${PACKER_VER}_SHA256SUMS \
		| grep 'linux_amd64' > /tmp/packer_sha256sums
	@sha256sum --check /tmp/packer_sha256sums
	@unzip -o ${PACKER_DIST_FILENAME}
	@touch --reference=Makefile ${PACKER_DIST_FILENAME}

.PHONY: test
test: libpod_base_images.json libpod_images.json packer
	./packer inspect libpod_base_images.json > /dev/null
	./packer inspect libpod_images.json > /dev/null
	@echo "All good"

.PHONY: libpod_images
libpod_images: libpod_images.json packer
ifndef PACKER_BUILDS
	$(error PACKER_BUILDS is undefined, expected builder-names CSV)
endif
	./packer build -only=${PACKER_BUILDS} \
		-var GOSRC=$(GOSRC) \
		-var PACKER_BASE=$(PACKER_BASE) \
		-var SCRIPT_BASE=$(SCRIPT_BASE) \
		libpod_images.json
	@echo ""
	@echo "Finished.  The images mentioned above, and in packer-manifest.json"
	@echo "can be used in .cirrus.yml as values for the 'image_name' keys"
	@echo ""

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
libpod_base_images: libpod_base_images.json cidata.iso cidata.ssh packer
ifndef GCP_PROJECT_ID
	$(error GCP_PROJECT_ID is undefined, expected complete GCP project ID string e.g. foobar-12345)
endif
ifndef GOOGLE_APPLICATION_CREDENTIALS
	$(error GOOGLE_APPLICATION_CREDENTIALS is undefined, expected absolute path to JSON file, like $HOME/.config/gcloud/legacy_credentials/*/adc.json)
endif
ifndef RHEL_IMAGE_FILE
	$(error RHEL_IMAGE_FILE is undefined, expected full path to a rhel-server-ec2-*.raw.xz file)
endif
ifndef RHEL_CSUM_FILE
	$(error RHEL_CSUM_FILE is undefined, expected full path to a rhel-server-ec2-*.raw.xz.SHA256SUM file)
endif
ifndef RHSM_COMMAND
	$(error RHSM_COMMAND is undefined, expected string required for temporarily registering VM)
endif
	PACKER_CACHE_DIR=/tmp ./packer build \
		-var TIMESTAMP=$(TIMESTAMP) \
		-var TTYDEV=$(TTYDEV) \
		-var GCP_PROJECT_ID=$(GCP_PROJECT_ID) \
		-var GOOGLE_APPLICATION_CREDENTIALS=$(GOOGLE_APPLICATION_CREDENTIALS) \
		-var GOSRC=$(GOSRC) \
		-var PACKER_BASE=$(PACKER_BASE) \
		-var SCRIPT_BASE=$(SCRIPT_BASE) \
		-var RHEL_BASE_IMAGE_NAME=$(shell basename $(RHEL_IMAGE_FILE) | tr -d '[[:space:]]' | sed -r -e 's/\.x86_64\.raw\.xz//' | tr '[[:upper:]]' '[[:lower:]]' | tr '[[:punct:]]' '-') \
		-var RHEL_IMAGE_FILE=$(RHEL_IMAGE_FILE) \
		-var RHEL_CSUM_FILE=$(RHEL_CSUM_FILE) \
	    -var 'RHSM_COMMAND=$(RHSM_COMMAND)' \
		-only $(PACKER_BUILDS) \
		libpod_base_images.json
	@echo ""
	@echo "Finished.  The images mentioned above, and in packer-manifest.json"
	@echo "can be used in .cirrus.yml as values for the *_BASE_IMAGE keys."
	@echo ""
