package annotations_test

import (
	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

func (suite *AnnotationSuite) TestDefaultLogFormatUpdate() {
	test := store.StringW{Value: "%ci:%cp [%tr] %ft %b/%s \"test\""}

	suite.T().Log(test)
	a := annotations.NewDefaultLogFormat("", suite.client)
	if suite.NoError(a.Parse(test, true)) {
		suite.NoError(a.Update())
		result, _ := suite.client.GlobalWriteConfig("defaults", "log-format")
		suite.Equal("log-format '%ci:%cp [%tr] %ft %b/%s \"test\"'", result)
	}
}

func (suite *AnnotationSuite) TestDefaultLogFormatFail() {
	test := store.StringW{Value: "  "}
	a := annotations.NewDefaultLogFormat("", suite.client)
	err := a.Parse(test, true)
	suite.T().Log(err)
	suite.Error(err)
}
