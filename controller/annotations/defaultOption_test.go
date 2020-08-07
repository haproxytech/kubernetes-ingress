package annotations

func (suite *AnnotationSuite) TestDefaultOptionUpdate() {
	tests := []struct {
		optionName string
		input      string
		expected   string
	}{
		{"http-server-close", "true", "option http-server-close"},
		{"http-keep-alive", "true", "option http-keep-alive"},
		{"dontlognull", "true", "option dontlognull"},
		{"logasap", "true", "option logasap"},
		{"http-server-close", "false", "no option http-server-close"},
	}
	for _, test := range tests {
		suite.T().Log(test.optionName + ": " + test.input)
		a := &defaultOption{name: test.optionName}
		if suite.NoError(a.Parse(test.input)) {
			suite.Equal(RELOAD, a.Update(suite.client))
			result, _ := suite.client.GlobalWriteConfig("defaults", "option "+test.optionName)
			suite.Equal(test.expected, result)
		}
	}
}

func (suite *AnnotationSuite) TestDefaultOptionFail() {
	test := "garbage"
	a := &defaultOption{name: "http-server-close"}
	err := a.Parse(test)
	suite.T().Log(err)
	suite.Error(err)
}
