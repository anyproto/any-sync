.PHONY: proto
export GOPRIVATE=github.com/anytypeio

proto:
	@$(eval GOGO_START := GOGO_NO_UNDERSCORE=1 GOGO_EXPORT_ONEOF_INTERFACE=1)
	$(GOGO_START) protoc --gogofaster_out=:. --go-drpc_out=protolib=github.com/gogo/protobuf:. api/apiproto/protos/*.proto