all: cmd/varlink-go-certification/orgvarlinkcertification/orgvarlinkcertification.go
	go test ./...

cmd/varlink-go-certification/orgvarlinkcertification/orgvarlinkcertification.go: cmd/varlink-go-certification/orgvarlinkcertification/org.varlink.certification.varlink
	go generate cmd/varlink-go-certification/orgvarlinkcertification/generate.go

.PHONY: all
