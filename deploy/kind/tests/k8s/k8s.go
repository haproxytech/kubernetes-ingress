// Copyright 2019 HAProxy Technologies LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8s

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/pointer"
)

func New(t *testing.T) *kubernetes.Clientset {
	home, err := os.UserHomeDir()
	assert.Nil(t, err)

	config, err := clientcmd.BuildConfigFromFlags("", fmt.Sprintf("%s/.kube/config", home))
	assert.Nil(t, err)

	cs, err := kubernetes.NewForConfig(config)
	assert.Nil(t, err)

	return cs
}

func AddAnnotations(ing metav1.Object, additionals map[string]string) {
	a := ing.GetAnnotations()
	for k, v := range additionals {
		a[k] = v
	}
	ing.SetAnnotations(a)
}

func NewIngress(name, release, path string) *networkingv1beta1.Ingress {
	return &networkingv1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", name, release),
			Annotations: map[string]string{
				"ingress.class": "haproxy",
			},
		},
		Spec: networkingv1beta1.IngressSpec{
			Rules: []networkingv1beta1.IngressRule{
				{
					Host: fmt.Sprintf("%s-%s.haproxy", name, release),
					IngressRuleValue: networkingv1beta1.IngressRuleValue{
						HTTP: &networkingv1beta1.HTTPIngressRuleValue{
							Paths: []networkingv1beta1.HTTPIngressPath{
								{
									Path: path,
									Backend: networkingv1beta1.IngressBackend{
										ServiceName: fmt.Sprintf("%s-%s", name, release),
										ServicePort: intstr.FromString("http"),
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func EditServicePort(service *corev1.Service, port int32) {
	service.Spec.Ports[0].Port = port
}

func NewService(name, release string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", name, release),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:       9898,
					TargetPort: intstr.FromString("http"),
					Protocol:   corev1.ProtocolTCP,
					Name:       "http",
				},
			},
			Selector: map[string]string{
				"app": fmt.Sprintf("%s-%s", name, release),
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

func NewDeployment(name, release string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", name, release),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": fmt.Sprintf("%s-%s", name, release),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": fmt.Sprintf("%s-%s", name, release),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  name,
							Image: "ghcr.io/stefanprodan/podinfo:5.0.1",
							Command: []string{
								"./podinfo",
								"--port=9898",
								"--level=info",
								"--random-delay=false",
								"--random-error=false",
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 9898,
									Protocol:      corev1.ProtocolTCP,
								},
							},
						},
					},
				},
			},
		},
	}
}

func EditPodImage(deployment *appsv1.Deployment, image string) {
	deployment.Spec.Template.Spec.Containers[0].Image = image
}

func EditPodCommand(deployment *appsv1.Deployment, command ...string) {
	deployment.Spec.Template.Spec.Containers[0].Command = command
}

func EditPodExposedPort(deployment *appsv1.Deployment, port int32) {
	deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort = port
}

func NewCertificateSigningRequest(name, release string, key *rsa.PrivateKey, dnsNames ...string) *certificatesv1beta1.CertificateSigningRequest {
	tpl := x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName: dnsNames[0],
		},
		SignatureAlgorithm: x509.SHA256WithRSA,
		DNSNames:           dnsNames,
	}
	csrBytes, err := x509.CreateCertificateRequest(rand.Reader, &tpl, key)
	if err != nil {
		panic(err)
	}
	certificateRequestBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: csrBytes})
	return &certificatesv1beta1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", name, release),
		},
		Spec: certificatesv1beta1.CertificateSigningRequestSpec{
			Groups:  []string{"system:authenticated"},
			Request: certificateRequestBytes,
			Usages: []certificatesv1beta1.KeyUsage{
				certificatesv1beta1.UsageDigitalSignature,
				certificatesv1beta1.UsageKeyEncipherment,
				certificatesv1beta1.UsageServerAuth,
			},
		},
	}
}

func NewTlsSecret(key *rsa.PrivateKey, cert []byte, name, release string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", name, release),
		},
		Data: map[string][]byte{
			"tls.crt": cert,
			"tls.key": pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}),
		},
	}
}

func GetCaOrFail(t *testing.T, cs *kubernetes.Clientset) (ca *x509.Certificate) {
	var err error

	var sal *corev1.SecretList
	sal, err = cs.CoreV1().Secrets("default").List(context.Background(), metav1.ListOptions{})
	assert.Nil(t, err)

	for _, sa := range sal.Items {
		if strings.HasPrefix(sa.Name, "default-token") {
			block, _ := pem.Decode(sa.Data["ca.crt"])
			ca, err = x509.ParseCertificate(block.Bytes)
			if !assert.Nil(t, err) {
				t.FailNow()
			}
			break
		}
	}

	return
}

func ApproveCSRAndGetCertificate(t *testing.T, clientset *kubernetes.Clientset, csr *certificatesv1beta1.CertificateSigningRequest) []byte {
	var err error

	csr.Status.Conditions = append(csr.Status.Conditions, certificatesv1beta1.CertificateSigningRequestCondition{
		Type:           certificatesv1beta1.CertificateApproved,
		Reason:         "Testing",
		Message:        "This CSR was approved by HAProxy Ingress Controller test suite",
		LastUpdateTime: metav1.Now(),
	})
	csr, err = clientset.CertificatesV1beta1().CertificateSigningRequests().UpdateApproval(context.Background(), csr, metav1.UpdateOptions{})
	if !assert.Nil(t, err) {
		t.FailNow()
	}

	if !assert.Eventually(t, func() bool {
		csr, err = clientset.CertificatesV1beta1().CertificateSigningRequests().Get(context.Background(), csr.Name, metav1.GetOptions{})
		if err != nil {
			panic(err)
		}
		return csr.Status.Certificate != nil
	}, time.Minute, time.Second) {
		t.FailNow()
	}

	return csr.Status.Certificate
}

// prometherion: source-code of the Docker image
// quay.io/prometherion/proxy-protocol-app:latest is available here:
// https://github.com/prometherion/test-app
// TODO: evaluate to host the sample apps source-code in the HAProxy GitHub org
func NewProxyProtocol(name, release string) (deployment *appsv1.Deployment, svc *corev1.Service) {
	deployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", name, release),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": fmt.Sprintf("%s-%s", name, release),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": fmt.Sprintf("%s-%s", name, release),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  name,
							Image: "quay.io/prometherion/proxy-protocol-app:latest",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8080,
									Protocol:      corev1.ProtocolTCP,
								},
							},
						},
					},
				},
			},
		},
	}
	svc = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", name, release),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:       8080,
					TargetPort: intstr.FromString("http"),
					Protocol:   corev1.ProtocolTCP,
					Name:       "http",
				},
			},
			Selector: map[string]string{
				"app": fmt.Sprintf("%s-%s", name, release),
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	return
}

func NewOffloadedSsl(name, release string) (deployment *appsv1.Deployment, svc *corev1.Service) {
	deployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", name, release),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": fmt.Sprintf("%s-%s", name, release),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": fmt.Sprintf("%s-%s", name, release),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  name,
							Image: "quay.io/prometherion/simple-https-listener:latest",
							Args: []string{
								"--listening-port=8443",
							},
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 8443,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "certs",
									MountPath: "/opt/simple-https-listener",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: fmt.Sprintf("%s-%s", name, release),
								},
							},
						},
					},
				},
			},
		},
	}
	svc = &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", name, release),
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:       8443,
					TargetPort: intstr.FromString("http"),
					Protocol:   corev1.ProtocolTCP,
					Name:       "http",
				},
			},
			Selector: map[string]string{
				"app": fmt.Sprintf("%s-%s", name, release),
			},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
	return
}
