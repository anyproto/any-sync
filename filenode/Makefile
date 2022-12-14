.PHONY: proto build test
export GOPRIVATE=github.com/anytypeio

build:
	@$(eval FLAGS := $$(shell govvv -flags -pkg github.com/anytypeio/go-anytype-infrastructure-experiments/filenode))
	go build -v -o ../bin/file-node -ldflags "$(FLAGS)" github.com/anytypeio/go-anytype-infrastructure-experiments/filenode/cmd

test:
	go test ./... --cover