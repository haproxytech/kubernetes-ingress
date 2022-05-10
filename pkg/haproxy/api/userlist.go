package api

import (
	parser "github.com/haproxytech/config-parser/v4"
	"github.com/haproxytech/config-parser/v4/types"
)

func (c *clientNative) UserListExistsByGroup(group string) (exist bool, err error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return false, err
	}
	c.activeTransactionHasChanges = true

	var p parser.Parser
	var sections []string
	if p, err = configuration.GetParser(c.activeTransaction); err != nil {
		return exist, err
	}
	sections, err = p.SectionsGet(parser.UserList)
	for _, section := range sections {
		if section == group {
			exist = true
			break
		}
	}
	return exist, err
}

func (c *clientNative) UserListDeleteAll() (err error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true

	var p parser.Parser
	if p, err = configuration.GetParser(c.activeTransaction); err != nil {
		return err
	}

	var sections []string
	sections, err = p.SectionsGet(parser.UserList)
	if err != nil {
		return err
	}
	for _, section := range sections {
		err = p.SectionsDelete(parser.UserList, section)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *clientNative) UserListCreateByGroup(group string, userPasswordMap map[string][]byte) (err error) {
	configuration, err := c.nativeAPI.Configuration()
	if err != nil {
		return err
	}
	c.activeTransactionHasChanges = true

	var p parser.Parser
	if p, err = configuration.GetParser(c.activeTransaction); err != nil {
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
