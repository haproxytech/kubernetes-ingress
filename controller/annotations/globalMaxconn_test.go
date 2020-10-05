package annotations

func (suite *AnnotationSuite) TestGlobalMaxconnUpdate() {
	test := "200"
	a := &globalMaxconn{}
	if suite.NoError(a.Parse(test)) {
		suite.Equal(RELOAD, a.Update(suite.client))
		result, _ := suite.client.GlobalWriteConfig("global", "maxconn")
		suite.Equal("maxconn 200", result)
	}
}

func (suite *AnnotationSuite) TestGlobalMaxconnFail() {
	test := "garbage"
	a := &globalMaxconn{}
	err := a.Parse(test)
	suite.T().Log(err)
	suite.Error(err)
}

func (suite *AnnotationSuite) TestGlobalMaxconnOverriddenOk() {
	suite.Run("empty", func() {
		err := (&globalMaxconn{}).Overridden("")
		suite.T().Log(err)
		suite.NoError(err)
	})
	suite.Run("data", func() {
		err := (&nbthread{}).Overridden("random-data")
		suite.T().Log(err)
		suite.NoError(err)
	})
}

func (suite *AnnotationSuite) TestGlobalMaxconnOverriddenFail() {
	err := (&globalMaxconn{}).Overridden("maxconn 2000")
	suite.T().Log(err)
	suite.Error(err)
}
