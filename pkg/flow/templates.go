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

upstream {{.Nginx.UpstreamPlain.Name}} {
{{- range $option := .Nginx.UpstreamPlain.Options}}
    {{$option}}
{{end -}}
{{- range $srv := .Nginx.UpstreamPlain.Addresses}}
    server {{$srv}};
{{- end}}
}

upstream {{.Nginx.UpstreamSecure.Name}} {
{{- range $option := .Nginx.UpstreamSecure.Options}}
    {{$option}}
{{end -}}
{{- range $srv := .Nginx.UpstreamSecure.Addresses}}
    server {{$srv}};
{{- end}}
}

{{- range $listener := .Nginx.Listeners }}
server {
{{- if eq $listener.Protocol "http"}}
    listen {{$listener.Listen}};
{{- else if eq $listener.Protocol "https"}}
    listen {{$listener.Listen}} ssl;
{{- end}}
    server_name {{$listener.Hostname}};
{{- if eq $listener.Protocol "https"}}
    ssl                  on;
    ssl_certificate      /etc/ssl/{{$listener.Hostname}}/certificate.crt;
    ssl_certificate_key  /etc/ssl/{{$listener.Hostname}}/certificate.key;
{{end}}
{{- range $location := $listener.Locations}}
    location {{$location.Path}} {

{{- if $location.SecureUpstream }}
        proxy_pass  https://{{$location.UpstreamSecure}};
{{- else }}
        proxy_pass  http://{{$location.Upstream}};
{{- end }}
{{- if $location.SkipVerify }}
        proxy_ssl_verify off;
{{- end }}
        proxy_set_header  Host $host;
        proxy_set_header  X-Real-IP $remote_addr;
        proxy_set_header  X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header  X-Forwarded-Proto $scheme;
        proxy_http_version  1.1;
        proxy_set_header  Upgrade $http_upgrade;
        proxy_set_header  Connection 'upgrade';
    }
{{- end}}
{{- if $listener.Return }}
    return {{$listener.Return.Code}} {{$listener.Return.Address}};
{{end}}
}
{{end}}
`))
