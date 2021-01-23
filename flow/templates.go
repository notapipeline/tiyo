package flow

import "text/template"

// Arguments: [org/]container, version, pipeline
const dockerTemplate string = `FROM %s
USER root
RUN useradd -ms /bin/bash -d /tiyo tiyo

WORKDIR /tiyo
COPY tiyo /usr/bin/tiyo
COPY config.json .
RUN chmod +x /usr/bin/tiyo
USER tiyo
CMD ["/usr/bin/tiyo", "syphon"]`

var TplNginxConf = template.Must(template.New("").Parse(`
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

upstream {{.Nginx.Upstream.Name}} {
{{- range $option := .Nginx.Upstream.Options}}
    {{$option}}
{{end -}}
{{- range $srv := .Nginx.Upstream.Addresses}}
    server {{$srv}};
{{- end}}
}

server {
{{- if eq .Nginx.Listener.Protocol "http"}}
    listen {{.Nginx.Listener.Listen}};
{{- else if eq .Nginx.Listener.Protocol "https"}}
    listen {{.Nginx.Listener.Listen}} ssl;
{{- end}}
    server_name {{.Nginx.Listener.Hostname}};
{{- if eq .Nginx.Listener.Protocol "https" -}}
    ssl                  on;
    ssl_certificate      /etc/ssl/{{.Nginx.Listener.Domain}}/{{.Nginx.Upstream.Name}}.crt;
    ssl_certificate_key  /etc/ssl/{{.Nginx.Listener.Domain}}/{{.Nginx.Upstream.Name}}.key;
{{end}}
{{- range $location := .Nginx.Listener.Locations}}
    location {{$location.Path}} {
        proxy_pass  http://{{$location.Upstream}};
        proxy_set_header  Host $host;
        proxy_set_header  X-Real-IP $remote_addr;
        proxy_set_header  X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header  X-Forwarded-Proto $scheme;
        proxy_http_version  1.1;
        proxy_set_header  Upgrade $http_upgrade;
        proxy_set_header  Connection 'upgrade';
    }
{{- end}}
}
`))
