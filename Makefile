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

proto:
	$(MAKE) -C common proto
	$(MAKE) -C consensus proto

build:
	$(MAKE) -C node build
	$(MAKE) -C consensus build

test:
	$(MAKE) -C node test
	$(MAKE) -C consensus test
	$(MAKE) -C common test