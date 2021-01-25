package annotations_test

import (
	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

func (suite *AnnotationSuite) TestDefaultOptionUpdate() {
	tests := []struct {
		optionName string
		input      store.StringW
		expected   string
	}{
		{"http-server-close", store.StringW{Value: "true"}, "option http-server-close"},
		{"http-keep-alive", store.StringW{Value: "true"}, "option http-keep-alive"},
		{"dontlognull", store.StringW{Value: "true"}, "option dontlognull"},
		{"logasap", store.StringW{Value: "true"}, "option logasap"},
		{"http-server-close", store.StringW{Value: "false"}, "no option http-server-close"},
	}
	for _, test := range tests {
		suite.T().Log(test.optionName + ": " + test.input.Value)
		a := annotations.NewDefaultOption(test.optionName, suite.client)
		if suite.NoError(a.Parse(test.input, true)) {
			suite.NoError(a.Update())
			result, _ := suite.client.GlobalWriteConfig("defaults", "option "+test.optionName)
			suite.Equal(test.expected, result)
		}
	}
}

func (suite *AnnotationSuite) TestDefaultOptionFail() {
	test := store.StringW{Value: "garbage"}
	a := annotations.NewDefaultOption("http-server-close", suite.client)
	err := a.Parse(test, true)
	suite.T().Log(err)
	suite.Error(err)
}
