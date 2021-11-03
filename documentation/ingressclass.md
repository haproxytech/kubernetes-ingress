# IngressClass

An Ingress resource can target a specific Ingress controller instance which is useful when running multiple ingress controllers in the same cluster. Targetting an Ingress controller means only a specific controller should handle/implement the ingress resource.
This can be done using either the `IngressClassName` field or the `ingress.class` annotation.

## IngressClassName
The `IngressClassName` field is available in kubernetes 1.18+ and is the [official](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-class) way to handle this. This field should reference an IngressClass resource that contains the name of the controller that should implement the class.
For example, let's consider the following Ingress object:
```yaml
kind: Ingress
apiVersion: networking.k8s.io/v1
metadata:
  name: test
spec:
  ingressClassName: haproxy
  rules:
    - host: test.k8s.local
      http:
        paths:
          - path: /
            backend:
              serviceName: http-echo
              servicePort: http
```

If there is a single HAProxy Ingress Controller instance then no need to set `--ingress.class`, but rather create an IngressClass resource with Spec.Controller set to **haproxy.org/ingress-controller**
```yaml
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: haproxy
spec:
  controller: haproxy.org/ingress-controller
```

In the other hand if multiple HAProxy Ingress Controllers are deployed then `--ingress.class` argument can be used to target one of them. This can be done via the following IngressClass (notice Spec.Controller is set to **haproxy.org/ingress-controller/prod**)
```yaml
apiVersion: networking.k8s.io/v1
kind: IngressClass
metadata:
  name: haproxy-prod
spec:
  controller: haproxy.org/ingress-controller/prod
```
In this case `--ingress.class` should be set to **prod** and the `ingressClassName` field should be **haproxy-prod**.

## ingress.class annotation
The `ingress.class` annotation is the legacy way to target an Ingress Controller.
For example, let's consider the following Ingress object:
```yaml
kind: Ingress
apiVersion: networking.k8s.io/v1
metadata:
  name: test
  annotations:
    ingress.class: haproxy
spec:
  rules:
    - host: test.k8s.local
      http:
        paths:
          - path: /
            backend:
              serviceName: http-echo
              servicePort: http
```
In this case only HAProxy Ingress Controllers with `--ingress.class` set to "haproxy" are going to implement the previous Ingress object.

## Eligibility rules
- If the `--ingress.class` argument of the controller is not configured:
  - **Accept** Ingress resource when neither `ingress.class` annotation nor `ingressClassName` fields are set.
  - **Accept** Ingress resource when `ingress.class` annotation is not set but `ingressClassName` field matches.
- If the `--ingress.class` argument of the controller is configured:
  - **Accept** Ingress resource when neither `ingress.class` annotation nor `ingressClassName` fields are set but controller argument `--EmptyIngressClass` is enabled.
  - **Accept** Ingress resource when --`ingress.class` argument is equal to `ingress.class` annotation.
  - **Accept** Ingress resource when `ingressClassName` field matches.
- **Ignore** Ingress resource otherwise.
