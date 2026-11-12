package service

import (
	"fmt"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/annotations/common"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type TimeoutCheck struct {
	backend *models.Backend
	name    string
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
	if !common.ValidRule(common.BackendRule, fmt.Sprintf("timeout check %d", *timeout)) {
		return fmt.Errorf("%s: HAProxy rejects the value '%s'", a.GetName(), input)
	}
	a.backend.CheckTimeout = timeout
	return nil
}
