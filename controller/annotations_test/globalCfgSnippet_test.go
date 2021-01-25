package annotations_test

import (
	"github.com/haproxytech/kubernetes-ingress/controller/annotations"
	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

func (suite *AnnotationSuite) TestGlobalCfgSnippetUpdate() {
	tests := []struct {
		input    store.StringW
		expected string
	}{
		{store.StringW{Value: "ssl-default-bind-ciphers EECDH+AESGCM:EECDH+CHACHA20"},
			"###_config-snippet_### BEGIN\n  ssl-default-bind-ciphers EECDH+AESGCM:EECDH+CHACHA20\n  ###_config-snippet_### END"},
		{store.StringW{Value: `tune.ssl.default-dh-param 2048
      tune.bufsize 32768`,
		},
			"###_config-snippet_### BEGIN\n  tune.ssl.default-dh-param 2048\n  tune.bufsize 32768\n  ###_config-snippet_### END"},
	}
	for _, test := range tests {
		suite.T().Log(test.input)
		a := annotations.NewGlobalCfgSnippet("", suite.client)
		if suite.NoError(a.Parse(test.input, true)) {
			suite.NoError(a.Update())
			result, _ := suite.client.GlobalWriteConfig("global", "config-snippet")
			suite.Equal(test.expected, result)
		}
	}
}

func (suite *AnnotationSuite) TestGlobalCfgSnippetFail() {
	test := store.StringW{Value: "   "}
	a := annotations.NewGlobalCfgSnippet("", suite.client)
	err := a.Parse(test, true)
	suite.T().Log(err)
	suite.Error(err)
}
