package annotations

import (
	"os"
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
	transactionDir := "/tmp/haproxy"
	logger.Panic(os.MkdirAll(transactionDir, 0755))
	suite.client, err = api.Init(transactionDir, "../../fs/etc/haproxy/haproxy.cfg", "", "")
	suite.Nil(err)
	suite.NoError(suite.client.APIStartTransaction())
}

func TestSuite(t *testing.T) {
	suite.Run(t, new(AnnotationSuite))
}
