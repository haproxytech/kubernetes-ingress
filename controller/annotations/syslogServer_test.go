package annotations

func (suite *AnnotationSuite) TestSyslogServerUpdate() {
	tests := []struct {
		input    string
		expected string
		daemon   bool
		r        Result
	}{
		{"address:192.168.1.1, port:514, facility:local1", "log 192.168.1.1:514 local1", true, RELOAD},
		{`  address:127.0.0.1, port:514, facility:local0
        address:192.168.1.1, port:514, facility:local1`,
			"log 127.0.0.1:514 local0\nlog 192.168.1.1:514 local1",
			true,
			RELOAD,
		},
		{"address:stdout, format: raw, facility:daemon", "log stdout format raw daemon", false, RESTART},
	}
	for _, test := range tests {
		suite.T().Log(test.input)
		a := &syslogServers{}
		if suite.NoError(a.Parse(test.input)) {
			suite.Equal(test.r, a.Update(suite.client))
			daemon, _ := suite.client.GlobalConfigEnabled("global", "daemon")
			suite.Equal(test.daemon, daemon)
			result, _ := suite.client.GlobalWriteConfig("global", "log")
			suite.Equal(test.expected, result)
		}
	}
}

func (suite *AnnotationSuite) TestSyslogServerFail() {
	test := "garbage"
	a := &syslogServers{}
	err := a.Parse(test)
	suite.T().Log(err)
	suite.Error(err)
}

func (suite *AnnotationSuite) TestSyslogServerOverriddenOk() {
	suite.Run("empty", func() {
		err := (&syslogServers{}).Overridden("")
		suite.T().Log(err)
		suite.NoError(err)
	})
	suite.Run("data", func() {
		err := (&syslogServers{}).Overridden("random-data")
		suite.T().Log(err)
		suite.NoError(err)
	})
}

func (suite *AnnotationSuite) TestSyslogServerOverriddenFail() {
	err := (&syslogServers{}).Overridden("log stdout format raw daemon")
	suite.T().Log(err)
	suite.Error(err)
}
