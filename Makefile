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
	@$(eval P_ACL_CHANGES := Mdata/pb/protos/aclchanges.proto=github.com/anytypeio/go-anytype-infrastructure-experiments/aclchanges/pb)
	@$(eval P_THREAD := Mdata/pb/protos/aclchanges.proto=github.com/anytypeio/go-anytype-infrastructure-experiments/thread/pb)
	@$(eval P_PLAINTEXT_CHANGES := Mdata/pb/protos/plaintextchanges.proto=github.com/anytypeio/go-anytype-infrastructure-experiments/testutils/testchanges/pb)

	$(eval PKGMAP := $$(P_TIMESTAMP),$$(P_STRUCT),$$(P_ACL_CHANGES))
	GOGO_NO_UNDERSCORE=1 GOGO_EXPORT_ONEOF_INTERFACE=1 protoc --gogofaster_out=$(PKGMAP):./aclchanges/pb aclchanges/pb/protos/*.*; mv aclchanges/pb/aclchanges/pb/protos/*.go aclchanges/pb; rm -rf aclchanges/pb/aclchanges
	$(eval PKGMAP := $$(P_TIMESTAMP),$$(P_STRUCT),$$(P_THREAD))
	GOGO_NO_UNDERSCORE=1 GOGO_EXPORT_ONEOF_INTERFACE=1 protoc --gogofaster_out=$(PKGMAP):./thread/pb thread/pb/protos/*.*; mv thread/pb/thread/pb/protos/*.go thread/pb; rm -rf thread/pb/thread
