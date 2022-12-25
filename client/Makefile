.PHONY: proto
export GOPRIVATE=github.com/anytypeio

proto:
	@$(eval GOGO_START := GOGO_NO_UNDERSCORE=1 GOGO_EXPORT_ONEOF_INTERFACE=1)
	$(GOGO_START) protoc --gogofaster_out=:. --go-drpc_out=protolib=github.com/gogo/protobuf:. debug/clientdebugrpc/clientdebugrpcproto/protos/*.proto
	$(GOGO_START) protoc --gogofaster_out=:. document/textchangeproto/protos/*.proto

build:
	@$(eval FLAGS := $$(shell govvv -flags -pkg github.com/anytypeio/go-anytype-infrastructure-experiments/client))
	go build -v -o ../bin/client -ldflags "$(FLAGS)" github.com/anytypeio/go-anytype-infrastructure-experiments/client/cmd

test:
	go test ./... --cover
