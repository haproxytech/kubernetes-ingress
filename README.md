# ![HAProxy](https://github.com/haproxytech/kubernetes-ingress/raw/master/assets/images/haproxy-weblogo-210x49.png "HAProxy")

## HAProxy Kubernetes Ingress Controller

### Description

An ingress controller is a Kubernetes resource that routes traffic from outside your cluster to services within the cluster. 

Detailed documentation can be found within the [Official Documentation](https://www.haproxy.com/documentation/hapee/1-9r1/traffic-management/kubernetes-ingress-controller/)

### Usage

Docker image is available on Docker Hub: [haproxytech/kubernetes-ingress](https://hub.docker.com/r/haproxytech/kubernetes-ingress)

If you prefer to build it from source use
```
docker build -t haproxytech/kubernetes-ingress -f build/Dockerfile .
```

Please see [controller.md](https://github.com/haproxytech/kubernetes-ingress/blob/master/documentation/controller.md) for all available arguments of controler image.

Available customisations are described in [doc](https://github.com/haproxytech/kubernetes-ingress/blob/master/documentation/README.md)

Basic setup to to run controller is described in [yaml](https://github.com/haproxytech/kubernetes-ingress/blob/master/deploy/haproxy-ingress.yaml) file.
```
kubectl apply -f deploy/haproxy-ingress.yaml
```

### Contributing

For commit messages and general style please follow the haproxy project's [CONTRIBUTING guide](https://github.com/haproxy/haproxy/blob/master/CONTRIBUTING) and use that where applicable.

Please use `golangci-lint run` from [github.com/golangci/golangci-lint](https://github.com/golangci/golangci-lint) for linting code.

## License

[Apache License 2.0](LICENSE)
