.PHONY: proto test test-coverage vet
export GOPRIVATE=github.com/anytypeio

proto:
	@echo 'Generating protobuf packages (Go)...'

	@$(eval GOGO_START := GOGO_NO_UNDERSCORE=1 GOGO_EXPORT_ONEOF_INTERFACE=1)
	@$(eval P_ACL_RECORDS_PATH_PB := commonspace/object/acl/aclrecordproto)
	@$(eval P_TREE_CHANGES_PATH_PB := commonspace/object/tree/treechangeproto)
	@$(eval P_ACL_RECORDS := M$(P_ACL_RECORDS_PATH_PB)/protos/aclrecord.proto=github.com/anytypeio/any-sync/$(P_ACL_RECORDS_PATH_PB))
	@$(eval P_TREE_CHANGES := M$(P_TREE_CHANGES_PATH_PB)/protos/treechange.proto=github.com/anytypeio/any-sync/$(P_TREE_CHANGES_PATH_PB))

	$(GOGO_START) protoc --gogofaster_out=:. $(P_ACL_RECORDS_PATH_PB)/protos/*.proto
	$(GOGO_START) protoc --gogofaster_out=:. $(P_TREE_CHANGES_PATH_PB)/protos/*.proto
	$(eval PKGMAP := $$(P_TREE_CHANGES),$$(P_ACL_RECORDS))
	$(GOGO_START) protoc --gogofaster_out=$(PKGMAP):. --go-drpc_out=protolib=github.com/gogo/protobuf:. commonspace/spacesyncproto/protos/*.proto
	$(GOGO_START) protoc --gogofaster_out=$(PKGMAP):. --go-drpc_out=protolib=github.com/gogo/protobuf:. commonfile/fileproto/protos/*.proto

vet:
	go vet ./...

test:
	go test ./... --cover

test-coverage:
	go test ./... -coverprofile coverage.out -covermode count
	go tool cover -func coverage.out
