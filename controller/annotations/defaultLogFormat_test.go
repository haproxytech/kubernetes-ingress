package annotations

func (suite *AnnotationSuite) TestDefaultLogFormatUpdate() {
	test := "%ci:%cp [%tr] %ft %b/%s \"test\""

	suite.T().Log(test)
	a := &defaultLogFormat{}
	if suite.NoError(a.Parse(test)) {
		suite.Equal(RELOAD, a.Update(suite.client))
		result, _ := suite.client.GlobalWriteConfig("defaults", "log-format")
		suite.Equal("log-format '%ci:%cp [%tr] %ft %b/%s \"test\"'", result)
	}
}

func (suite *AnnotationSuite) TestDefaultLogFormatFail() {
	test := "  "
	a := &defaultLogFormat{}
	err := a.Parse(test)
	suite.T().Log(err)
	suite.Error(err)
}
