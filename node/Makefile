.PHONY: proto build test
export GOPRIVATE=github.com/anytypeio

build:
	@$(eval FLAGS := $$(shell govvv -flags -pkg github.com/anytypeio/go-anytype-infrastructure-experiments/node))
	go build -v -o ../bin/anytype-node -ldflags "$(FLAGS)" github.com/anytypeio/go-anytype-infrastructure-experiments/node/cmd

test:
	go test ./... --cover