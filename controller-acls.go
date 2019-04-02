package main

import (
	"github.com/haproxytech/models"
)

func (c *HAProxyController) addACL(acl models.ACL, frontends ...string) {
	for _, frontend := range frontends {
		err := c.NativeAPI.Configuration.CreateACL("frontend", frontend, &acl, c.ActiveTransaction, 0)
		LogErr(err)
	}
}

func (c *HAProxyController) removeACL(acl models.ACL, frontends ...string) {
	nativeAPI := c.NativeAPI
	for _, frontend := range frontends {
		aclsModel, err := nativeAPI.Configuration.GetACLs("frontend", frontend, c.ActiveTransaction)
		if err == nil {
			indexShift := int64(0)
			data := aclsModel.Data
			for _, d := range data {
				if acl.ACLName == d.ACLName {
					err = nativeAPI.Configuration.DeleteACL(*d.ID-indexShift, "frontend", frontend, c.ActiveTransaction, 0)
					LogErr(err)
					if err == nil {
						indexShift++
					}
				}
			}
		}
	}
}
