package annotations

func (suite *AnnotationSuite) TestGlobalHardStopAfterUpdate() {
	test := "200s"
	a := &globalHardStopAfter{}
	if suite.NoError(a.Parse(test)) {
		suite.Equal(RELOAD, a.Update(suite.client))
		result, _ := suite.client.GlobalWriteConfig("global", "hard-stop-after")
		suite.Equal("hard-stop-after 3m20s", result)
	}
}

func (suite *AnnotationSuite) TestGlobalHardStopAfterFail() {
	test := "garbage"
	a := &globalHardStopAfter{}
	err := a.Parse(test)
	suite.T().Log(err)
	suite.Error(err)
}

func (suite *AnnotationSuite) TestGlobalHardStopAfterOverriddenOk() {
	suite.Run("empty", func() {
		err := (&globalHardStopAfter{}).Overridden("")
		suite.T().Log(err)
		suite.NoError(err)
	})
	suite.Run("data", func() {
		err := (&nbthread{}).Overridden("random-data")
		suite.T().Log(err)
		suite.NoError(err)
	})
}

func (suite *AnnotationSuite) TestGlobalHardStopAfterOverriddenFail() {
	err := (&globalHardStopAfter{}).Overridden("hard-stop-after 2000s")
	suite.T().Log(err)
	suite.Error(err)
}
