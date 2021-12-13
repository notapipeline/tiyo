module github.com/notapipeline/tiyo

go 1.15

replace github.com/coreos/go-systemd => github.com/coreos/go-systemd/v22 v22.1.0

require (
	github.com/boltdb/bolt v1.3.1
	github.com/containerd/containerd v1.5.8 // indirect
	github.com/coreos/go-systemd v0.0.0-20190321100706-95778dfbb74e
	github.com/crewjam/saml v0.4.6
	github.com/docker/docker v20.10.1+incompatible
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/elazarl/go-bindata-assetfs v1.0.1
	github.com/gin-contrib/multitemplate v0.0.0-20200916052041-666a7309d230
	github.com/gin-contrib/sessions v0.0.4
	github.com/gin-contrib/static v0.0.0-20200916080430-d45d9a37d28e
	github.com/gin-gonic/gin v1.7.7
	github.com/go-git/go-git/v5 v5.2.0
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/moby/term v0.0.0-20201216013528-df9cb8a40635 // indirect
	github.com/morikuni/aec v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.0.2 // indirect
	github.com/pquerna/otp v1.3.0
	github.com/rjeczalik/notify v0.9.2
	github.com/sirupsen/logrus v1.8.1
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
	k8s.io/api v0.20.6
	k8s.io/apimachinery v0.20.6
	k8s.io/client-go v0.20.6
)
