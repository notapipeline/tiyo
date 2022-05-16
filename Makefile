install:
	go install

.DEFAULT_GOAL := help
.PHONY: help clean

BINDATA=${GOPATH}/bin/go-bindata-assetfs
BUILD_VERSION?=unknown
APPNAME="tiyo"


help:  ## Display this help message and exit
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: clean bindata.go mod ## Build the binary
	@echo "Compiling ${APPNAME}..."
	@CGO_ENABLED=0 \
		go build -v --compiler gc --ldflags "-extldflags -static -s -w -X main.version=${BUILD_VERSION}" -o ${APPNAME} .
	@echo "+++ ${APPNAME} compiled"

clean:  ## Remove old binaries
	rm -f ${APPNAME} ${APPNAME}.exe pkg/server/assets.go

bindata.go: $(BINDATA)
	@echo "Creating bindata.go..."
	@go-bindata -o pkg/server/assets.go --pkg server assets/...
	@echo "+++ assets.go created"

mod: # run go mod in 1.17 compatibility mode only
	go mod tidy -compat=1.17

$(BINDATA):
	go get github.com/kevinburke/go-bindata/go-bindata

