#!/bin/bash

set -e

source $(dirname $0)/lib.sh

git clone https://github.com/go-swagger/go-swagger $GOSRC/github.com/go-swagger/go-swagger
cd $GOSRC/github.com/go-swagger/go-swagger
go install ./cmd/swagger

SWAG_OUT=swagger-latest.yaml make swagger
