export GO111MODULE=off

SWAGGER_OUT ?= swagger.yaml

.PHONY: ${SWAGGER_OUT}
${SWAGGER_OUT}:
	# generate doesn't remove file on error
	rm -f ${SWAGGER_OUT}
	swagger generate spec -o ${SWAGGER_OUT} -w ./

# TODO: when pass validation move it under swagger.
validate:
	swagger validate ${SWAGGER_OUT}
