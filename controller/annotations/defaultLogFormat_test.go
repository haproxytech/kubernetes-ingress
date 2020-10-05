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

func (suite *AnnotationSuite) TestDefaultLogOverriddenOk() {
	suite.Run("empty", func() {
		err := (&globalMaxconn{}).Overridden("")
		suite.T().Log(err)
		suite.NoError(err)
	})
	suite.Run("data", func() {
		err := (&defaultLogFormat{}).Overridden("random-data")
		suite.T().Log(err)
		suite.NoError(err)
	})
}

func (suite *AnnotationSuite) TestDefaultLogOverriddenFail() {
	err := (&defaultLogFormat{}).Overridden(`log-format '%ci:%cp [%tr] %ft %b/%s %TR/%Tw/%Tc/%Tr/%Ta %ST %B %CC %CS %tsc %ac/%fc/%bc/%sc/%rc %sq/%bq %hr %hs "%HM %[var(txn.base)] %HV"'`)
	suite.T().Log(err)
	suite.Error(err)
}
