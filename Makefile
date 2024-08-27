.PHONY: proto test deps
export GOPRIVATE=github.com/anyproto
export PATH:=deps:$(PATH)

all:
	@set -e;
	@git config core.hooksPath .githooks;

proto: proto-execute replace-gogo-strings

proto-execute:
	@echo 'Generating protobuf packages (Go)...'

	@$(eval P_ACL_RECORDS_PATH_PB := commonspace/object/acl/aclrecordproto)
	@$(eval P_TREE_CHANGES_PATH_PB := commonspace/object/tree/treechangeproto)
	@$(eval P_CRYPTO_PATH_PB := util/crypto/cryptoproto)
	@$(eval P_ACL_RECORDS := M$(P_ACL_RECORDS_PATH_PB)/protos/aclrecord.proto=github.com/anyproto/any-sync/$(P_ACL_RECORDS_PATH_PB))
	@$(eval P_TREE_CHANGES := M$(P_TREE_CHANGES_PATH_PB)/protos/treechange.proto=github.com/anyproto/any-sync/$(P_TREE_CHANGES_PATH_PB))

	protoc --gogofaster_out=:. $(P_ACL_RECORDS_PATH_PB)/protos/*.proto
	protoc --gogofaster_out=:. $(P_TREE_CHANGES_PATH_PB)/protos/*.proto
	protoc --gogofaster_out=:. $(P_CRYPTO_PATH_PB)/protos/*.proto
	$(eval PKGMAP := $$(P_TREE_CHANGES),$$(P_ACL_RECORDS))
	protoc --gogofaster_out=$(PKGMAP):. --go-drpc_out=protolib=github.com/gogo/protobuf:. commonspace/spacesyncproto/protos/*.proto
	protoc --gogofaster_out=$(PKGMAP):. --go-drpc_out=protolib=github.com/gogo/protobuf:. commonfile/fileproto/protos/*.proto
	protoc --gogofaster_out=$(PKGMAP):. --go-drpc_out=protolib=github.com/gogo/protobuf:. net/streampool/testservice/protos/*.proto
	protoc --gogofaster_out=:. net/secureservice/handshake/handshakeproto/protos/*.proto
	protoc --gogofaster_out=:. net/rpc/limiter/limiterproto/protos/*.proto
	protoc --gogofaster_out=$(PKGMAP):. --go-drpc_out=protolib=github.com/gogo/protobuf:. coordinator/coordinatorproto/protos/*.proto
	protoc --gogofaster_out=:. --go-drpc_out=protolib=github.com/gogo/protobuf:. consensus/consensusproto/protos/*.proto
	protoc --gogofaster_out=:. --go-drpc_out=protolib=github.com/gogo/protobuf:. identityrepo/identityrepoproto/protos/*.proto
	protoc --gogofaster_out=:. --go-drpc_out=protolib=github.com/gogo/protobuf:. nameservice/nameserviceproto/protos/*.proto
	protoc --gogofaster_out=:. --go-drpc_out=protolib=github.com/gogo/protobuf:. paymentservice/paymentserviceproto/protos/*.proto

deps:
	go mod download
	go build -o deps storj.io/drpc/cmd/protoc-gen-go-drpc
	go build -o deps github.com/anyproto/protobuf/protoc-gen-gogofaster

test:
	go test ./... --cover

replace-gogo-strings:
	@echo "Replacing 'github.com/gogo/protobuf' with 'github.com/anyproto/protobuf' in all files recursively..."
	LC_CTYPE=C LANG=C find . -type f -name "*.go" | xargs sed -i '' "s|github.com/gogo/protobuf|github.com/anyproto/protobuf|g"
