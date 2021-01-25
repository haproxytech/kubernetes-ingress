package annotations_test

import (
	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

func (suite *AnnotationSuite) TestSyslogServerUpdate() {
	tests := []struct {
		input    store.StringW
		expected string
		daemon   bool
	}{
		{store.StringW{
			Value: "address:192.168.1.1, port:514, facility:local1",
		}, "log 192.168.1.1:514 local1", true},
		{store.StringW{
			Value: `  address:127.0.0.1, port:514, facility:local0
        address:192.168.1.1, port:514, facility:local1`,
		}, "log 127.0.0.1:514 local0\nlog 192.168.1.1:514 local1", true},
		{store.StringW{
			Value: "address:stdout, format: raw, facility:daemon",
		}, "log stdout format raw daemon", false},
	}
	for _, test := range tests {
		suite.T().Log(test.input)
		a := annotations.NewGlobalSyslogServers("", suite.client)
		if suite.NoError(a.Parse(test.input, true)) {
			suite.NoError(a.Update())
			suite.Equal(!test.daemon, a.Restart())
			daemon, _ := suite.client.GlobalConfigEnabled("global", "daemon")
			suite.Equal(test.daemon, daemon)
			result, _ := suite.client.GlobalWriteConfig("global", "log")
			suite.Equal(test.expected, result)
		}
	}
}

func (suite *AnnotationSuite) TestSyslogServerFail() {
	test := store.StringW{Value: "garbage"}
	a := annotations.NewGlobalSyslogServers("", suite.client)
	err := a.Parse(test, true)
	suite.T().Log(err)
	suite.Error(err)
}
