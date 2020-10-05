package annotations

func (suite *AnnotationSuite) TestNbthreadUpdate() {
	test := "1"
	a := &nbthread{}
	if suite.NoError(a.Parse(test)) {
		suite.Equal(RELOAD, a.Update(suite.client))
		result, _ := suite.client.GlobalWriteConfig("global", "nbthread")
		suite.Equal("nbthread 1", result)
	}
}

func (suite *AnnotationSuite) TestNbthreadFail() {
	test := "garbage"
	a := &nbthread{}
	err := a.Parse(test)
	suite.T().Log(err)
	suite.Error(err)
}

func (suite *AnnotationSuite) TestNbthreadrOverriddenOk() {
	suite.Run("empty", func() {
		err := (&nbthread{}).Overridden("")
		suite.T().Log(err)
		suite.NoError(err)
	})
	suite.Run("data", func() {
		err := (&nbthread{}).Overridden("random-data")
		suite.T().Log(err)
		suite.NoError(err)
	})
}

func (suite *AnnotationSuite) TestNbthreadOverriddenFail() {
	err := (&nbthread{}).Overridden("nbthread 4")
	suite.T().Log(err)
	suite.Error(err)
}
