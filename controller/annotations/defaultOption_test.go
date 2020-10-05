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

func (suite *AnnotationSuite) TestDefaultOptionOverriddenOk() {
	for _, n := range []string{
		"http-server-close",
		"http-keep-alive",
		"dontlognull",
		"logasap",
	} {
		suite.Run("empty", func() {
			err := (&defaultOption{name: n}).Overridden("")
			suite.T().Log(err)
			suite.NoError(err)
		})
		suite.Run("data", func() {
			err := (&defaultOption{name: n}).Overridden("random-data")
			suite.T().Log(err)
			suite.NoError(err)
		})
	}
}

func (suite *AnnotationSuite) TestDefaultOptionOverriddenFail() {
	for n, cs := range map[string]string{
		"http-server-close": "option http-server-close",
		"http-keep-alive":   "option http-keep-alive",
		"dontlognull":       "option dontlognull",
		"logasap":           "option logasap",
	} {
		err := (&defaultOption{name: n}).Overridden(cs)
		suite.T().Log(err)
		suite.Error(err)
	}
}
