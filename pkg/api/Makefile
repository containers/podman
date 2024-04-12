SWAGGER_OUT ?= swagger.yaml

validate: ${SWAGGER_OUT}
	swagger validate ${SWAGGER_OUT}

serve: ${SWAGGER_OUT}
	swagger serve -F redoc -p=8080 swagger.yaml

.PHONY: ${SWAGGER_OUT}
${SWAGGER_OUT}:
	# generate doesn't remove file on error
	rm -f ${SWAGGER_OUT}
	swagger generate spec -x github.com/sigstore/rekor -o ${SWAGGER_OUT} -i tags.yaml -w ./ -m
