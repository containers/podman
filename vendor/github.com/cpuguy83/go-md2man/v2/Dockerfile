ARG GO_VERSION=1.18
ARG GO_IMAGE=golang:${GO_VERSION}

FROM --platform=$BUILDPLATFORM $GO_IMAGE AS build
COPY . /go/src/github.com/cpuguy83/go-md2man
WORKDIR /go/src/github.com/cpuguy83/go-md2man
ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT
RUN \
    export GOOS="${TARGETOS}"; \
    export GOARCH="${TARGETARCH}"; \
    if [ "${TARGETARCH}" = "arm" ] && [ "${TARGETVARIANT}" ]; then \
    export GOARM="${TARGETVARIANT#v}"; \
    fi; \
    CGO_ENABLED=0 go build

FROM scratch
COPY --from=build /go/src/github.com/cpuguy83/go-md2man/go-md2man /go-md2man
ENTRYPOINT ["/go-md2man"]
