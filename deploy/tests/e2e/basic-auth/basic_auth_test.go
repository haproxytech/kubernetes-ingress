package basicauth

import (
	"net/http"

	"github.com/haproxytech/kubernetes-ingress/deploy/tests/e2e"
)

func (suite *HTTPBasicAuthSuite) Test_BasicAuth() {
	suite.Run("Denied", func() {
		suite.NoError(suite.test.DeployYamlTemplate("config/deploy.yaml.tmpl", suite.test.GetNS(), suite.tmplData))
		suite.Require().Eventually(func() bool {
			res, cls, err := suite.client.Do()
			if res == nil {
				suite.T().Log(err)
				return false
			}
			defer cls()
			return res.StatusCode == http.StatusUnauthorized
		}, e2e.WaitDuration, e2e.TickDuration)
	})
	suite.Run("Allowed", func() {
		for _, user := range []string{"des", "md5", "sha-256", "sha-512"} {
			suite.Eventually(func() bool {
				suite.client.Req.SetBasicAuth(user, "password")
				res, cls, err := suite.client.Do()
				if err != nil {
					suite.FailNow(err.Error())
				}
				defer cls()
				return res.StatusCode == http.StatusOK
			}, e2e.WaitDuration, e2e.TickDuration)
		}
	})
}
