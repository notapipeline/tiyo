package kubernetes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"
)

type collectionTypes struct {
	sync.Mutex
	collections []string
}

var collections collectionTypes = collectionTypes{
	collections: make([]string, 0),
}

type Version3 struct {
	Description string                 `json:"description"`
	Type        string                 `json:"type"`
	Required    []string               `json:"required"`
	Properties  map[string]interface{} `json:"properties"`
}

type Schema struct {
	Spec struct {
		Group string `json:"group"`
		Scope string `json:"scope"`
		Names struct {
			Kind string `json:"kind"`
		} `json:"names"`
		Versions []struct {
			Name   string `json:"name"`
			Schema struct {
				V3 Version3 `json:"openAPIV3Schema"`
			} `json:"schema"`
		} `json:"versions"`
	} `json:"spec"`
}

var metadata map[string]map[string]string = map[string]map[string]string{
	"name": {
		"description": "The name of the resource representation",
		"type":        "string",
	},
	"namespace": {
		"description": "The namespace the resource will be created in",
		"type":        "string",
	},
	"annotations": {
		"description": "A map of annotations to be applied to the resource",
		"type":        "object",
	},
	"labels": {
		"description": "A map of labels to be applied to the resource",
		"type":        "object",
	},
}

// ApiResource contains the APIGroup and APIResource
type ApiResource struct {
	Group         string `json:"group"`
	Version       string `json:"version"`
	Collection    string `json:"collection"`
	Kind          string `json:"kind"`
	Package       string `json:"package"`
	Provider      string `json:"provider"`
	Native        bool   `json:"native"`
	IsCollection  bool   `json:"isCollection"`
	IsCollectable bool   `json:"isCollectable"`

	Resource metav1.APIResource
}

type apiResource struct {
	resource ApiResource
	err      error
}

type apiResourceList struct {
	resources []apiResource
	err       error
}

func stringKeys(from map[string][]ApiResource) []string {
	keys := make([]string, 0, len(from))
	for k := range from {
		keys = append(keys, k)
	}
	return keys
}

func (kube *Kubernetes) isNativeSchema(schema string) bool {
	var re = regexp.MustCompile(`^(\w+[/])?(\w+)$`)
	if re.MatchString(schema) {
		return true
	}
	return false
}

func (kube *Kubernetes) ApiDiscovery() {
	for {
		log.Info("(Re)Loading kubernetes api resources")
		go kube.apiDiscovery()
		select {
		case <-time.After(30 * time.Minute):
			continue
		}
	}
}

func (kube *Kubernetes) GetCRDApiDefinition(apigroup, apiversion string) (*Version3, error) {
	var (
		err error
		b   []byte
		s   Schema
		v3  Version3
		r   *rest.Request
		rsp rest.Result
	)

	kube.RestClient.Client.CloseIdleConnections()
	r = kube.RestClient.Get()
	r = r.AbsPath(fmt.Sprintf("/apis/apiextensions.k8s.io/v1/customresourcedefinitions/%s", apigroup))
	r = r.SetHeader("Connection", "close")

	log.Info(fmt.Sprintf("Fetching CRD definition for %s/%s", apigroup, apiversion))
	if rsp = r.Do(context.TODO()); rsp.Error() != nil {
		return nil, rsp.Error()
	}

	if b, err = rsp.Raw(); err == nil {
		json.Unmarshal(b, &s)
	}
	for _, v := range s.Spec.Versions {
		if v.Name != apiversion {
			continue
		}
		if v.Schema.V3.Properties["kind"] == nil {
			return nil, NewInvalidV3Schema(apigroup, apiversion)
		}
		mutateProperties(&v.Schema.V3, apigroup, apiversion, s.Spec.Names.Kind, s.Spec.Scope)
		v3 = v.Schema.V3
	}

	return &v3, err
}

func (kube *Kubernetes) GetCRDApiDefinitionAsJson(apigroup, apiversion string) (string, error) {
	var (
		b   []byte
		v3  *Version3
		err error
	)
	if v3, err = kube.GetCRDApiDefinition(apigroup, apiversion); err != nil {
		return "", err
	}
	b, _ = json.Marshal(*v3)

	return string(b), err
}

func (kube *Kubernetes) apiDiscovery() {
	d := kube.ClientSet.DiscoveryClient

	var (
		lists []*metav1.APIResourceList
		err   error
	)
	// TODO error handling, specifically for too many open connections
	for {
		if lists, err = d.ServerPreferredResources(); err == nil {
			break
		}
		log.Errorf("%s - %+v\n", err.Error(), err)
		if kerrors.IsTimeout(err) || kerrors.IsTooManyRequests(err) || errors.Is(err, context.DeadlineExceeded) {
			// retry after
			log.Debug("Retrying fetch for server api resources")
			var duration time.Duration = 5 * time.Millisecond
			// if rest.IsRetryableError(err) {}
			<-time.After(duration)
		}
	}

	apiResources := make(map[string][]ApiResource)
	var (
		resourceChan chan apiResourceList = make(chan apiResourceList)
		count        int                  = 0
	)
	for _, list := range lists {
		go kube.parseApiGroup(list, resourceChan)
	}

	for {
		select {
		case resources := <-resourceChan:
			if resources.err != nil {
				if !errors.Is(err, ContainsErrors) {
				}
			}
			for _, r := range resources.resources {
				if r.err != nil {
					continue
				}
				if !contains(r.resource.Provider, stringKeys(apiResources)) {
					apiResources[r.resource.Provider] = make([]ApiResource, 0)
				}
				apiResources[r.resource.Provider] = append(apiResources[r.resource.Provider], r.resource)
			}
			count++
		}
		if count == len(lists) {
			log.Info("Breaking parseApiGroup")
			break
		}
	}

	for k, v := range apiResources {
		for i, r := range v {
			if kube.isCollection(strings.ToLower(r.Kind)) {
				apiResources[k][i].IsCollection = true
			}
		}
	}
	kube.ApiResources.Lock()
	kube.ApiResources.Content = apiResources
	kube.ApiResources.Unlock()
}

func (kube *Kubernetes) parseApiGroup(list *metav1.APIResourceList, rc chan apiResourceList) {
	var resourceList apiResourceList = apiResourceList{
		resources: make([]apiResource, 0),
		err:       nil,
	}
	if len(list.APIResources) == 0 {
		resourceList.err = NoResourceListsFound
		rc <- resourceList
		return
	}

	gv, err := schema.ParseGroupVersion(list.GroupVersion)
	if err != nil {
		resourceList.err = err
		rc <- resourceList
		return
	}

	var (
		count        int              = 0
		resourceChan chan apiResource = make(chan apiResource)
	)
	for _, resource := range list.APIResources {
		go kube.parseApiResource(resource, gv, resourceChan)
	}
	for {
		select {
		case resource := <-resourceChan:
			if resource.err != nil && resourceList.err == nil {
				resourceList.err = ContainsErrors
			}
			resourceList.resources = append(resourceList.resources, resource)
			count++
		}

		if count == len(list.APIResources) {
			break
		}
	}
	rc <- resourceList
}

func (kube *Kubernetes) parseApiResource(resource metav1.APIResource, gv schema.GroupVersion, rc chan apiResource) {
	if len(resource.Verbs) == 0 {
		rc <- apiResource{
			resource: ApiResource{},
			err:      NoVerbs,
		}
		return
	}
	var (
		re *regexp.Regexp = regexp.MustCompile(
			`^((?P<collection>\w+)(\.(?P<package>\w+))?\.)?(?P<provider>[-\w]+)\.\w+|(?P<group>\w+[^/])?`)
		provider      string = "kubernetes"
		group         string = fmt.Sprintf("%s.%s", resource.Name, gv.Group)
		isNative      bool   = kube.isNativeSchema(gv.String())
		v3            *Version3
		collectionStr string
		packageStr    string
		collection    bool
		collectable   bool
		err           error
	)

	{
		match := re.FindStringSubmatch(gv.Group)
		result := make(map[string]string)
		for i, name := range re.SubexpNames() {
			if i != 0 && name != "" {
				result[name] = match[i]
			}
		}

		collectionStr = result["collection"]
		packageStr = result["package"]
		if result["provider"] != "" {
			provider = result["provider"]
		}
	}

	if !isNative {
		for {
			if v3, err = kube.GetCRDApiDefinition(group, gv.Version); err == nil {
				log.Infof("Found CRD schema for %s/%s", group, gv.Version)
				break
			}
			log.Errorf("%s (%s/%s) - %+v\n", err.Error(), group, gv.Version, err)
			if kerrors.IsTimeout(err) || kerrors.IsTooManyRequests(err) ||
				errors.Is(err, context.DeadlineExceeded) ||
				strings.Contains(err.Error(), "Too many open connections") ||
				strings.Contains(err.Error(), "Too many open files") {
				// retry after
				log.Infof("Retrying fetch for CRD %s/%s\n", group, gv.Version)
				// how do we get the actual wait time from the response error?
				<-time.After(5 * time.Millisecond)
				continue
			}
			break
		}
	}

	if v3 != nil {
		collectable = kube.isCollectable(v3, resource.Kind)
	}

	rc <- apiResource{
		resource: ApiResource{
			Group:         gv.Group,
			Version:       gv.Version,
			Collection:    collectionStr,
			Kind:          resource.Kind,
			Package:       packageStr,
			Provider:      provider,
			Native:        isNative,
			IsCollection:  collection,
			IsCollectable: collectable,
			Resource:      resource,
		},
		err: err,
	}
}

func (kube *Kubernetes) isCollectable(v3 *Version3, kd string) bool {
	if v3 == nil {
		return false
	}
	var flat map[string]interface{} = Flatten((*v3).Properties)
	for k := range flat {
		p := strings.Split(k, ".")
		i := strings.ToLower(p[len(p)-2])
		if i != "id" && strings.HasSuffix(i, "id") {
			i = strings.TrimSuffix(string(i[0])+i[1:], "id")
			if i == "instance" {
				fmt.Printf("%s requires %s\n", kd, i)
			}
			kube.addCollectionType(i)
			return true
		}
	}
	return false
}

func (kube *Kubernetes) isCollection(kind string) (si bool) {
	collections.Lock()
	for _, v := range collections.collections {
		if kind == v {
			si = true
		}
	}
	collections.Unlock()
	return
}

func (kube *Kubernetes) addCollectionType(c string) {
	collections.Lock()
	var added bool = false
	for _, co := range collections.collections {
		if c == co {
			added = true
			break
		}
	}

	if !added {
		collections.collections = append(collections.collections, c)
	}
	collections.Unlock()
}

func mutateProperties(v3 *Version3, apigroup, apiversion, kind, scope string) {
	// Inject known or static fields
	v3.Properties["kind"].(map[string]interface{})["enum"] = []string{kind}
	v3.Properties["apiVersion"].(map[string]interface{})["enum"] = []string{
		fmt.Sprintf("%s/%s", apigroup, apiversion),
	}
	if v3.Properties["metadata"] == nil {
		v3.Properties["metadata"] = make(map[string]interface{})
		// Don't send back namespace field for cluster scoped resources
		if scope != "Namespaced" {
			delete(
				v3.Properties["metadata"].(map[string]interface{})["properties"].(map[string]map[string]string),
				"namespace")
		}

	}
	v3.Properties["metadata"].(map[string]interface{})["properties"] = metadata

	// Status fields should not be populated by customer, we drop it from the form
	delete(v3.Properties, "status")
	delete(v3.Properties, "atProvider")
}

func Flatten(m map[string]interface{}) map[string]interface{} {
	o := map[string]interface{}{}
	for k, v := range m {
		switch child := v.(type) {
		case map[string]interface{}:
			nm := Flatten(child)
			for nk, nv := range nm {
				o[k+"."+nk] = nv
			}
		case []interface{}:
			for i := 0; i < len(child); i++ {
				o[k+"."+strconv.Itoa(i)] = child[i]
			}
		default:
			o[k] = v
		}
	}
	return o
}
