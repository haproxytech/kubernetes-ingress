
# ![HAProxy](../../assets/images/haproxy-weblogo-210x49.png "HAProxy")

# HAProxy kubernetes ingress controller in kind

# prerequisites

[kind](https://kind.sigs.k8s.io/docs/user/quick-start/)

```bash
GO111MODULE="on" go get sigs.k8s.io/kind@v0.8.1
```

## How to :runner: it

### Cluster initialization
```bash
./create.sh
```

Creates a Kubernetes cluster named `dev` via [kind](https://kind.sigs.k8s.io/) tool.

This will also deploy the HAProxy Ingress Controller using config in `deploy/tests/config` directory.

### Testing application
```bash
kubectl apply -f ./config/echo-app.yaml
```

Deploys `haproxytech/http-echo` as a test application.

```bash
curl --header "Host: echo.haproxy.local" 127.0.0.1:30080
```

Response will include a couple of useful information:
- The application POD name
- Request attributes


### E2E tests

```bash
go test -v --tags=<tag_name> ./e2e/...
```

This will run all e2e tests in `./e2e` directory tagged with `<tag_name>`.
There are two available tags:
- **e2e_parallel**: which will run tests in parallel
- **e2e_sequential**: which will run tests in sequence

Currently two tests are run sequentially:
- endpoints: in order to test endpoints scaling without reloading haproxy.
- tls-auth:  tls authentication is a global config that will impact other tests running in parallel.

Each E2E test runs in its **own Namespace** and has its own directory.
Tests are deployed by applying yaml files or/and templates from the `config` directory of the corresponding test.
When using yaml templates, the generated yaml files are stored in a temporary directory in `/tmp/`.
Using `--dev` option will keep generated files after test execution.
Example:
```bash
go test -v --tags=e2e_sequential --dev ./e2e/endpoints/
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
