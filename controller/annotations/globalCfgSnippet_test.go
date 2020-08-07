package annotations

func (suite *AnnotationSuite) TestGlobalCfgSnippetUpdate() {
	tests := []struct {
		input    string
		expected string
	}{
		{"ssl-default-bind-ciphers EECDH+AESGCM:EECDH+CHACHA20",
			"###_config-snippet_### BEGIN\n  ssl-default-bind-ciphers EECDH+AESGCM:EECDH+CHACHA20\n  ###_config-snippet_### END"},
		{`tune.ssl.default-dh-param 2048
      tune.bufsize 32768`,
			"###_config-snippet_### BEGIN\n  tune.ssl.default-dh-param 2048\n  tune.bufsize 32768\n  ###_config-snippet_### END"},
	}
	for _, test := range tests {
		suite.T().Log(test.input)
		a := &globalCfgSnippet{}
		if suite.NoError(a.Parse(test.input)) {
			suite.Equal(RELOAD, a.Update(suite.client))
			result, _ := suite.client.GlobalWriteConfig("global", "config-snippet")
			suite.Equal(test.expected, result)
		}
	}
}

func (suite *AnnotationSuite) TestGlobalCfgSnippetFail() {
	test := "  "
	a := &globalCfgSnippet{}
	err := a.Parse(test)
	suite.T().Log(err)
	suite.Error(err)
}
