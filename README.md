# ![HAProxy](https://github.com/haproxytech/kubernetes-ingress/raw/master/assets/images/haproxy-weblogo-210x49.png "HAProxy")

## HAProxy Kubernetes Ingress Controller

[![Contributors](https://img.shields.io/github/contributors/haproxytech/kubernetes-ingress?color=purple)](https://github.com/haproxy/haproxy/blob/master/CONTRIBUTING)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/haproxytech/kubernetes-ingress)](https://goreportcard.com/report/github.com/haproxytech/kubernetes-ingress)

### Description

An ingress controller is a Kubernetes resource that routes traffic from outside your cluster to services within the cluster.

Detailed documentation can be found within the [Official Documentation](https://www.haproxy.com/documentation/kubernetes/latest/).

You can also find in this repository a list of all available [Ingress annotations](https://github.com/haproxytech/kubernetes-ingress/blob/master/documentation/README.md).
### Usage

Docker image is available on Docker Hub: [haproxytech/kubernetes-ingress](https://hub.docker.com/r/haproxytech/kubernetes-ingress)

If you prefer to build it from source use (change to appropriate platform if needed with TARGETPLATFORM, default platform is linux/amd64)

```bash
make build
```
With non default platform add appropriate TARGETPLATFORM

```bash
make build TARGETPLATFORM=linux/arm/v6
```

Example environment can be created with

```bash
make example
```

Please see [controller.md](https://github.com/haproxytech/kubernetes-ingress/blob/master/documentation/controller.md) for all available arguments of controller image.

Available customisations are described in [doc](https://github.com/haproxytech/kubernetes-ingress/blob/master/documentation/README.md)

Basic setup to to run controller is described in [yaml](https://github.com/haproxytech/kubernetes-ingress/blob/master/deploy/haproxy-ingress.yaml) file.

```bash
kubectl apply -f deploy/haproxy-ingress.yaml
```

### HAProxy Helm Charts

Official HAProxy Technologies Helm Charts for deploying on [Kubernetes](https://kubernetes.io/) are available in [haproxytech/helm-charts](https://github.com/haproxytech/helm-charts) repository

### Contributing

Thanks for your interest in the project and your willing to contribute:

- Pull requests are welcome!
- For commit messages and general style please follow the haproxy project's [CONTRIBUTING guide](https://github.com/haproxy/haproxy/blob/master/CONTRIBUTING) and use that where applicable.
- Please use `golangci-lint run` from [github.com/golangci/golangci-lint](https://github.com/golangci/golangci-lint) for linting code.

### Discussion

A Github issue is the right place to discuss feature requests, bug reports or any other subject that needs tracking.

To ask questions, get some help or even have a little chat, you can join our #ingress-controller channel in [HAProxy Community Slack](https://slack.haproxy.org).

## License

[Apache License 2.0](LICENSE)
