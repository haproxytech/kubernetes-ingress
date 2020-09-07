
# ![HAProxy](../../assets/images/haproxy-weblogo-210x49.png "HAProxy")

# HAProxy kubernetes ingress controller in kind

# prerequisites

[kind](https://kind.sigs.k8s.io/docs/user/quick-start/)

```bash
GO111MODULE="on" go get sigs.k8s.io/kind@v0.8.1
```

## How to :runner: it

```bash
./create.sh
```

it will create all the services, pods, resources needed for initial start

all services are build from https://github.com/oktalz/go-web-simple, each has it unique response

for http services u can test

```bash
curl --header "Host: hr.haproxy" 127.0.0.1:30080/gids
```

response will be from 4 pods (example is 8 requests):

```bash
zagreb-TH9-1
zagreb-PAQ-1
zagreb-X34-1
zagreb-XVT-1
zagreb-TH9-2
zagreb-PAQ-2
zagreb-X34-2
zagreb-XVT-2
```

`[SERVICE]-[ID]-[RESPONSE_COUNTER]`

- you can see what service and what pod (each has its own id) gives response.
- last number is response number so you can see how many request was put to each service

for second http service

```bash
curl --header "Host: fr.haproxy" 127.0.0.1:30080/gids
```

for pure tcp service

```bash
curl 127.0.0.1:32767/gids
```

# recommended software

bash

- to enter container use:

```bash
name=$(kubectl get --namespace=haproxy-controller pods \
-o go-template --template '{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}' \
--sort-by=.metadata.creationTimestamp | grep haproxy-ingress | tail -1); \
kubectl exec -it --namespace=haproxy-controller $name -- /bin/sh
```

[K9s - Kubernetes CLI](https://github.com/derailed/k9s)

- after starting press 0 to see all namespaces
