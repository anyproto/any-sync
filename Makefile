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

protos-go:
	@echo 'Generating protobuf packages (Go)...'
	$(eval P_TIMESTAMP := Mgoogle/protobuf/timestamp.proto=github.com/gogo/protobuf/types)
	$(eval P_STRUCT := Mgoogle/protobuf/struct.proto=github.com/gogo/protobuf/types)
	@$(eval P_ACL_CHANGES := Mdata/pb/protos/aclchanges.proto=github.com/anytypeio/go-anytype-infrastructure-experiments/data/pb)
	@$(eval P_PLAINTEXT_CHANGES := Mdata/pb/protos/plaintextchanges.proto=github.com/anytypeio/go-anytype-infrastructure-experiments/data/pb)

	$(eval PKGMAP := $$(P_TIMESTAMP),$$(P_STRUCT),$$(P_ACL_CHANGES),$$(P_PLAINTEXT_CHANGES))
	GOGO_NO_UNDERSCORE=1 GOGO_EXPORT_ONEOF_INTERFACE=1 protoc --gogofaster_out=$(PKGMAP):./data/pb data/pb/protos/*.*; mv data/pb/data/pb/protos/*.go data/pb; rm -rf data/pb/data
