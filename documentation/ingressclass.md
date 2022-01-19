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

## Ingress eligibility rules

### Inputs

#### Controller
- [`--ingress.class`](./controller.md#--ingressclass): CLI param of the Ingress Controller to select Ingress resources.
- [`--EmptyIngressClass`](./controller.md#--empty-ingress-class):  CLI param of the Ingress Controller to select Ingress resources with ingress class config.

#### Ingress resource
- [`ingress.class`](https://kubernetes.io/docs/concepts/services-networking/ingress/#deprecated-annotation):  annotation which can be set in the ingress resource.
- [`ingressClassName`](https://kubernetes.io/docs/concepts/services-networking/ingress/#deprecated-annotation): field in the ingress resource that reference a kubernetes IngressClass.

#### IngressClass resource
- [`is-default-class`](https://kubernetes.io/docs/concepts/services-networking/ingress/#default-ingress-class): annotation in ingressClass resource.
- [`controller`](https://kubernetes.io/docs/concepts/services-networking/ingress/#ingress-class): field in the ingressClass resource.

### Prerequisites
A **matching default IngressClass** is true when a **valid IngressClass** resource is available in the cluster and its `is-default-class` annotation is enabled.  
A **matching IngressClass** is true when the `ingressClassName` field of an Ingress resource points to a **valid IngressClass**.  
A **valid IngressClass** is true when the `controller` field of the IngressClass is equal to:  
- "*haproxy.org/ingress-controller*" if `--ingress.class` argument is empty
- "*haproxy.org/ingress-controller/<--ingress.class value>*" if `--ingress.class` argument is not empty.

### Rules
- If the `--ingress.class` controller param is not set (empty):
  - **Accept** Ingress resource with:
	  - no `ingress.class` annotation
	  - no `ingressClassName` field.
	  - no IngressClass resource with `is-default-class` enabled.
  - **Accept** Ingress resource with:
	  - no `ingress.class` annotation
	  - no `ingressClassName` field.
	  - a "matching default IngressClass"
  - **Accept** Ingress resource with:
	  - no `ingress.class` annotation
	  - a "matching IngressClass"
- If the `--ingress.class` controller param is set (not empty):
  - **Accept** Ingress resource with:
	  - an `ingress.class` annotation equal to the `--ingress.class` argument.
  - **Accept** Ingress resource with:
	  - no `ingress.class` annotation
	  - no `ingressClassName` field.
	  - `--EmptyIngressClass` controller param is enabled.
  - **Accept** Ingress resource  with:
	  - no `ingress.class` annotation
	  - no `ingressClassName` field.
	  - a "matching default IngressClass"
  - **Accept** Ingress resource  with:
	  - no `ingress.class` annotation
	  - a "matching IngressClass"
- **Ignore** Ingress resource otherwise.
