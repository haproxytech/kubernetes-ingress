package annotations

import (
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type Annotation interface {
	Parse(value string) error
	GetName() string
	Update() error
}

var logger = utils.GetLogger()

func HandleAnnotation(a Annotation, value string) {
	err := a.Parse(value)
	if err != nil {
		logger.Errorf("%s: %s", a.GetName(), err)
		return
	}
	err = a.Update()
	if err != nil {
		logger.Errorf("%s: %s", a.GetName(), err)
		return
	}
}
