package controller

import (
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

func (c *HAProxyController) handleMaxconn(maxconn *int64, frontends ...string) error {
	for _, frontendName := range frontends {
		if frontend, err := c.frontendGet(frontendName); err == nil {
			frontend.Maxconn = maxconn
			err1 := c.frontendEdit(frontend)
			utils.LogErr(err1)
		} else {
			return err
		}
	}
	return nil
}
