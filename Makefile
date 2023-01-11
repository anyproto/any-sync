.PHONY: proto test test-coverage vet deps
export GOPRIVATE=github.com/anytypeio/*
export GIT_TERMINAL_PROMPT=1
export PATH:=deps:$(PATH)

proto:
	@echo 'Generating protobuf packages (Go)...'

	@$(eval P_ACL_RECORDS_PATH_PB := commonspace/object/acl/aclrecordproto)
	@$(eval P_TREE_CHANGES_PATH_PB := commonspace/object/tree/treechangeproto)
	@$(eval P_ACL_RECORDS := M$(P_ACL_RECORDS_PATH_PB)/protos/aclrecord.proto=github.com/anytypeio/any-sync/$(P_ACL_RECORDS_PATH_PB))
	@$(eval P_TREE_CHANGES := M$(P_TREE_CHANGES_PATH_PB)/protos/treechange.proto=github.com/anytypeio/any-sync/$(P_TREE_CHANGES_PATH_PB))

	protoc --gogofaster_out=:. $(P_ACL_RECORDS_PATH_PB)/protos/*.proto
	protoc --gogofaster_out=:. $(P_TREE_CHANGES_PATH_PB)/protos/*.proto
	$(eval PKGMAP := $$(P_TREE_CHANGES),$$(P_ACL_RECORDS))
	protoc --gogofaster_out=$(PKGMAP):. --go-drpc_out=protolib=github.com/gogo/protobuf:. commonspace/spacesyncproto/protos/*.proto
	protoc --gogofaster_out=$(PKGMAP):. --go-drpc_out=protolib=github.com/gogo/protobuf:. commonfile/fileproto/protos/*.proto

vet:
	go vet ./...

test:
	go test ./... --cover

test-coverage:
	go test ./... -coverprofile coverage.out -covermode count
	go tool cover -func coverage.out

deps:
	go mod download
	go build -o deps storj.io/drpc/cmd/protoc-gen-go-drpc
	go build -o deps github.com/gogo/protobuf/protoc-gen-gogofaster
