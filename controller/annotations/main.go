package annotations

import (
	"errors"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
	"github.com/haproxytech/kubernetes-ingress/controller/utils"
)

type Annotation interface {
	Parse(value store.StringW, forceParse bool) error
	GetName() string
	Update() error
}

var ErrEmptyStatus = errors.New("emptyST")
var logger = utils.GetLogger()

func HandleAnnotation(a Annotation, value store.StringW, forceParse bool) (updated bool) {
	err := a.Parse(value, forceParse)
	if err != nil {
		if !errors.Is(err, ErrEmptyStatus) {
			logger.Error(err)
		}
		return false
	}
	err = a.Update()
	if err != nil {
		logger.Error(err)
		return false
	}
	return true
}
