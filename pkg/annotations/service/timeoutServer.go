package service

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type TimeoutServer struct {
	backend *models.Backend
	name    string
}

func NewTimeoutServer(n string, b *models.Backend) *TimeoutServer {
	return &TimeoutServer{name: n, backend: b}
}

func (a *TimeoutServer) GetName() string {
	return a.name
}

func (a *TimeoutServer) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		a.backend.ServerTimeout = nil
		return nil
	}
	timeout, err := utils.ParseTime(input)
	if err != nil {
		return err
	}
	if !common.ValidRule(common.BackendRule, fmt.Sprintf("timeout server %d", *timeout)) {
		return fmt.Errorf("%s: HAProxy rejects the value '%s'", a.GetName(), input)
	}
	a.backend.ServerTimeout = timeout
	return nil
}
