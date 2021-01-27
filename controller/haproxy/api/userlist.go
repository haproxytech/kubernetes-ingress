package api

import (
	parser "github.com/haproxytech/config-parser/v3"
	"github.com/haproxytech/config-parser/v3/types"
)

func (c *clientNative) UserListDeleteByGroup(group string) (err error) {
	c.activeTransactionHasChanges = true

	var p *parser.Parser
	if p, err = c.nativeAPI.Configuration.GetParser(c.activeTransaction); err != nil {
		return
	}

	return p.SectionsDelete(parser.UserList, group)
}

func (c *clientNative) UserListCreateByGroup(group string, userPasswordMap map[string][]byte) (err error) {
	c.activeTransactionHasChanges = true

	var p *parser.Parser
	if p, err = c.nativeAPI.Configuration.GetParser(c.activeTransaction); err != nil {
		return
	}

	if err = p.SectionsCreate(parser.UserList, group); err != nil {
		return
	}

	names := make([]string, 0, len(userPasswordMap))
	for name, password := range userPasswordMap {
		user := &types.User{
			Name:     name,
			Password: string(password),
			Groups:   []string{"authenticated-users"},
		}
		if err = p.Insert(parser.UserList, group, "user", user); err != nil {
			return
		}
		names = append(names, user.Name)
	}
	err = p.Insert(parser.UserList, group, "group", types.Group{
		Name:  "authenticated-users",
		Users: names,
	})

	return
}
