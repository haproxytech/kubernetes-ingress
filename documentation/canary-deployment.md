# Canary Deployment

[Canary deployment](https://martinfowler.com/bliki/CanaryRelease.html) is a technique for rolling out releases to a subset of users.
There can be different criteria to select a subset of users for a given release. This can be based on user cookie, header, based on a fixed percentage, etc.
The [route-acl](./README.md#route-acl) annotation can be used to configure canary-deployment by providing an in-line [HAProxy ACL](https://www.haproxy.com/blog/introduction-to-haproxy-acls/).
The route-acl is a service annotation, so the provided ACL will be used to route ingress traffic to the service annotated by route-acl.

The following example describes how to configure Canary Deployment with HAProxy Ingress Controller where 25% percent of the traffic will go to a staging backend while the rest will be routed to production.

## Production application

```yaml
kind: Deployment
apiVersion: apps/v1
metadata:
  name: echo-prod
spec:
  replicas: 1
  selector:
    matchLabels:
      app: echo-prod
  template:
    metadata:
      labels:
        app: echo-prod
    spec:
      containers:
        - name: echo-prod
          image: haproxytech/http-echo:latest
          imagePullPolicy: Never
          args:
          - --default-response=hostname
          ports:
            - name: http
              containerPort: 8888
              protocol: TCP
---
kind: Service
apiVersion: v1
metadata:
  name: echo-prod
spec:
  ports:
    - name: http
      port: 80
      protocol: TCP
      targetPort: http
  selector:
    app: echo-prod
```

## Staging application

```yaml
kind: Deployment
apiVersion: apps/v1
metadata:
  name: echo-staging
spec:
  replicas: 1
  selector:
    matchLabels:
      app: echo-staging
  template:
    metadata:
      labels:
        app: echo-staging
    spec:
      containers:
        - name: echo-staging
          image: haproxytech/http-echo:latest
          imagePullPolicy: Never
          args:
          - --default-response=hostname
          ports:
            - name: http
              containerPort: 8888
              protocol: TCP
---
kind: Service
apiVersion: v1
metadata:
  name: echo-staging
  annotations:
    route-acl: "rand(100) lt 25"
spec:
  ports:
    - name: http
      port: 80
      protocol: TCP
      targetPort: http
  selector:
    app: echo-staging
```

## Ingress

```yaml
kind: Ingress
apiVersion: networking.k8s.io/v1beta1
metadata:
  name: echo
spec:
  rules:
  - host: echo.haproxy.local
    http:
      paths:
        - path: /
          backend:
            serviceName: echo-prod
            servicePort: http
        - backend:
            serviceName: echo-staging
            servicePort: http
```

We can have ingress rules for staging and production in the same ingress resource but they **should not** share the same path otherwise the latter will overwrite the former. To avoid any confusion we can have rules in different ingress resources.

Also notice  the `route-acl:  "rand(100) lt 25"` annotation in staging application service which in addition to the defined ingress rules will result in controller writing the following HAProxy route:
```
use_backend default-echo-staging-http if { var(txn.host) echo.haproxy.local}  { rand(100) lt 25 }
```

Which means HAProxy will route traffic to `default-echo-staging-http` (assuming the application was deployed in "default" namespace) when:
- The Host header is equal to "echo.haproxy.local"
- "rand(100) lt 25" is true which is supposed to be the case 25% of the time according to the rand [documentation](https://cbonte.github.io/haproxy-dconv/2.3/configuration.html#7.3.2-rand)

If ingress traffic does not much the above rule it will follow the standard routing decision of ingress rules.

# Testing

```
$ for i in `seq 10`; do curl -H "Host: echo.haproxy.local"  http://127.0.0.1; done
echo-staging-54b9c88646-pdlzp
echo-prod-6f84dd8bfb-4rx5d
echo-prod-6f84dd8bfb-4rx5d
echo-prod-6f84dd8bfb-4rx5d
echo-staging-54b9c88646-pdlzp
echo-prod-6f84dd8bfb-4rx5d
echo-prod-6f84dd8bfb-4rx5d
echo-prod-6f84dd8bfb-4rx5d
echo-prod-6f84dd8bfb-4rx5d
echo-staging-54b9c88646-pdlzp
```
