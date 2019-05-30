package main

import (
	"github.com/haproxytech/models"
)

func (c *HAProxyController) addACL(acl models.ACL, frontends ...string) {
	if len(frontends) == 0 {
		frontends = []string{"http", "https"}
	}
	for _, frontend := range frontends {
		_, acls, err := c.NativeAPI.Configuration.GetACLs("frontend", frontend, c.ActiveTransaction)
		found := false
		if err == nil {
			for _, d := range acls {
				if acl.ACLName == d.ACLName {
					found = true
					break
				}
			}
		}
		if !found {
			err = c.NativeAPI.Configuration.CreateACL("frontend", frontend, &acl, c.ActiveTransaction, 0)
			LogErr(err)
		}
	}
}

func (c *HAProxyController) removeACL(acl models.ACL, frontends ...string) {
	nativeAPI := c.NativeAPI
	for _, frontend := range frontends {
		_, acls, err := nativeAPI.Configuration.GetACLs("frontend", frontend, c.ActiveTransaction)
		if err == nil {
			indexShift := int64(0)
			for _, d := range acls {
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
