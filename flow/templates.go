// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package flow

import "text/template"

// Template string for creating docker containers
const dockerTemplate string = `FROM %s
USER root

RUN if getent passwd | grep -q 1000; then \
  if ! which userdel; then \
    deluser $(getent passwd | grep 1000 | awk -F: '{print $1}'); \
  else \
    userdel $(getent passwd | grep 1000 | awk -F: '{print $1}'); \
  fi ; \
fi

RUN if ! which useradd; then \
    adduser -S -s /bin/sh --uid 1000 -h /tiyo tiyo; \
  else \
    useradd -ms /bin/sh -u 1000 -d /tiyo tiyo; \
  fi

WORKDIR /tiyo
COPY tiyo /usr/bin/tiyo
RUN chmod 755 /usr/bin/tiyo
COPY config.json tiyo.json
RUN chmod 644 tiyo.json
USER tiyo
CMD ["/usr/bin/tiyo", "syphon"]`

// Template string for Nginx config files
var tplNginxConf = template.Must(template.New("").Parse(`
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
