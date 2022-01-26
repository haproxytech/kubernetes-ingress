package service

import (
	"github.com/haproxytech/client-native/v2/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type TimeoutCheck struct {
	name    string
	backend *models.Backend
}

func NewTimeoutCheck(n string, b *models.Backend) *TimeoutCheck {
	return &TimeoutCheck{name: n, backend: b}
}

func (a *TimeoutCheck) GetName() string {
	return a.name
}

func (a *TimeoutCheck) Process(k store.K8s, annotations ...map[string]string) error {
	input := common.GetValue(a.GetName(), annotations...)
	if input == "" {
		a.backend.CheckTimeout = nil
		return nil
	}
	timeout, err := utils.ParseTime(input)
	if err != nil {
		return err
	}
	a.backend.CheckTimeout = timeout
	return nil
}
