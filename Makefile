build:
	go-bindata-assetfs -o pkg/server/assets.go -pkg server assets/...
	CGO_ENABLED=0 go build .

install:
	go install

