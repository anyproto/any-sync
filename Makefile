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

proto:
	@echo 'Generating protobuf packages (Go)...'
#   Uncomment if needed
	@$(eval ROOT_PKG := pkg)
	@$(eval GOGO_START := GOGO_NO_UNDERSCORE=1 GOGO_EXPORT_ONEOF_INTERFACE=1)
	@$(eval P_ACL_RECORDS_PATH_PB := $(ROOT_PKG)/acl/aclrecordproto)
	@$(eval P_TREE_CHANGES_PATH_PB := $(ROOT_PKG)/acl/treechangeproto)
	@$(eval P_SYNC_CHANGES_PATH_PB := syncproto)
	@$(eval P_TEST_CHANGES_PATH_PB := $(ROOT_PKG)/acl/testutils/testchanges)
	@$(eval P_ACL_RECORDS := M$(P_ACL_RECORDS_PATH_PB)/protos/aclrecord.proto=github.com/anytypeio/go-anytype-infrastructure-experiments/$(P_ACL_RECORDS_PATH_PB))
	@$(eval P_TREE_CHANGES := M$(P_TREE_CHANGES_PATH_PB)/protos/treechange.proto=github.com/anytypeio/go-anytype-infrastructure-experiments/$(P_TREE_CHANGES_PATH_PB))

	$(GOGO_START) protoc --gogofaster_out=:. $(P_ACL_RECORDS_PATH_PB)/protos/*.proto
	$(GOGO_START) protoc --gogofaster_out=:. $(P_TREE_CHANGES_PATH_PB)/protos/*.proto
	$(GOGO_START) protoc --gogofaster_out=:. $(P_TEST_CHANGES_PATH_PB)/proto/*.proto
	$(eval PKGMAP := $$(P_TREE_CHANGES),$$(P_ACL_RECORDS))
	$(GOGO_START) protoc --gogofaster_out=$(PKGMAP):. --go-drpc_out=protolib=github.com/gogo/protobuf:. common/commonspace/spacesyncproto/protos/*.proto
	$(GOGO_START) protoc --gogofaster_out=:. --go-drpc_out=protolib=github.com/gogo/protobuf:. consensus/consensusproto/protos/*.proto

build:
	@$(eval FLAGS := $$(shell govvv -flags -pkg github.com/anytypeio/go-anytype-infrastructure-experiments/app))
	go build -v -o bin/anytype-node -ldflags "$(FLAGS)" cmd/node/node.go

test-deps:
	@echo 'Generating test mocks...'
	@go install github.com/golang/mock/mockgen
	@go generate ./...

build-consensus:
	@$(eval FLAGS := $$(shell govvv -flags -pkg github.com/anytypeio/go-anytype-infrastructure-experiments/app))
	go build -v -o bin/consensus-node -ldflags "$(FLAGS)" github.com/anytypeio/go-anytype-infrastructure-experiments/cmd/consensusnode
