.PHONY: proto build test
export GOPRIVATE=github.com/anytypeio

proto:
	protoc --gogofaster_out=:. --go-drpc_out=protolib=github.com/gogo/protobuf:. consensusproto/protos/*.proto

build:
	@$(eval FLAGS := $$(shell govvv -flags -pkg github.com/anytypeio/go-anytype-infrastructure-experiments/consensus))
	go build -v -o ../bin/anytype-consensus -ldflags "$(FLAGS)" github.com/anytypeio/go-anytype-infrastructure-experiments/consensus/cmd

test:
	go test ./... --cover
