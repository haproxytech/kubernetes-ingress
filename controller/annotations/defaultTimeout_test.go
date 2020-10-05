package annotations

func (suite *AnnotationSuite) TestDefaultTimeoutUpdate() {
	tests := []struct {
		timeoutName string
		input       string
		expected    string
	}{
		{"http-request", "5s", "timeout http-request 5s"},
		{"http-keep-alive", "1s", "timeout http-keep-alive 1s"},
		{"connect", "5s", "timeout connect 5s"},
		{"queue", "5s", "timeout queue 5s"},
		{"tunnel", "1h", "timeout tunnel 1h"},
		{"client", "1m", "timeout client 1m"},
		{"client-fin", "5s", "timeout client-fin 5s"},
		{"server", "1m", "timeout server 1m"},
		{"server-fin", "5s", "timeout server-fin 5s"},
	}
	for _, test := range tests {
		suite.T().Log(test.timeoutName + ": " + test.input)
		a := &defaultTimeout{name: test.timeoutName}
		if suite.NoError(a.Parse(test.input)) {
			suite.Equal(RELOAD, a.Update(suite.client))
			result, _ := suite.client.GlobalWriteConfig("defaults", "timeout "+test.timeoutName)
			suite.Equal(test.expected, result)
		}
	}
}

func (suite *AnnotationSuite) TestDefaultTimeoutFail() {
	test := "garbage"
	a := &defaultTimeout{name: "http-request"}
	err := a.Parse(test)
	suite.T().Log(err)
	suite.Error(err)
}

func (suite *AnnotationSuite) TestDefaultTimeoutOverriddenOk() {
	for _, n := range []string{
		"http-request",
		"http-keep-alive",
		"connect",
		"queue",
		"tunnel",
		"client",
		"client-fin",
		"server",
		"server-fin",
	} {
		suite.Run("empty", func() {
			err := (&defaultTimeout{name: n}).Overridden("")
			suite.T().Log(err)
			suite.NoError(err)
		})
		suite.Run("data", func() {
			err := (&defaultTimeout{name: n}).Overridden("random-data")
			suite.T().Log(err)
			suite.NoError(err)
		})
	}
}

func (suite *AnnotationSuite) TestDefaultTimeoutOverriddenFail() {
	for n, cs := range map[string]string{
		"http-request":    "timeout http-request 5s",
		"http-keep-alive": "timeout http-keep-alive 1s",
		"connect":         "timeout connect 5s",
		"queue":           "timeout queue 5s",
		"tunnel":          "timeout tunnel 1h",
		"client":          "timeout client 1m",
		"client-fin":      "timeout client-fin 5s",
		"server":          "timeout server 1m",
		"server-fin":      "timeout server-fin 5s",
	} {
		err := (&defaultTimeout{name: n}).Overridden(cs)
		suite.T().Log(err)
		suite.Error(err)
	}
}
