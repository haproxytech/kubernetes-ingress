
# ![HAProxy](../assets/images/haproxy-weblogo-210x49.png "HAProxy")

## Gateway API

Current supported version is 0.5.1 - we currently only support TCP Route

### Getting started

The set of levels of requirements for a feature (resource or fields of a resource) is located [here](https://gateway-api.sigs.k8s.io/concepts/conformance/#2-support-levels).
The support level for each resource is :
| Resource | Support | Comment|
|---|---|---|
| GatewayClass | Partially supported | All but ParametersRef|
| Gateway | Supported | All but Addresses (extended) and Status |
| TCPRoute | Supported | All but Status |
| ReferenceGrant |  supported| |

the easiest way of testing the feature is to run `make example-experimental-gwapi`.

This will install all resources and and a simple service `http-echo` that is accessible both via classic ingress and via TCP route defined with Gateway API

### Step By Step

Go to [github.com v0.5.1](https://github.com/kubernetes-sigs/gateway-api/releases/tag/v0.5.1) and download [experimental-install.yaml](https://github.com/kubernetes-sigs/gateway-api/releases/download/v0.5.1/experimental-install.yaml) file

Or use one provided in this repository

```bash
kubectl apply -f deploy/tests/config/experimental/gwapi.experimental.yaml
```

keep in mind this needs to be installed first and prior to any other resource

## Resources

next step is to install other resources:

```bash
kubectl apply -f deploy/tests/config/experimental/gwapi-resources.yaml
kubectl apply -f deploy/tests/config/experimental/gwapi-echo-app.yaml
kubectl apply -f deploy/tests/config/experimental/gwapi-rbac.yaml
```

or we can go step by step

### GatewayClass

gatewayclass is more or less the counterpart of ingressclass from ingresscontroller. Its main responsibility is to identify which gateways should be managed by an instance of controller.

The controllerName field is used as an identifier which should match an id given to the gateway controller to determine if this gatewayclass is ok to handle.

```bash
echo '
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: GatewayClass
metadata:
  name: haproxy-gwc
spec:
  controllerName: haproxy.org/gateway-controller' | kubectl apply -f -
```

A new parameter has been added to the current implementation of HAProxy kubernetes controller. This parameter is named `gateway-controller-name` and should be added as a command line parameter like:

```bash
--gateway-controller-name=haproxy.org/gateway-controller
```

### Gateway

The gateway holds all connectivity configuration for listeners. A gateway listener can be seen as a frontend in HAProxy world. They must be linked to a gatewayclass to determine whether it should be handled by a specific instance of a controller or not. In the following gateway, you can see the
gatewayClassName pointing to the previous defined gatewayclass. If the controller got the corresponding parameter it will handle this gateway.

```bash
echo '
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: Gateway
metadata:
  name: gateway1
  namespace: default
spec:
  gatewayClassName: haproxy-gwc
  listeners:
    - allowedRoutes:
        kinds:
          - group: gateway.networking.k8s.io
            kind: TCPRoute
        namespaces:
          from: All
      name: listener1
      port: 8000
      protocol: TCP' | kubectl apply -f -
```

Listener configures the connectivity but also how a route, i.e. a backend, could attach to it. Please note that it is a generic data. It's used for HTTP and TCP routes. Thus some fields, like hostname or tls, are related to HTTP only and not used for TCP. The allowedRoutes offers a mix of namespace and kind of resources check. The namespace check offers two simple options and one more complex. It can allow attachment of resources from "all" or "same" namespace(s) but also only from namespace presenting some labels in complex combinations.
Note that the resource could be in theory of any kind, this gives an hint of possible extensions in the future.

### ReferenceGrant

To improve security and solidity inside the cluster, a resource implements the authorization for a resource to refer to an other one in an other namespace. This enforces the namespace boundaries inside the clusters for security and consistency sakes. The ReferenceGrant defines the allowed references from a certain kind of resource in a specific namespace to a certain kind of resource in the same namespace as the ReferenceGrant and potentially named. ReferenceGrant are used with backendRefs from TCPRoute.

```bash
echo '
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: ReferenceGrant
metadata:
  name: refgrantns1
  namespace: default
spec:
  from:
    - group: "gateway.networking.k8s.io"
      kind: "TCPRoute"
      namespace: default
  to:
    - group: ""
      kind: "Service"' | kubectl apply -f -
```

### RBAC

Same as with all other resources RBAC is also needed,
we can create a new Cluster Role

```bash
echo '
kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
 name: haproxy-kubernetes-ingress-gwapi
rules:
 - apiGroups:
    - "gateway.networking.k8s.io"
   resources:
   - referencegrants
   - gateways
   - gatewayclasses
   - tcproutes
   verbs:
    - get
    - list
    - watch
 - apiGroups:
    - "gateway.networking.k8s.io"
   resources:
    - gatewayclasses/status
    - gateways/status
    - tcproutes/status
   verbs:
    - update' | kubectl apply -f -
```

and with ClusterRoleBinding we can connect it to already existing service account

```bash
echo '
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: haproxy-kubernetes-ingress-gwapi
  namespace: haproxy-controller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: haproxy-kubernetes-ingress-gwapi
subjects:
- kind: ServiceAccount
  name: haproxy-kubernetes-ingress
  namespace: haproxy-controller' | kubectl apply -f -
```

this can also be added to standard ClusterRole for ingress controller directly, but since Gateway API is in experimental phase,
its better to have a separation

### TCPRoute

A TCPRoute manages the relation between a collection of backend servers and a collection of  listeners, i.e frontends. The collection of backend servers is managed with backendRefs which impersonates the backend servers. As for the parentRefs they refer to the attachment destinations. Currently the only resource required to be supported is the gateway. But it could also be extended in the future.

```bash
echo '
apiVersion: gateway.networking.k8s.io/v1alpha2
kind: TCPRoute
metadata:
  name: route1
  namespace: default
spec:
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: gateway1
      namespace: default
  rules:
    - backendRefs:
        - group: ''
          kind: Service
          name: http-echo
          namespace: default
          port: 80
          weight: 13' | kubectl apply -f -
```
