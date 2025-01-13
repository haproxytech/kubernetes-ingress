package acls

import (
	"github.com/haproxytech/client-native/v6/models"
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

func Populate(client api.ACL, name string, rules models.Acls, resource ACL_DESTINATION) {
	currentAcls, errAcls := client.ACLsGet(string(resource), name)

	// There is a resource ...
	if errAcls == nil {
		diffAcls := rules.Diff(currentAcls)
		// ... with different acls from the resource.
		if len(diffAcls) != 0 {
			err := client.ACLsReplace(string(resource), name, rules)
			if err != nil {
				utils.GetLogger().Err(err)
				return
			}
			// ... we reload because we created some acls.
			instance.Reload("%s '%s', acls updated: %+v", resource, name, utils.JSONDiff(diffAcls))
		}
	}
}
