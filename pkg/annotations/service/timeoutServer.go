package service

import (
	"github.com/haproxytech/client-native/v5/models"

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
	a.backend.ServerTimeout = timeout
	return nil
}
