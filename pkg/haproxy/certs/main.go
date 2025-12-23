package certs

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/renameio"
	"github.com/haproxytech/client-native/v6/runtime"
	"github.com/haproxytech/kubernetes-ingress/pkg/fs"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/api"
	"github.com/haproxytech/kubernetes-ingress/pkg/haproxy/instance"
	"github.com/haproxytech/kubernetes-ingress/pkg/store"
	"github.com/haproxytech/kubernetes-ingress/pkg/utils"
)

type certs struct {
	frontend map[string]*cert
	backend  map[string]*cert
	ca       map[string]*cert
	TCPCR    map[string]*cert
	client   api.HAProxyClient
	mu       *sync.Mutex
}

type Certificates interface {
	// Add takes a secret and its type and creats or updates the corresponding certificate
	AddSecret(secret *store.Secret, secretType SecretType) (certPath string, err error)
	// FrontCertsInuse returns true if a frontend certificate is configured.
	FrontCertsInUse() bool
	// Updated returns true if there is any updadted/created certificate
	CertsUpdated() bool
	// Refresh removes unused certs from HAProxyCertDir
	RefreshCerts(api api.HAProxyClient)
	// Clean cleans certificates state
	CleanCerts()
	SetAPI(api api.HAProxyClient)
}

type cert struct {
	name    string
	path    string
	inUse   bool
	updated bool
	ca      bool
}

type SecretType int

type Env struct {
	MainDir     string
	FrontendDir string
	BackendDir  string
	CaDir       string
	TCPCRDir    string
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
	TCP_CERT
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
	if env.TCPCRDir == "" {
		return nil, errors.New("empty name for TCP Cert Directory")
	}
	return &certs{
		frontend: make(map[string]*cert),
		backend:  make(map[string]*cert),
		ca:       make(map[string]*cert),
		TCPCR:    make(map[string]*cert),
		mu:       &sync.Mutex{},
	}, nil
}

func (c *certs) AddSecret(secret *store.Secret, secretType SecretType) (certPath string, err error) {
	if secret == nil {
		err = errors.New("nil secret")
		return certPath, err
	}

	var certs map[string]*cert
	var crt *cert
	var crtOk, isCa bool
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
		isCa = true
	case TCP_CERT:
		certName = fmt.Sprintf("%s_%s", secret.Namespace, secret.Name)
		certPath = path.Join(env.TCPCRDir, certName)
		certs = c.TCPCR
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
		path:  certPath,
		name:  fmt.Sprintf("%s/%s", secret.Namespace, secret.Name),
		inUse: true,
		ca:    isCa,
	}
	err = c.writeSecret(secret, crt, isCa)
	if err != nil {
		return "", err
	}
	certs[certName] = crt
	return crt.path, nil
}

func (c *certs) updateRuntime(filename string, payload []byte, isCa bool) (bool, error) {
	// Only 1 transaction in parallel is possible for now in haproxy
	// Keep this mutex for now to ensure that we perform 1 transaction at a time
	certType := "cert"
	entryCreate := c.client.CertEntryCreate
	entrySet := c.client.CertEntrySet
	entryCommit := c.client.CertEntryCommit
	if isCa {
		entryCreate = c.client.CertAuthEntryCreate
		entrySet = c.client.CertAuthEntrySet
		entryCommit = c.client.CertAuthEntryCommit
		certType = "ca"
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	var updated, alreadyExists bool

	err = entryCreate(filename)
	// If already exists
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			alreadyExists = true
		} else {
			return updated, err
		}
	} else {
		utils.GetLogger().Debugf("`new ssl %s` ok[%s]", certType, filename)
	}

	err = entrySet(filename, payload)
	if err != nil {
		return updated, err
	}
	utils.GetLogger().Debugf("`set ssl %s` ok[%s]", certType, filename)

	err = entryCommit(filename)
	if err != nil {
		// Abort transaction
		errAbort := c.client.CertEntryAbort(filename)
		// If error, just log it
		// a Reload will follow, transaction will be gone no matter what
		if errAbort != nil {
			utils.GetLogger().Error(errAbort)
		}

		return updated, err
	}
	updated = true
	utils.GetLogger().Debugf("`commit ssl %s` ok [%s]", certType, filename)

	if !alreadyExists && !isCa {
		dirPath := filepath.Dir(filename)
		err = c.client.CrtListEntryAdd(dirPath,
			runtime.CrtListEntry{
				File: filename,
			})
		if err != nil {
			return false, err
		}
		utils.GetLogger().Debugf("`%s-list add` ok [%s] [%s] ", certType, dirPath, filename)
	}

	return updated, nil
}

func (c *certs) deleteRuntime(crtList, filename string) error {
	// Only 1 transaction in parallel is possible for now in haproxy
	// Keep this mutex for now to ensure that we perform 1 transaction at a time
	c.mu.Lock()
	defer c.mu.Unlock()

	var err error
	certFile := path.Join(crtList, filename)
	err = c.client.CrtListEntryDelete(crtList, certFile, nil)
	if err != nil {
		return err
	}
	utils.GetLogger().Debugf("del ssl crt-list` ok [%s %s]", crtList, certFile)

	err = c.client.CertEntryDelete(certFile)
	if err != nil {
		return err
	}
	utils.GetLogger().Debugf("del ssl cert` ok [%s]", certFile)
	return nil
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
	for i := range c.TCPCR {
		c.TCPCR[i].inUse = false
		c.TCPCR[i].updated = false
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

func (c *certs) RefreshCerts(api api.HAProxyClient) {
	c.SetAPI(api)
	c.refreshCerts(c.frontend, env.FrontendDir)
	c.refreshCerts(c.backend, env.BackendDir)
	c.refreshCerts(c.ca, env.CaDir)
	c.refreshCerts(c.TCPCR, env.TCPCRDir)
}

func (c *certs) CertsUpdated() (reload bool) {
	for _, certs := range []map[string]*cert{c.frontend, c.backend, c.ca, c.TCPCR} {
		for _, crt := range certs {
			if crt.updated {
				logger.Debugf("Secret '%s' was updated", crt.name)
				reload = true
			}
		}
	}
	return reload
}

func (c *certs) refreshCerts(certs map[string]*cert, certDir string) {
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
		// SKIP temporary file created by renameio
		// fileName .e2e-tests-https-runtime_haproxy-offload-test.pem2179154433
		// revisit this, take time to think about another way
		if certName+".pem" != filename {
			// This happens with temp files: created by renameio
			continue
		}
		if !crtOk || !crt.inUse {
			err := c.deleteRuntime(certDir, filename)
			if err != nil {
				instance.Reload("Runtime delete of cert file '%s' failed : %s", filename, err.Error())
			} else {
				utils.GetLogger().Debugf("Runtime delete of cert ok [%s]", filename)
			}
			fs.AddDelayedFunc(filename, func() {
				logger.Error(os.Remove(path.Join(certDir, filename)))
			})
			delete(certs, certName)
		}
	}
}

func (c *certs) writeSecret(secret *store.Secret, cert *cert, isCa bool) (err error) {
	var crtValue, keyValue []byte
	var crtOk, keyOk, pemOk bool
	var certPath string
	if isCa {
		crtValue, crtOk = secret.Data["tls.crt"]
		if !crtOk {
			return fmt.Errorf("certificate missing in %s/%s", secret.Namespace, secret.Name)
		}
		cert.path += ".pem"
		content := certContent([]byte(""), crtValue)
		return c.writeCert(cert, cert.path, content, isCa)
	}
	for _, k := range []string{"tls", "rsa", "ecdsa", "dsa"} {
		keyValue, keyOk = secret.Data[k+".key"]
		crtValue, crtOk = secret.Data[k+".crt"]
		if keyOk && crtOk {
			pemOk = true
			certPath = cert.path + ".pem"
			if k != "tls" {
				// HAProxy "cert bundle"
				certPath = fmt.Sprintf("%s.%s", certPath, k)
			}
			content := certContent(keyValue, crtValue)
			err = c.writeCert(cert, certPath, content, isCa)
			if err != nil {
				return err
			}
		}
	}
	if !pemOk {
		return fmt.Errorf("certificate or private key missing in %s/%s", secret.Namespace, secret.Name)
	}
	cert.path = certPath
	return nil
}

func (c *certs) writeCert(cert *cert, filename string, content []byte, isCa bool) error {
	fs.Writer.Write(func() {
		if _, err := os.Stat(filename); err != nil {
			// If file does not exist, contrary to the map files, it's not working to create an empty file.
			// There need to be a valid certificate file.
			// So, let's create the right content.
			// Then, on update, it will be written with the delayed function.
			if os.IsNotExist(err) {
				err := renameio.WriteFile(filename, content, 0o666)
				if err != nil {
					logger.Error(err)
					return
				}
				utils.GetLogger().Debugf("cert written on disk[%s]", filename)
			} else {
				logger.Error(err)
				return
			}
		}

		updated, err := c.updateRuntime(filename, content, isCa)
		if err != nil {
			instance.Reload("Runtime update of cert file '%s' failed : %s", filename, err.Error())
		} else if updated {
			utils.GetLogger().Debugf("Runtime update of cert ok [%s]", filename)
			cert.updated = true
		}

		// In runtime failed or did succeed, it needs to be written on disk.
		fs.AddDelayedFunc(filename, func() {
			err := renameio.WriteFile(filename, content, 0o666)
			if err != nil {
				logger.Error(err)
				return
			}
			utils.GetLogger().Debugf("Delayed writing cert on disk ok [%s] ", filename)
		})
	})

	return nil
}

func certContent(key, crt []byte) []byte {
	buff := make([]byte, 0, len(key)+len(crt)+1)
	buff = append(buff, key...)
	if len(key) > 0 && key[len(key)-1] != byte('\n') {
		buff = append(buff, byte('\n'))
	}
	buff = append(buff, crt...)
	return buff
}

func (c *certs) SetAPI(api api.HAProxyClient) {
	c.client = api
}
