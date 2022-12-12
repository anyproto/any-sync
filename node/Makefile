.PHONY: proto build test
export GOPRIVATE=github.com/anytypeio

build:
	@$(eval FLAGS := $$(shell govvv -flags -pkg github.com/anytypeio/go-anytype-infrastructure-experiments/node))
	go build -v -o ../bin/anytype-node -ldflags "$(FLAGS)" github.com/anytypeio/go-anytype-infrastructure-experiments/node/cmd

test:
	go test ./... --cover

proto:
	@$(eval GOGO_START := GOGO_NO_UNDERSCORE=1 GOGO_EXPORT_ONEOF_INTERFACE=1)
	$(GOGO_START) protoc --gogofaster_out=:. --go-drpc_out=protolib=github.com/gogo/protobuf:. api/apiproto/protos/*.proto