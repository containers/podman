SWAGGER_OUT ?= swagger.yaml

swagger:
	swagger generate spec -o ${SWAGGER_OUT} -w ./
	cat tags.yaml >> swagger.yaml
