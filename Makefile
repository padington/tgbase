.PHONY: proto

PROTOC ?= protoc
PROTO_DIR := proto
PROTOC_GEN_GO := $(shell go env GOPATH)/bin/protoc-gen-go

proto:
	@command -v $(PROTOC_GEN_GO) >/dev/null || { echo "protoc-gen-go not found at $(PROTOC_GEN_GO); run: go install google.golang.org/protobuf/cmd/protoc-gen-go@latest"; exit 1; }
	PATH="$$(go env GOPATH)/bin:$$PATH" $(PROTOC) \
		-I=$(PROTO_DIR) \
		--go_out=. \
		--go_opt=module=github.com/padington/tgbase \
		$(PROTO_DIR)/products.proto \
		$(PROTO_DIR)/state.proto \
		$(PROTO_DIR)/settings.proto \
		$(PROTO_DIR)/i18n.proto
	@echo "Generated *.pb.go committed under internal/<pkg>/pb/"
