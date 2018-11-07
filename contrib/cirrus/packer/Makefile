
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
