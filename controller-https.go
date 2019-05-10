package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/haproxytech/models"
)

func (c *HAProxyController) writeCert(filename string, key, crt []byte) error {
	var f *os.File
	var err error
	if f, err = os.Create(filename); err != nil {
		log.Println(err)
		return err
	}
	defer f.Close()
	if _, err = f.Write(key); err != nil {
		log.Println(err)
		return err
	}
	if _, err = f.Write(crt); err != nil {
		log.Println(err)
		return err
	}
	if err = f.Sync(); err != nil {
		log.Println(err)
		return err
	}
	if err = f.Close(); err != nil {
		log.Println(err)
		return err
	}
	return nil
}

func (c *HAProxyController) handleHTTPS(maxProcsStatus, numThreadsStat *StringW, transaction *models.Transaction) (reloadRequested bool, usingHTTPS bool, err error) {
	usingHTTPS = false
	nativeAPI := c.NativeAPI
	reloadRequested = false
	if c.osArgs.DefaultCertificate.Name == "" {
		err := c.removeHTTPSListeners(transaction)
		return reloadRequested, usingHTTPS, err
	}
	secretAnn, errSecret := GetValueFromAnnotations("ssl-certificate", c.cfg.ConfigMap.Annotations)
	secretData := strings.Split(secretAnn.Value, "/")
	if len(secretData) != 2 {
		return reloadRequested, usingHTTPS, errors.New("invalid secret data")
	}

	minProc := 1
	maxProcs, _ := strconv.Atoi(maxProcsStatus.Value) // always number
	numThreads, _ := strconv.Atoi(numThreadsStat.Value)
	if maxProcs < 2 {
		if numThreads < 2 {
			minProc = 0
		}
	}
	namespace, ok := c.cfg.Namespace[secretData[0]]
	if !ok {
		return reloadRequested, usingHTTPS, errors.New("invalid namespace " + secretData[0])
	}

	if errSecret == nil && (secretAnn.Status != "" || maxProcsStatus.Status != "") {
		secret, ok := namespace.Secret[secretData[1]]
		if !ok {
			log.Println("secret not found", secretData[1])
			err := c.removeHTTPSListeners(transaction)
			return reloadRequested, usingHTTPS, err
		}
		//two options are allowed, tls, rsa+ecdsa
		rsaKey, rsaKeyOK := secret.Data["rsa.key"]
		rsaCrt, rsaCrtOK := secret.Data["rsa.crt"]
		ecdsaKey, ecdsaKeyOK := secret.Data["ecdsa.key"]
		ecdsaCrt, ecdsaCrtOK := secret.Data["ecdsa.crt"]
		haveCert := false
		//log.Println(secretName.Value, rsaCrtOK, rsaKeyOK, ecdsaCrtOK, ecdsaKeyOK)
		if rsaKeyOK && rsaCrtOK || ecdsaKeyOK && ecdsaCrtOK {
			if rsaKeyOK && rsaCrtOK {
				err := c.writeCert(HAProxyCertDir+"cert.pem.rsa", rsaKey, rsaCrt)
				if err != nil {
					err1 := c.removeHTTPSListeners(transaction)
					LogErr(err1)
					return reloadRequested, usingHTTPS, err
				}
				haveCert = true
			}
			if ecdsaKeyOK && ecdsaCrtOK {
				err := c.writeCert(HAProxyCertDir+"cert.pem.ecdsa", ecdsaKey, ecdsaCrt)
				if err != nil {
					err1 := c.removeHTTPSListeners(transaction)
					LogErr(err1)
					return reloadRequested, usingHTTPS, err
				}
				haveCert = true
			}
		} else {
			tlsKey, tlsKeyOK := secret.Data["tls.key"]
			tlsCrt, tlsCrtOK := secret.Data["tls.crt"]
			if tlsKeyOK && tlsCrtOK {
				err := c.writeCert(HAProxyCertDir+"cert.pem", tlsKey, tlsCrt)
				if err != nil {
					err1 := c.removeHTTPSListeners(transaction)
					LogErr(err1)
					return reloadRequested, usingHTTPS, err
				}
				haveCert = true
			}
		}
		if !haveCert {
			err := c.removeHTTPSListeners(transaction)
			LogErr(err)
			return reloadRequested, usingHTTPS, fmt.Errorf("no certificate")
		}

		port := int64(443)
		listener := &models.Bind{
			Address:        "0.0.0.0",
			Port:           &port,
			Ssl:            true,
			SslCertificate: HAProxyCertDir,
		}
		maxIndex := maxProcs
		if maxProcs < 2 {
			maxIndex = numThreads
		}
		listeners := *c.cfg.HTTPSListeners
		if len(listeners) > maxIndex {
			maxIndex = len(listeners)
		}
		usingHTTPS = true
		for index := minProc; index < maxIndex; index++ {
			data, ok := listeners[index]
			if !ok {
				data = &IntW{
					Status: ADDED,
				}
				listeners[index] = data
			} else {
				if secret.Status != "" {
					data.Status = secret.Status
				} else if maxProcsStatus.Status != "" {
					data.Status = maxProcsStatus.Status
				}
			}
			if index >= maxProcs && index >= numThreads {
				data.Status = DELETED
			}
			if numThreads < 2 {
				listener.Process = strconv.Itoa(index + 1)
			} else {
				listener.Process = fmt.Sprintf("1/%d", index+1)
			}
			listener.Name = "https_" + strconv.Itoa(index+1)
			switch data.Status {
			case ADDED:
				if err = nativeAPI.Configuration.CreateBind(FrontendHTTPS, listener, transaction.ID, 0); err != nil {
					if strings.Contains(err.Error(), "already exists") {
						if err = nativeAPI.Configuration.EditBind(listener.Name, FrontendHTTPS, listener, transaction.ID, 0); err != nil {
							return reloadRequested, usingHTTPS, err
						}
					} else {
						return reloadRequested, usingHTTPS, err
					}
				}
			case MODIFIED:
				if err = nativeAPI.Configuration.EditBind(listener.Name, FrontendHTTPS, listener, transaction.ID, 0); err != nil {
					return reloadRequested, usingHTTPS, err
				}
			case DELETED:
				if err = nativeAPI.Configuration.DeleteBind(listener.Name, FrontendHTTPS, transaction.ID, 0); err != nil {
					return reloadRequested, usingHTTPS, err
				}
			}
		}
	}

	listeners := *c.cfg.HTTPSListeners
	for _, listener := range listeners {
		if listener.Status != DELETED {
			return reloadRequested, true, nil
		}
	}
	return reloadRequested, usingHTTPS, nil
}
