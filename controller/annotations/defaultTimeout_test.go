package annotations

import (
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

func (suite *AnnotationSuite) TestDefaultTimeoutUpdate() {
	tests := []struct {
		timeoutName string
		input       store.StringW
		expected    string
	}{
		{"http-request", store.StringW{Value: "5s"}, "timeout http-request 5s"},
		{"http-keep-alive", store.StringW{Value: "1s"}, "timeout http-keep-alive 1s"},
		{"connect", store.StringW{Value: "5s"}, "timeout connect 5s"},
		{"queue", store.StringW{Value: "5s"}, "timeout queue 5s"},
		{"tunnel", store.StringW{Value: "1h"}, "timeout tunnel 1h"},
		{"client", store.StringW{Value: "1m"}, "timeout client 1m"},
		{"client-fin", store.StringW{Value: "5s"}, "timeout client-fin 5s"},
		{"server", store.StringW{Value: "1m"}, "timeout server 1m"},
		{"server-fin", store.StringW{Value: "5s"}, "timeout server-fin 5s"},
	}
	for _, test := range tests {
		suite.T().Log(test.timeoutName + ": " + test.input.Value)
		a := NewDefaultTimeout("timeout-"+test.timeoutName, suite.client)
		if suite.NoError(a.Parse(test.input, true)) {
			suite.NoError(a.Update())
			result, _ := suite.client.GlobalWriteConfig("defaults", "timeout "+test.timeoutName)
			suite.Equal(test.expected, result)
		}
	}
}

func (suite *AnnotationSuite) TestDefaultTimeoutFail() {
	test := store.StringW{Value: "garbage"}
	a := NewDefaultTimeout("timeout-http-request", suite.client)
	err := a.Parse(test, true)
	suite.T().Log(err)
	suite.Error(err)
}
