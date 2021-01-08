package annotations

import (
	"runtime"
	"strconv"

	"github.com/haproxytech/config-parser/v3/types"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type GlobalNbthread struct {
	name   string
	data   *types.Int64C
	client api.HAProxyClient
}

func NewGlobalNbthread(n string, c api.HAProxyClient) *GlobalNbthread {
	return &GlobalNbthread{name: n, client: c}
}

func (a *GlobalNbthread) GetName() string {
	return a.name
}

func (a *GlobalNbthread) Parse(input store.StringW, forceParse bool) error {
	if input.Status == store.EMPTY && !forceParse {
		return ErrEmptyStatus
	}
	if input.Status == store.DELETED {
		return nil
	}
	v, err := strconv.Atoi(input.Value)
	if err != nil {
		return err
	}
	maxProcs := runtime.GOMAXPROCS(0)
	if v > maxProcs {
		v = maxProcs
	}
	a.data = &types.Int64C{Value: int64(v)}
	return nil
}

func (a *GlobalNbthread) Update() error {
	if a.data == nil {
		logger.Infof("Removing nbThread option")
		return a.client.Nbthread(nil)
	}
	logger.Infof("Setting nbThread to: %d", a.data.Value)
	return a.client.Nbthread(a.data)
}
