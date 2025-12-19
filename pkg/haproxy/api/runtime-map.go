package api

import (
	"fmt"
	"strings"

	"github.com/haproxytech/client-native/v6/models"

	"github.com/haproxytech/kubernetes-ingress/pkg/metrics"
)

func (c *clientNative) SetMapContent(mapFile string, payload []string) error {
	var mapVer, mapPath string
	pmm := metrics.New()
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return err
	}
	mapVer, err = runtime.PrepareMap(mapFile)
	if err != nil {
		if strings.HasPrefix(err.Error(), "maps dir doesn't exists") {
			err = ErrMapNotFound
		}
		err = fmt.Errorf("error preparing map file: %w", err)
		return err
	}
	mapPath, err = runtime.GetMapsPath(mapFile)
	if err != nil {
		err = fmt.Errorf("error getting map path: %w", err)
		return err
	}
	for i := range payload {
		_, err = runtime.ExecuteRaw(fmt.Sprintf("add map @%s %s <<\n%s\n", mapVer, mapPath, payload[i]))
		pmm.UpdateRuntimeMetrics(metrics.ObjectMap, err)
		if err != nil {
			err = fmt.Errorf("error loading map payload: %w", err)
			return err
		}
	}
	err = runtime.CommitMap(mapVer, mapFile)
	if err != nil {
		err = fmt.Errorf("error committing map file: %w", err)
	}
	return err
}

func (c *clientNative) GetMap(mapFile string) (*models.Map, error) {
	runtime, err := c.nativeAPI.Runtime()
	if err != nil {
		return nil, err
	}
	return runtime.GetMap(mapFile)
}
