build:
	go-bindata-assetfs -o server/assets.go -pkg server server/assets/...
	CGO_ENABLED=0 go build .

install:
	go install

