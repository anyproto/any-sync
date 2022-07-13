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
protos-go:
	@echo 'Generating protobuf packages (Go)...'
	$(eval P_TIMESTAMP := Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types)
	$(eval P_STRUCT := Mgoogle/protobuf/struct.proto=github.com/gogo/protobuf/types)
	@$(eval ROOT_PKG := pkg)
	@$(eval P_TREE_STORAGE_PATH_PB := $(ROOT_PKG)/acl/treestorage/pb)
	@$(eval P_ACL_CHANGES_PATH_PB := $(ROOT_PKG)/acl/aclchanges/pb)
	@$(eval P_PLAINTEXT_CHANGES_PATH_PB := $(ROOT_PKG)/acl/testutils/testchanges/pb)

	# TODO: check if PKGMAP should include other proto files
	$(eval PKGMAP := $$(P_TIMESTAMP),$$(P_STRUCT))
	GOGO_NO_UNDERSCORE=1 GOGO_EXPORT_ONEOF_INTERFACE=1 protoc --gogofaster_out=$(PKGMAP):./$(P_ACL_CHANGES_PATH_PB) $(P_ACL_CHANGES_PATH_PB)/protos/*.*; mv $(P_ACL_CHANGES_PATH_PB)/$(P_ACL_CHANGES_PATH_PB)/protos/*.go $(P_ACL_CHANGES_PATH_PB); rm -rf $(P_ACL_CHANGES_PATH_PB)/$(ROOT_PKG)
	$(eval PKGMAP := $$(P_TIMESTAMP),$$(P_STRUCT))
	GOGO_NO_UNDERSCORE=1 GOGO_EXPORT_ONEOF_INTERFACE=1 protoc --gogofaster_out=$(PKGMAP):./$(P_TREE_STORAGE_PATH_PB) $(P_TREE_STORAGE_PATH_PB)/protos/*.*; mv $(P_TREE_STORAGE_PATH_PB)/$(P_TREE_STORAGE_PATH_PB)/protos/*.go $(P_TREE_STORAGE_PATH_PB); rm -rf $(P_TREE_STORAGE_PATH_PB)/$(ROOT_PKG)
	$(eval PKGMAP := $$(P_TIMESTAMP),$$(P_STRUCT))
	GOGO_NO_UNDERSCORE=1 GOGO_EXPORT_ONEOF_INTERFACE=1 protoc --gogofaster_out=$(PKGMAP):./$(P_PLAINTEXT_CHANGES_PATH_PB) $(P_PLAINTEXT_CHANGES_PATH_PB)/protos/*.*; mv $(P_PLAINTEXT_CHANGES_PATH_PB)/$(P_PLAINTEXT_CHANGES_PATH_PB)/protos/*.go $(P_PLAINTEXT_CHANGES_PATH_PB); rm -rf $(P_PLAINTEXT_CHANGES_PATH_PB)/$(ROOT_PKG)

build:
	@$(eval FLAGS := $$(shell govvv -flags -pkg github.com/anytypeio/go-anytype-infrastructure-experiments/app))
	go build -o bin/anytype-node -ldflags "$(FLAGS)" cmd/node/node.go