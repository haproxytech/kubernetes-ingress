# ![HAProxy](../assets/images/haproxy-weblogo-210x49.png "HAProxy")


# TCP Custom Resources

Refer to [Custom Resources](../custom-resources.md) for a general overview of Custom Resources for the Ingress Controller.

This documentation will focus on the `TCP` CR (Custom Resource) that allows more flexibility to configure a TCP service (Frontend and backend) that the previous way of doing it (a specialized TCP services Configmap).

It basically allows to configure any option available in `client-native` *frontend*, *bind* and *backend* sections.

**IMPORTANT NOTE**: The TCP ConfigMap and the Custom Resources `TCP` are not compatible.
If you use both (a TCP CR and the TCP confimap with a TCP Service on the same Address/Port), this would be lead to *random configuration*.
Please ensure when deploying `TCP` Custom Resources that no TCP configmap is present that contain a TCP service on the same Address/Port


## Difference of concept between Global/Default/Backend CRs and TCP CRs

- For Global/Default CRs:
The resouce have to be referenced via the `cr-global` annotation in the `Ingress Controller ConfigMap`.
- For Backend CRs: The resource has to be referenced via the `cr-backend` annotation in corresponding `backend service`. `cr-backend` annotation can be used also at the ConfigMap level (as default backend config for all services) or Ingress level (as a default backend config for the underlying services)

For the TCP CRs, no need to reference them.
TCP CRs are namespaced, you can deploy several of them in a namespace. They will all be applied.

## TCP CRD (Custom Resource Defintion)

The definition can be found [definitions](../crs/definition/)

Current implementation relies on the client-native library and its models to configure HAProxy.

```yaml
apiVersion: ingress.v1.haproxy.org/v1
kind: TCP
metadata:
  name: tcp-2
spec:
  - name: tcp-http-echo-445
    frontend:
      name: fe-http-echo-445
      tcplog: true
      log_format: "%{+Q}o %t %s"
      binds:
        - name: v4ssl
          #address: 1.2.3.4
          port: 32769
          ssl: true
          ssl_certificate: tcp-test-cert
        - name: v4acceptproxy
          #address: 172.0.0.2
          port: 32769
          accept_proxy: true
    service:
      name: "http-echo"
      port: 445
  - name: tcp-http-echo-444
    frontend:
      name: fe-http-echo-444
      tcplog: true
      log_format: "%{+Q}o %t %s %v"
      binds:
        - name: v4acceptproxy-2
          port: 32768
          accept_proxy: true
    service:
      name: "http-echo"
      port: 444
```

A `TCP` CR contains a list of TCP services definitions.
Each of them has:
- a `name`
- a `frontend` section that contains:
  - a `frontend`: any setting from client-native frontend model is allowed
  - a list of `binds`: any setting from client-native bind model is allowed
- a `service` defintion that is an Kubernetes upstream Service/Port (the K8s Service has to be in the same namespace as the TCP CR is deployed)


### HAProxy configuration generated for this TCP CR

#### Frontend sections


```
frontend tcpcr_test_fe-http-echo-443
  mode tcp
  bind :32766 name v4
  bind [::]:32766 name v4v6 v4v6
  log-format '%{+Q}o %t %s'
  option tcplog
  default_backend test_http-echo_https

frontend tcpcr_test_fe-http-echo-444
  mode tcp
  bind :32767 name v4acceptproxy accept-proxy
  log-format '%{+Q}o %t %s'
  option tcplog
  default_backend test_http-echo_https2
```

The frontend name `tcpcr_test_fe-http-echo-443` follow the pattern:
- tcpcr_\<namespace\>_\<tcpcr.frontend.name\>

#### Backend sections

```
backend test_http-echo_https
  mode tcp
  balance roundrobin
  no option abortonclose
  timeout server 50000
  default-server check
  server SRV_1 10.244.0.64:8443 enabled
  server SRV_2 127.0.0.1:8443 disabled
  server SRV_3 127.0.0.1:8443 disabled
  server SRV_4 127.0.0.1:8443 disabled

backend test_http-echo_https2
  mode tcp
  balance roundrobin
  no option abortonclose
  timeout server 50000
  default-server check
  server SRV_1 10.244.0.64:8443 enabled
  server SRV_2 127.0.0.1:8443 disabled
  server SRV_3 127.0.0.1:8443 disabled
  server SRV_4 127.0.0.1:8443 disabled
```

with the following Kubernetes Service and Ingress manifests:
<details>
<summary>Service</summary>

```yaml
kind: Service
apiVersion: v1
metadata:
  name: http-echo
spec:
  ports:
    - name: http
      protocol: TCP
      port: 80
      targetPort: http
    - name: https
      protocol: TCP
      port: 443
      targetPort: https
    - name: https2
      protocol: TCP
      port: 444
      targetPort: https
    - name: https3
      protocol: TCP
      port: 445
      targetPort: https
  selector:
    app: http-echo

```
</details>

<details>
<summary>Ingress</summary>

```yaml
kind: Ingress
apiVersion: networking.k8s.io/v1
metadata:
  name: http-echo
  annotations:
    ingress.class: haproxy
spec:
  rules:
    - host: "echo.haproxy.local"
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: http-echo
                port:
                  name: http
          - path: /foo_s
            pathType: Prefix
            backend:
              service:
                name: http-echo
                port:
                  name: https
          - path: /foo_s2
            pathType: Prefix
            backend:
              service:
                name: http-echo
                port:
                  name: https2

```
</details>

## How to configure the backend ?

You can use the `Backend CR` (and reference it in the Ingress Controller Configmap or the Ingress or the Service) in conjonction to the TCP CR.

For example, by adding the following Backend CR in the `test` namespace:

<details>
<summary>Backend CR</summary>

```yaml
apiVersion: ingress.v1.haproxy.org/v1
kind: Backend
metadata:
  name: mybackend
  namespace: haproxy-controller
spec:
  config:
    mode: http
    balance:
      algorithm: "leastconn"
    abortonclose: disabled
    name: toto
    default_server:
      verify: none
      resolve-prefer: ipv4
      check-sni: example.com
      sni: str(example.com)
```
</details>


The following backend section would be generated in the HAProxy configuration instead of what was explained above:

```
backend test_http-echo_https
  mode tcp
  balance leastconn
  no option abortonclose
  default-server check-sni example.com resolve-prefer ipv4 sni str(example.com) verify none
  server SRV_1 10.244.0.64:8443 enabled
  server SRV_2 127.0.0.1:8443 disabled
  server SRV_3 127.0.0.1:8443 disabled
  server SRV_4 127.0.0.1:8443 disabled

backend test_http-echo_https2
  mode tcp
  balance leastconn
  no option abortonclose
  default-server check-sni example.com resolve-prefer ipv4 sni str(example.com) verify none
  server SRV_1 10.244.0.64:8443 enabled
  server SRV_2 127.0.0.1:8443 disabled
  server SRV_3 127.0.0.1:8443 disabled
  server SRV_4 127.0.0.1:8443 disabled

```

## Collisions

2 types of collisions are detected and managed:
- collisions on frontend names
- collisions on bind address/port

In case several TCPs (*in the same namespace*) have this kind of collisions, we only apply the one that was created first based on the older CreationTimestamp of the CR.

There will also be an ERROR log
```
│ 2024/05/22 15:40:42 ERROR    handler/tcp-cr.go:61 [transactionID=e1bca8c7-8f8e-415c-b4b2-2746aa64a837] tcp-cr: skipping tcp 'test/tcp-2/tcp-http-echo-444' due to collision - Collistion FE.Name with test/tcp-1/tcp-http-echo-444
```

explaining in the TCP (in namespace `test`) named `tcp2` that a tcp service specification named `tcp-htt-echo-444` that will not be applied (in favor of the oldest one in namespace `test` in TCP CR `tcp1` named `tcp-http-echo-444`) due a collision on frontend names (`FE.Name`)

## Note on SSL

To setup SSL in a TCP CR

```yaml
apiVersion: ingress.v1.haproxy.org/v1
kind: TCP
metadata:
  name: tcp-1
spec:
  - name: tcp-http-echo-443
    frontend:
      name: fe-http-echo-443
      tcplog: true
      log_format: "%{+Q}o %t %s"
      binds:
        - name: v4
          ssl: true
          ssl_certificate: tcp-test-cert
          port: 32766
        - name: v4v6
          address: "::"
          port: 32766
          v4v6: true
    service:
      name: "http-echo"
      port: 443
```

Note that `ssl_certificate` can be:
- the name of a Kubernetes Secret (**in the same namespace as the TCP CR**) containing the certificated and key
- or a filename on the pod local filesystem
- or a folder on the pod local filesystem


It's for example possible to mount a SSL Secret in the Ingress Controller Pod on a volume and reference the volume mount path in `ssl_certificate`.
Without change the Pod (/deployment manifest), you can use a Secret name in `ssl_certificate`.
Then the cert + key will be written in the Pod filesystem in:
- `/etc/haproxy/certs/tcp`
