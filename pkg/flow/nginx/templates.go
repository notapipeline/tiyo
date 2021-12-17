// Copyright 2021 The Tiyo authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package nginx

import "text/template"

// Template string for Nginx config files
var tplNginxConf = template.Must(template.New("").Parse(`
map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

{{ $upslen := len .Nginx.UpstreamPlain.Addresses }}
{{ $upsseclen := len .Nginx.UpstreamSecure.Addresses }}

{{ if gt $upslen 0 }}
upstream {{.Nginx.UpstreamPlain.Name}} {
{{- range $option := .Nginx.UpstreamPlain.Options}}
    {{$option}}
{{end -}}
{{- range $srv := .Nginx.UpstreamPlain.Addresses}}
    server {{$srv}};
{{- end}}
}
{{ end }}

{{ if gt $upsseclen 0 }}
upstream {{.Nginx.UpstreamSecure.Name}} {
{{- range $option := .Nginx.UpstreamSecure.Options}}
    {{$option}}
{{end -}}
{{- range $srv := .Nginx.UpstreamSecure.Addresses}}
    server {{$srv}};
{{- end}}
}
{{ end }}

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
