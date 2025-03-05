.PHONY: proto test deps mocks
export GOPRIVATE=github.com/anyproto

all:
	@set -e;
	@git config core.hooksPath .githooks;

PROTOC=protoc
PROTOC_GEN_GO=$(shell which protoc-gen-go)
PROTOC_GEN_DRPC=$(shell which protoc-gen-go-drpc)
PROTOC_GEN_VTPROTO=$(shell which protoc-gen-go-vtproto)

define generate_proto
	@echo "Generating Protobuf for directory: $(1)"
	$(PROTOC) \
		--go_out=. --plugin protoc-gen-go="$(PROTOC_GEN_GO)" \
		--go-vtproto_out=. --plugin protoc-gen-go-vtproto="$(PROTOC_GEN_VTPROTO)" \
		--go-vtproto_opt=features=marshal+unmarshal+size \
		--proto_path=$(1) $(wildcard $(1)/*.proto)
endef

define generate_drpc
	@echo "Generating Protobuf for directory: $(1) $(which protoc-gen-go)"
	$(PROTOC) \
		--go_out=. --plugin protoc-gen-go=$$(which protoc-gen-go) \
		--plugin protoc-gen-go-drpc=$(PROTOC_GEN_DRPC) \
		--go_opt=$(1) \
		--go-vtproto_out=:. --plugin protoc-gen-go-vtproto=$(PROTOC_GEN_VTPROTO) \
		--go-vtproto_opt=features=marshal+unmarshal+size \
		--go-drpc_out=protolib=github.com/planetscale/vtprotobuf/codec/drpc:. $(wildcard $(2)/*.proto)
endef

proto:
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

test:
	go test ./... --cover