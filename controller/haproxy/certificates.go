package haproxy

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type Certificates struct {
	// Index is secretPath
	// Value is true if secret is used otherwise false
	frontend map[string]bool
	backend  map[string]bool
	ca       map[string]bool
}

type SecretType int

//nolint:golint,stylecheck
const (
	NONE_CERT SecretType = iota
	FT_CERT
	FT_DEFAULT_CERT
	BD_CERT
	CA_CERT
)

type SecretCtx struct {
	DefaultNS  string
	SecretPath string
	SecretType SecretType
}

var ErrCertNotFound = errors.New("notFound")
var frontendCertDir string
var backendCertDir string
var caCertDir string

func NewCertificates(caDir, ftDir, bdDir string) *Certificates {
	frontendCertDir = ftDir
	backendCertDir = bdDir
	caCertDir = caDir
	return &Certificates{
		frontend: make(map[string]bool),
		backend:  make(map[string]bool),
		ca:       make(map[string]bool),
	}
}

func (c *Certificates) HandleTLSSecret(k8s store.K8s, secretCtx SecretCtx) (certPath string, updated bool, err error) {
	secret, err := k8s.FetchSecret(secretCtx.SecretPath, secretCtx.DefaultNS)
	if secret == nil {
		logger.Warning(err)
		return "", false, ErrCertNotFound
	}
	if secret.Status == store.DELETED {
		return "", true, nil
	}
	var certs map[string]bool
	certName := ""
	var privateKeyNull bool
	switch secretCtx.SecretType {
	case FT_DEFAULT_CERT:
		// starting filename with "0" makes it first cert to be picked by HAProxy when no SNI matches.
		certName = fmt.Sprintf("0_%s_%s.pem", secret.Namespace, secret.Name)
		certPath = path.Join(frontendCertDir, certName)
		certs = c.frontend
	case FT_CERT:
		certName = fmt.Sprintf("%s_%s.pem", secret.Namespace, secret.Name)
		certPath = path.Join(frontendCertDir, certName)
		certs = c.frontend
	case BD_CERT:
		certName = fmt.Sprintf("%s_%s.pem", secret.Namespace, secret.Name)
		certPath = path.Join(backendCertDir, certName)
		certs = c.backend
	case CA_CERT:
		certName = fmt.Sprintf("%s_%s.pem", secret.Namespace, secret.Name)
		certPath = path.Join(caCertDir, certName)
		certs = c.ca
		privateKeyNull = true
	default:
		return "", false, errors.New("unspecified context")
	}
	if _, ok := certs[certName]; ok && secret.Status == store.EMPTY {
		certs[certName] = true
		return certPath, false, nil
	}
	err = writeSecret(secret, certPath, privateKeyNull)
	if err != nil {
		return "", false, err
	}
	certs[certName] = true
	return certPath, true, nil
}

func (c *Certificates) Clean() {
	for i := range c.frontend {
		c.frontend[i] = false
	}
	for i := range c.backend {
		c.backend[i] = false
	}
	for i := range c.ca {
		c.backend[i] = false
	}
}

func (c *Certificates) FrontendCertsEnabled() bool {
	for _, used := range c.frontend {
		if used {
			return true
		}
	}
	return false
}

// Refresh removes unused certs from HAProxyCertDir
// Certificates will remain in HAProxy memory until next Reload
func (c *Certificates) Refresh() {
	refreshCerts(c.frontend, frontendCertDir)
	refreshCerts(c.backend, backendCertDir)
	refreshCerts(c.ca, caCertDir)
}

func refreshCerts(certs map[string]bool, certDir string) {
	files, err := ioutil.ReadDir(certDir)
	if err != nil {
		logger.Error(err)
		return
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}
		filename := f.Name()
		used := certs[filename]
		if !used {
			logger.Error(os.Remove(path.Join(certDir, filename)))
			delete(certs, filename)
		}
	}
}

func writeSecret(secret *store.Secret, certPath string, privateKeyNull bool) (err error) {
	for _, k := range []string{"tls", "rsa", "ecdsa"} {
		key := secret.Data[k+".key"]
		if key == nil {
			if privateKeyNull {
				key = []byte("")
			} else {
				return fmt.Errorf("private key missing in %s/%s", secret.Namespace, secret.Name)
			}
		}
		crt := secret.Data[k+".crt"]
		if len(crt) != 0 {
			return writeCert(certPath, key, crt)
		}
	}
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
