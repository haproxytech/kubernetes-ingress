package api

import (
	"github.com/haproxytech/models/v2"
)

func (c *clientNative) ExecuteRaw(command string) (result []string, err error) {
	return c.nativeAPI.Runtime.ExecuteRaw(command)
}

func (c *clientNative) SetServerAddr(backendName string, serverName string, ip string, port int) error {
	return c.nativeAPI.Runtime.SetServerAddr(backendName, serverName, ip, port)
}

func (c *clientNative) SetServerState(backendName string, serverName string, state string) error {
	return c.nativeAPI.Runtime.SetServerState(backendName, serverName, state)
}

func (c *clientNative) SetMapContent(mapFile string, payload string) error {
	err := c.nativeAPI.Runtime.ClearMap(mapFile, false)
	if err != nil {
		return err
	}
	return c.nativeAPI.Runtime.AddMapPayload(mapFile, payload)
}

func (c *clientNative) GetMap(mapFile string) (*models.Map, error) {
	return c.nativeAPI.Runtime.GetMap(mapFile)
}
