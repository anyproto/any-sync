.PHONY: proto test deps mocks
export GOPRIVATE=github.com/anyproto
ROOT:=${PWD}

comma:=,

UNAME_S := $(shell uname -s)
UNAME_M := $(shell uname -m)

ifeq ($(UNAME_S), Darwin)
    PROTOC_OS = osx
else
    PROTOC_OS = linux
endif

ifeq ($(UNAME_M), x86_64)
    PROTOC_ARCH = x86_64
else
    PROTOC_ARCH = aarch_64
endif

PROTOC_ZIP = protoc-$(PROTOC_VERSION)-$(PROTOC_OS)-$(PROTOC_ARCH).zip

export DEPS=${ROOT}/deps

PROTOC_VERSION = 29.3

PROTOC_URL = https://github.com/protocolbuffers/protobuf/releases/download/v$(PROTOC_VERSION)/$(PROTOC_ZIP)

all:
	@set -e;
	@git config core.hooksPath .githooks;

proto: proto-execute

PROTOC = $(DEPS)/protoc
PROTOC_GEN_GO = $(DEPS)/protoc-gen-go
PROTOC_GEN_DRPC = $(DEPS)/protoc-gen-go-drpc
PROTOC_GEN_VTPROTO = $(DEPS)/protoc-gen-go-vtproto

define generate_proto
	@echo "Generating Protobuf for directory: $(1)"
	$(PROTOC) \
		--go_out=. --plugin protoc-gen-go="$(PROTOC_GEN_GO)" \
		--go-vtproto_out=. --plugin protoc-gen-go-vtproto="$(PROTOC_GEN_VTPROTO)" \
		--go-vtproto_opt=features=marshal+unmarshal+size+pool+clone \
		--proto_path=$(1) $(wildcard $(1)/*.proto)
endef

define generate_drpc
	@echo "Generating Protobuf for directory: $(1)"
	$(PROTOC) \
		--go_out=. --plugin protoc-gen-go="$(PROTOC_GEN_GO)" \
		--plugin protoc-gen-go-drpc="$(PROTOC_GEN_DRPC)" \
		--go_opt=$(1) \
		--go-vtproto_out=:. --plugin protoc-gen-go-vtproto="$(PROTOC_GEN_VTPROTO)" \
		--go-vtproto_opt=features=marshal+unmarshal+size \
		--go-drpc_out=protolib=github.com/planetscale/vtprotobuf/codec/drpc:. $(wildcard $(2)/*.proto)
endef

proto-execute:
	@echo 'Generating protobuf packages (Go)...'
	@$(eval P_ACL_RECORDS_PATH_PB := commonspace/object/acl/aclrecordproto)
	@$(eval P_ACL_RECORDS_PATH_PB := commonspace/object/acl/aclrecordproto)
	@$(eval P_TREE_CHANGES_PATH_PB := commonspace/object/tree/treechangeproto)
	@$(eval P_CRYPTO_PATH_PB := util/crypto/cryptoproto)
	@$(eval P_ACL_RECORDS := M$(P_ACL_RECORDS_PATH_PB)/protos/aclrecord.proto=github.com/anyproto/any-sync/$(P_ACL_RECORDS_PATH_PB))
	@$(eval P_TREE_CHANGES := M$(P_TREE_CHANGES_PATH_PB)/protos/treechange.proto=github.com/anyproto/any-sync/$(P_TREE_CHANGES_PATH_PB))

	$(call generate_proto,$(P_ACL_RECORDS_PATH_PB)/protos)
	$(call generate_proto,$(P_TREE_CHANGES_PATH_PB)/protos)
	$(call generate_proto,$(P_CRYPTO_PATH_PB)/protos)
	$(eval PKGMAP := $$(P_TREE_CHANGES)$(comma)$$(P_ACL_RECORDS))
	$(call generate_drpc,$(PKGMAP),commonspace/spacesyncproto/protos)
	$(call generate_drpc,$(PKGMAP),commonfile/fileproto/protos)
	$(call generate_drpc,$(PKGMAP),net/streampool/testservice/protos)

	$(call generate_drpc,,net/secureservice/handshake/handshakeproto/protos)
	$(call generate_drpc,,net/rpc/limiter/limiterproto/protos)
	$(call generate_drpc,$(PKGMAP),coordinator/coordinatorproto/protos)
	$(call generate_drpc,,consensus/consensusproto/protos)
	$(call generate_drpc,,identityrepo/identityrepoproto/protos)
	$(call generate_drpc,,nameservice/nameserviceproto/protos)
	$(call generate_drpc,,paymentservice/paymentserviceproto/protos)


mocks:
	echo 'Generating mocks...'
	go build -o deps go.uber.org/mock/mockgen
	PATH=$(CURDIR)/deps:$(PATH) go generate ./...

deps:
	go mod download
	@echo "Downloading protoc $(PROTOC_VERSION)..."
	curl -OL $(PROTOC_URL)
	mkdir -p $(DEPS)
	unzip -o $(PROTOC_ZIP) -d $(ROOT)
	mv bin/protoc $(DEPS)
	rm $(PROTOC_ZIP)
	rm -rf include
	rm -rf readme.txt
	rm -rf bin
	@echo "protoc installed in $(DEPS)/bin"

	GOBIN=$(DEPS) go install storj.io/drpc/cmd/protoc-gen-go-drpc@latest
	GOBIN=$(DEPS) go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	GOBIN=$(DEPS) go install github.com/planetscale/vtprotobuf/cmd/protoc-gen-go-vtproto@latest

test:
	go test ./... --cover