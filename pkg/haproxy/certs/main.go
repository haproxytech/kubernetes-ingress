package certs

import (
	"errors"
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type certs struct {
	frontend map[string]*cert
	backend  map[string]*cert
	ca       map[string]*cert
}

type Certificates interface {
	// Add takes a secret and its type and creats or updates the corresponding certificate
	AddSecret(secret *store.Secret, secretType SecretType) (certPath string, err error)
	// FrontCertsInuse returns true if a frontend certificate is configured.
	FrontCertsInUse() bool
	// Updated returns true if there is any updadted/created certificate
	CertsUpdated() bool
	// Refresh removes unused certs from HAProxyCertDir
	RefreshCerts()
	// Clean cleans certificates state
	CleanCerts()
}

type cert struct {
	name    string
	path    string
	inUse   bool
	updated bool
}

type SecretType int

type Env struct {
	MainDir     string
	FrontendDir string
	BackendDir  string
	CaDir       string
}

var env Env

// module logger
var logger = utils.GetLogger()

//nolint:golint,stylecheck
const (
	NONE_CERT SecretType = iota
	FT_CERT
	FT_DEFAULT_CERT
	BD_CERT
	CA_CERT
)

type SecretCtx struct {
	Namespace  string
	Name       string
	SecretType SecretType
}

func New(envParam Env) (Certificates, error) { //nolint:ireturn
	env = envParam
	if env.FrontendDir == "" {
		return nil, errors.New("empty name for Frontend Cert Directory")
	}
	if env.BackendDir == "" {
		return nil, errors.New("empty name for Backend Cert Directory")
	}
	if env.CaDir == "" {
		return nil, errors.New("empty name for CA Cert Directory")
	}
	return &certs{
		frontend: make(map[string]*cert),
		backend:  make(map[string]*cert),
		ca:       make(map[string]*cert),
	}, nil
}

func (c *certs) AddSecret(secret *store.Secret, secretType SecretType) (certPath string, err error) {
	if secret == nil {
		err = errors.New("nil secret")
		return
	}

	var certs map[string]*cert
	var crt *cert
	var crtOk, privateKeyNull bool
	var certName string
	switch secretType {
	case FT_DEFAULT_CERT:
		// starting filename with "0" makes it first cert to be picked by HAProxy when no SNI matches.
		certName = fmt.Sprintf("0_%s_%s", secret.Namespace, secret.Name)
		certPath = path.Join(env.FrontendDir, certName)
		certs = c.frontend
	case FT_CERT:
		certName = fmt.Sprintf("%s_%s", secret.Namespace, secret.Name)
		certPath = path.Join(env.FrontendDir, certName)
		certs = c.frontend
	case BD_CERT:
		certName = fmt.Sprintf("%s_%s", secret.Namespace, secret.Name)
		certPath = path.Join(env.BackendDir, certName)
		certs = c.backend
	case CA_CERT:
		certName = fmt.Sprintf("%s_%s", secret.Namespace, secret.Name)
		certPath = path.Join(env.CaDir, certName)
		certs = c.ca
		privateKeyNull = true
	default:
		return "", errors.New("unspecified context")
	}
	crt, crtOk = certs[certName]
	if crtOk {
		crt.inUse = true
		if secret.Status == store.EMPTY {
			return crt.path, nil
		}
	}
	crt = &cert{
		path:    certPath,
		name:    fmt.Sprintf("%s/%s", secret.Namespace, secret.Name),
		inUse:   true,
		updated: true,
	}
	err = writeSecret(secret, crt, privateKeyNull)
	if err != nil {
		return "", err
	}
	certs[certName] = crt
	return crt.path, nil
}

func (c *certs) CleanCerts() {
	for i := range c.frontend {
		c.frontend[i].inUse = false
		c.frontend[i].updated = false
	}
	for i := range c.backend {
		c.backend[i].inUse = false
		c.backend[i].updated = false
	}
	for i := range c.ca {
		c.ca[i].inUse = false
		c.ca[i].updated = false
	}
}

func (c *certs) FrontCertsInUse() bool {
	for _, cert := range c.frontend {
		if cert.inUse {
			return true
		}
	}
	return false
}

func (c *certs) RefreshCerts() {
	refreshCerts(c.frontend, env.FrontendDir)
	refreshCerts(c.backend, env.BackendDir)
	refreshCerts(c.ca, env.CaDir)
}

func (c *certs) CertsUpdated() (reload bool) {
	for _, certs := range []map[string]*cert{c.frontend, c.backend, c.ca} {
		for _, crt := range certs {
			if crt.updated {
				logger.Debugf("Secret '%s' was updated", crt.name)
				reload = true
			}
		}
	}
	return reload
}

func refreshCerts(certs map[string]*cert, certDir string) {
	files, err := os.ReadDir(certDir)
	if err != nil {
		logger.Error(err)
		return
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		filename := f.Name()
		// certificate file name should be already in the format: certName.pem
		certName := strings.Split(filename, ".pem")[0]
		crt, crtOk := certs[certName]
		if !crtOk || !crt.inUse {
			logger.Error(os.Remove(path.Join(certDir, filename)))
			delete(certs, certName)
			instance.Reload("secret %s removed", certName)
		}
	}
}

func writeSecret(secret *store.Secret, c *cert, privateKeyNull bool) (err error) {
	var crtValue, keyValue []byte
	var crtOk, keyOk, pemOk bool
	var certPath string
	if privateKeyNull {
		crtValue, crtOk = secret.Data["tls.crt"]
		if !crtOk {
			return fmt.Errorf("certificate missing in %s/%s", secret.Namespace, secret.Name)
		}
		c.path += ".pem"
		return writeCert(c.path, []byte(""), crtValue)
	}
	for _, k := range []string{"tls", "rsa", "ecdsa", "dsa"} {
		keyValue, keyOk = secret.Data[k+".key"]
		crtValue, crtOk = secret.Data[k+".crt"]
		if keyOk && crtOk {
			pemOk = true
			certPath = c.path + ".pem"
			if k != "tls" {
				// HAProxy "cert bundle"
				certPath = fmt.Sprintf("%s.%s", certPath, k)
			}
			err = writeCert(certPath, keyValue, crtValue)
			if err != nil {
				return err
			}
		}
	}
	if !pemOk {
		return fmt.Errorf("certificate or private key missing in %s/%s", secret.Namespace, secret.Name)
	}
	c.path = certPath
	return nil
}

func writeCert(filename string, key, crt []byte) error {
	var f *os.File
	var err error
	if f, err = os.Create(filename); err != nil {
		logger.Error(err)
		return err
	}
	defer f.Close()
	if _, err = f.Write(key); err != nil {
		logger.Error(err)
		return err
	}
	// Force writing a newline so that parsing does not barf
	if len(key) > 0 && key[len(key)-1] != byte('\n') {
		logger.Warningf("secret key in %s does not end with \\n, appending it to avoid mangling key and certificate", filename)
		if _, err = f.WriteString("\n"); err != nil {
			logger.Error(err)
			return err
		}
	}
	if _, err = f.Write(crt); err != nil {
		logger.Error(err)
		return err
	}
	if err = f.Sync(); err != nil {
		logger.Error(err)
		return err
	}
	if err = f.Close(); err != nil {
		logger.Error(err)
		return err
	}
	return nil
}
