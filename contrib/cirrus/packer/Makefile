
PACKER_VER ?= 1.3.1
PACKER_BUILDS ?= fah-28,fedora-28
PACKER_DIST_FILENAME := packer_${PACKER_VER}_linux_amd64.zip
PACKER_DATA_FILENAME := libpod_images.json

.PHONY: all

all: libpod_images

${PACKER_DATA_FILENAME}: libpod_images.yml
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

.PHONY: test
test: ${PACKER_DATA_FILENAME} packer
	./packer inspect ${PACKER_DATA_FILENAME}

.PHONY: libpod_images
libpod_images: ${PACKER_DATA_FILENAME} packer
	./packer build -only=${PACKER_BUILDS} ${PACKER_DATA_FILENAME}
