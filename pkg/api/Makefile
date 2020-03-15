export GO111MODULE=off

SWAGGER_OUT ?= swagger.yaml

validate: ${SWAGGER_OUT}
	swagger validate ${SWAGGER_OUT}

.PHONY: ${SWAGGER_OUT}
${SWAGGER_OUT}:
	# generate doesn't remove file on error
	rm -f ${SWAGGER_OUT}
	swagger generate spec -o ${SWAGGER_OUT} -i tags.yaml -w ./ -m
