package acls

import (
	"github.com/haproxytech/client-native/v5/models"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

//nolint:golint,stylecheck
type ACL_DESTINATION string

//nolint:golint,stylecheck
const (
	ACL_FRONTEND ACL_DESTINATION = "frontend"
	ACL_BACKEND  ACL_DESTINATION = "backend"
)

func PopulateBackend(client api.HAProxyClient, name string, acls models.Acls) {
	Populate(client, name, acls, ACL_BACKEND)
}

func PopulateFrontend(client api.HAProxyClient, name string, acls models.Acls) {
	Populate(client, name, acls, ACL_FRONTEND)
}

func Populate(client api.HAProxyClient, name string, acls models.Acls, resource ACL_DESTINATION) {
	currentAcls, errAcls := client.ACLsGet(string(resource), name)

	// There is a resource ...
	if errAcls == nil {
		diffAcls := acls.Diff(currentAcls)
		// ... with different acls from the resource.
		if len(diffAcls) != 0 {
			// ... we remove all the acls
			_ = client.ACLDeleteAll(string(resource), name)
			// ... and create all the new ones if any
			for _, acl := range acls {
				errCreate := client.ACLCreate(string(resource), name, acl)
				if errCreate != nil {
					utils.GetLogger().Err(errCreate)
					break
				}
			}
			// ... we reload because we created some acls.
			instance.Reload("%s '%s', acls updated: %+v", resource, name, diffAcls)
		}
	}
}
