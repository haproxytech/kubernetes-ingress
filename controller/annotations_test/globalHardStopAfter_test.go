package annotations_test

import (
	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

func (suite *AnnotationSuite) TestHardStopAfterUpdate() {
	test := store.StringW{Value: "200s"}
	a := annotations.NewGlobalHardStopAfter("", suite.client)
	if suite.NoError(a.Parse(test, true)) {
		suite.NoError(a.Update())
		result, _ := suite.client.GlobalWriteConfig("global", "hard-stop-after")
		suite.Equal("hard-stop-after 3m20s", result)
	}
}

func (suite *AnnotationSuite) TestHardStopAfterFail() {
	test := store.StringW{Value: "garbage"}
	a := annotations.NewGlobalHardStopAfter("", suite.client)
	err := a.Parse(test, true)
	suite.T().Log(err)
	suite.Error(err)
}
