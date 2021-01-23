module github.com/choclab-net/tiyo

go 1.15

replace github.com/coreos/go-systemd => github.com/coreos/go-systemd/v22 v22.1.0

require (
	github.com/Microsoft/go-winio v0.4.16 // indirect
	github.com/boltdb/bolt v1.3.1
	github.com/containerd/containerd v1.4.3 // indirect
	github.com/coreos/go-systemd v0.0.0-00010101000000-000000000000
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v20.10.1+incompatible
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/elazarl/go-bindata-assetfs v1.0.1
	github.com/gin-contrib/multitemplate v0.0.0-20200916052041-666a7309d230
	github.com/gin-contrib/static v0.0.0-20200916080430-d45d9a37d28e
	github.com/gin-gonic/gin v1.6.3
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/moby/term v0.0.0-20201216013528-df9cb8a40635 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/rjeczalik/notify v0.9.2
	github.com/sirupsen/logrus v1.7.0
	gotest.tools/v3 v3.0.3 // indirect
	k8s.io/api v0.20.0
	k8s.io/apimachinery v0.20.0
	k8s.io/client-go v0.20.0
)
