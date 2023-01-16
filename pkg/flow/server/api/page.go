package api

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/notapipeline/tiyo/pkg/kubernetes"
)

type sidebarEntry struct {
	Icon          string `json:"icon"`
	Schema        string `json:"schema"`
	Group         string `json:"group"`
	Version       string `json:"version"`
	Kind          string `json:"kind"`
	IsCollection  bool   `json:"isCollection"`  // Is this item a collection
	IsCollectable bool   `json:"isCollectable"` // can this item belong to a Collection
	IsNative      bool   `json:"isNative"`      // Is this a kubernetes builtin
}

func (api *API) waitForApiResourceContent() {
	for {
		var ready bool = false
		switch api.flow.Kubernetes.ApiResources.Content {
		case nil:
			// We have to wait for an API to return
			time.After(1 * time.Microsecond)
			continue
		default:
			ready = true
			break
		}
		if ready {
			break
		}
	}
}

func (api *API) GetSidebar(c *gin.Context) {
	var (
		sidebar   map[string]map[string][]sidebarEntry = make(map[string]map[string][]sidebarEntry)
		resources map[string][]kubernetes.ApiResource
	)
	api.waitForApiResourceContent()

	api.flow.Kubernetes.ApiResources.RLock()
	resources = api.flow.Kubernetes.ApiResources.Content
	api.flow.Kubernetes.ApiResources.RUnlock()

	for k, item := range resources {
		var (
			exists bool     = false
			files  []string = api.bfs.ListFilesRecursive("img/icons/" + k)
		)
		for sk := range sidebar {
			if sk == k {
				exists = true
			}
		}
		if !exists {
			sidebar[k] = make(map[string][]sidebarEntry)
		}

		for _, r := range item {
			var current sidebarEntry = sidebarEntry{
				Group:         r.Group,
				Version:       r.Version,
				Kind:          r.Kind,
				IsCollection:  r.IsCollection,
				IsCollectable: r.IsCollectable,
				IsNative:      r.Native,
				// This will be the default icon if no other icon is found.
				Icon: "/static/img/icons/kubernetes/miscellaneous/compositeresourcedefinition.svg",
			}
			exists = false
			var cgroup = strings.ToLower(r.Group)
			for sk := range sidebar[k] {
				if sk == cgroup {
					exists = true
				}
			}
			if !exists {
				sidebar[k][cgroup] = make([]sidebarEntry, 0)
			}

			var found []string = make([]string, 0)
			for _, f := range files {
				var (
					sf             = strings.ToLower(strings.ReplaceAll(f, "-", ""))
					parts []string = strings.Split(sf, "/")
					svg   string   = parts[len(parts)-1]
					kind           = strings.ToLower(r.Kind)
				)

				if kind+".svg" == svg {
					found = []string{f}
					break // exact match, hard break.
				}

				re := regexp.MustCompile(fmt.Sprintf(".*_%s_.*.svg$", kind))
				if re.MatchString(svg) {
					found = append(found, f)
				}
			}
			switch len(found) {
			case 0:
				if k == "kubernetes" {
					continue
				}
			case 1:
				current.Icon = "/static/" + found[0]
			default:
				var extension []string = strings.Split(r.Package, ",")
				var provider string = r.Package
				if len(extension) > 1 {
					provider = extension[0]
				}
				for _, f := range found {
					if strings.Contains(f, strings.ToLower(fmt.Sprintf("/%s/", provider))) ||
						strings.Contains(f, strings.ToLower(r.Package)) {
						current.Icon = "/static/" + f
						break
					}
				}
			}
			sidebar[k][cgroup] = append(sidebar[k][cgroup], current)
		}
	}
	c.JSON(200, sidebar)
}
