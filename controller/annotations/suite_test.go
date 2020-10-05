package annotations

import (
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/haproxytech/kubernetes-ingress/controller/haproxy/api"
)

type AnnotationSuite struct {
	suite.Suite
	client api.HAProxyClient
}

func (suite *AnnotationSuite) SetupSuite() {
	var err error
	suite.client, err = api.Init("../../fs/etc/haproxy/haproxy.cfg", "", "")
	suite.Nil(err)
	suite.NoError(suite.client.APIStartTransaction())
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(AnnotationSuite))
}
