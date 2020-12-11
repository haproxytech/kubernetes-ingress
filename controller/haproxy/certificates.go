package haproxy

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/haproxytech/kubernetes-ingress/controller/store"
)

type Certificates struct {
	//Index is secretPath
	// Value is true if secret is used otherwise false
	frontend map[string]bool
}

type SecretType int

//nolint
const (
	NONE_CERT SecretType = iota
	FT_CERT
	FT_DEFAULT_CERT
)

type SecretCtx struct {
	DefaultNS  string
	SecretPath string
	SecretType SecretType
}

var certDir string

func NewCertificates(dir string) *Certificates {
	certDir = dir
	return &Certificates{
		frontend: make(map[string]bool),
	}
}

func (c *Certificates) HandleTLSSecret(k8s store.K8s, secretCtx SecretCtx) (reload bool) {
	secret := fetchSecret(k8s, secretCtx.SecretPath, secretCtx.DefaultNS)
	if secret == nil || secret.Status == store.DELETED {
		return false
	}
	certName := ""
	switch secretCtx.SecretType {
	case FT_DEFAULT_CERT:
		// starting filename with "0" makes it first cert to be picked by HAProxy when no SNI matches.
		certName = path.Join(certDir, fmt.Sprintf("0_%s_%s.pem", secret.Namespace, secret.Name))
	case FT_CERT:
		certName = path.Join(certDir, fmt.Sprintf("%s_%s.pem", secret.Namespace, secret.Name))
	default:
		logger.Error("unspecified context")
		return false
	}
	if _, ok := c.frontend[certName]; ok {
		c.frontend[certName] = true
		return false
	}
	err := writeSecret(secret, certName)
	if err != nil {
		logger.Error(err)
		return false
	}
	c.frontend[certName] = true
	return true
}

func (c *Certificates) Clean() {
	for i := range c.frontend {
		c.frontend[i] = false
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
		used := c.frontend[filename]
		if !used {
			logger.Error(os.Remove(path.Join(certDir, filename)))
			delete(c.frontend, filename)
		}
	}
}

func fetchSecret(k8s store.K8s, defaultNs, secretPath string) (secret *store.Secret) {
	secretName := ""
	secretNamespace := defaultNs
	parts := strings.Split(secretPath, "/")
	if len(parts) > 1 {
		secretNamespace = parts[0]
		secretName = parts[1]
	} else {
		secretName = parts[0] // only secretname is here
	}
	ns, namespaceOK := k8s.Namespaces[secretNamespace]
	if !namespaceOK {
		logger.Warningf("namespace [%s] does not exist, ignoring.", secretNamespace)
		return nil
	}
	secret, secretOK := ns.Secret[secretName]
	if !secretOK {
		logger.Warningf("secret [%s/%s] does not exist, ignoring.", secretNamespace, secretName)
		return nil
	}
	return secret
}

func writeSecret(secret *store.Secret, certName string) (err error) {
	for _, k := range []string{"tls", "rsa", "ecdsa"} {
		key := secret.Data[k+".key"]
		crt := secret.Data[k+".crt"]
		if len(key) != 0 && len(crt) != 0 {
			return writeCert(certName, key, crt)
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
	//Force writing a newline so that parsing does not barf
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
