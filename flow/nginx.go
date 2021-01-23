package flow

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/choclab-net/tiyo/config"
	"github.com/coreos/go-systemd/dbus"
	log "github.com/sirupsen/logrus"
	networkv1 "k8s.io/api/networking/v1"
)

type Upstream struct {
	Name      string
	Options   []string
	Addresses []string
}

type NginxReturn struct {
	Code    int
	Address string
}

type Location struct {
	Path     string
	Upstream string
}

type Listener struct {
	Listen    int
	Hostname  string
	Domain    string
	Protocol  string
	Locations []*Location
	Return    *NginxReturn
}

type Nginx struct {
	Upstream *Upstream
	Listener *Listener
	Config   *config.Config
}

type NginxServer struct {
	sync.Mutex
	Systemd *dbus.Conn
}

var nginxServer *NginxServer

func CreateNginxConfig(config *config.Config, name string, rules []networkv1.IngressRule, upstream *[]ServicePort) {
	log.Info("Creating NGINX config for ", name)
	var directory string = "/etc/nginx/default.d/"
	for _, rule := range rules {
		ups := Upstream{}
		ups.Name = name
		ups.Options = []string{"least_conn;"}
		addresses := make([]string, 0)
		for _, addr := range *upstream {
			log.Debug(fmt.Sprintf("Found service port %+v", addr))
			addresses = append(addresses, fmt.Sprintf("%s:%d", addr.Address, addr.Port))
		}
		ups.Addresses = addresses

		// basic first round listener
		http := Listener{
			Listen:   80,
			Hostname: ups.Name + "." + rule.Host,
			Domain:   config.DnsName,
			Protocol: "http",
		}
		for range rule.HTTP.Paths {
			location := Location{
				Path:     "/",
				Upstream: name,
			}
			http.Locations = append(http.Locations, &location)
		}

		nginx := Nginx{
			Upstream: &ups,
			Listener: &http,
		}

		log.Debug(fmt.Sprintf("Creating NGINX config file with upstream %+v and listener %+v", nginx.Upstream, nginx.Listener))
		file, err := os.Create(filepath.Join(directory, name+".conf"))
		if err != nil {
			log.Error("Failed to create config file - ", err)
			return
		}
		if err = TplNginxConf.Execute(file, struct {
			Nginx *Nginx
		}{
			Nginx: &nginx,
		}); err != nil {
			log.Error("Failed to create template file - ", err)
		}
		file.Close()
	}
	reloadNginx()
}

// Deletes a file from nginx config directory
//
func DeleteNginxConfig(name string) {
	var (
		re  *regexp.Regexp
		err error
	)
	if re, err = regexp.Compile("[.]*/?"); err != nil {
		return
	}

	// sanitize name to prevent directory traversal
	name = re.ReplaceAllString(name, "")
	if len(name) == 0 {
		return
	}

	var filename string = "/etc/nginx/sites.d/" + name + ".conf"
	info, err := os.Stat(filename)
	if os.IsNotExist(err) || info.IsDir() {
		return
	}
	os.Remove(filename)
	reloadNginx()
}

func reloadNginx() {
	var (
		err     error
		systemd *dbus.Conn
	)
	if nginxServer == nil {
		if systemd, err = dbus.NewSystemdConnection(); err != nil {
			log.Error("Failed to create dbus connection - ", err)
			return
		}
		nginxServer = &NginxServer{
			Systemd: systemd,
		}
	}

	nginxServer.Lock()
	defer nginxServer.Unlock()
	channel := make(chan string)
	_, err = nginxServer.Systemd.ReloadUnit("nginx.service", "replace", channel)
	if err != nil {
		log.Error("Failed to restart nginx service: ", err)
	}
	log.Info(<-channel)
}

func CreateSSLCertificates(hostname string) {}
