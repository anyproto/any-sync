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
	$(MAKE) -C client proto

build:
	$(MAKE) -C node build
	$(MAKE) -C filenode build
	$(MAKE) -C consensus build
	$(MAKE) -C client build

test:
	$(MAKE) -C node test
	$(MAKE) -C filenode test
	$(MAKE) -C consensus test
	$(MAKE) -C common test
	$(MAKE) -C client test