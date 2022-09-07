export GOPRIVATE=github.com/anytypeio

ifndef $(GOPATH)
    GOPATH=$(shell go env GOPATH)
    export GOPATH
endif

ifndef $(GOROOT)
    GOROOT=$(shell go env GOROOT)
    export GOROOT
endif

export PATH=$(GOPATH)/bin:$(shell echo $$PATH)

# TODO: folders were changed, so we should update Makefile and protos generation
proto:
	@echo 'Generating protobuf packages (Go)...'
#   Uncomment if needed
	@$(eval ROOT_PKG := pkg)
	@$(eval GOGO_START := GOGO_NO_UNDERSCORE=1 GOGO_EXPORT_ONEOF_INTERFACE=1)
	@$(eval P_ACL_CHANGES_PATH_PB := $(ROOT_PKG)/acl/aclchanges/aclpb)
	@$(eval P_SYNC_CHANGES_PATH_PB := syncproto)
	@$(eval P_TEST_CHANGES_PATH_PB := $(ROOT_PKG)/acl/testutils/testchanges)
	@$(eval P_TIMESTAMP := Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types)
	@$(eval P_STRUCT := Mgoogle/protobuf/struct.proto=github.com/gogo/protobuf/types)
	@$(eval P_ACL_CHANGES := M$(P_ACL_CHANGES_PATH_PB)/protos/aclchanges.proto=github.com/anytypeio/go-anytype-infrastructure-experiments/$(P_ACL_CHANGES_PATH_PB))

	# use if needed $(eval PKGMAP := $$(P_TIMESTAMP),$$(P_STRUCT))
	$(GOGO_START) protoc --gogofaster_out=:. $(P_ACL_CHANGES_PATH_PB)/protos/*.proto; mv $(P_ACL_CHANGES_PATH_PB)/protos/*.go $(P_ACL_CHANGES_PATH_PB)
	$(GOGO_START) protoc --gogofaster_out=:. $(P_TEST_CHANGES_PATH_PB)/proto/*.proto
	$(eval PKGMAP := $$(P_ACL_CHANGES))
	$(GOGO_START) protoc --gogofaster_out=$(PKGMAP):. $(P_SYNC_CHANGES_PATH_PB)/proto/*.proto
	$(GOGO_START) protoc --gogofaster_out=$(PKGMAP):. service/space/spacesync/protos/*.proto

build:
	@$(eval FLAGS := $$(shell govvv -flags -pkg github.com/anytypeio/go-anytype-infrastructure-experiments/app))
	go build -v -o bin/anytype-node -ldflags "$(FLAGS)" cmd/node/node.go