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
  name: tcp-1
  namespace: test
spec:
  - name: tcp-http-echo-8443
    frontend:
      name: fe-http-echo-8443
      tcplog: true
      log_format: "%{+Q}o %t %s"
      binds:
        - name: v4
          port: 32766
        - name: v4v6
          address: "::"
          port: 32766
          v4v6: true
    service:
      name: "http-echo"
      port: 8443
```

A `TCP` CR contains a list of TCP services definitions.
Each of them has:
- a `name`
- a `frontend` section that contains:
  - a `frontend`: any setting from client-native frontend model is allowed (**except the `mode` that is forced to `tcp`**)
  - a list of `binds`: any setting from client-native bind model is allowed
- a `service` defintion that is an Kubernetes upstream Service/Port (the K8s Service has to be in the same namespace as the TCP CR is deployed)

## Pod and Service definitions

with the following Kubernetes Service and Pod manifests:


```yaml
---
kind: Deployment
apiVersion: apps/v1
metadata:
  name: http-echo
  namespace: test
spec:
  replicas: 1
  selector:
    matchLabels:
      app: http-echo
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: http-echo
    spec:
      containers:
        - name: http-echo
          image: haproxytech/http-echo:latest
          imagePullPolicy: Never
          args:
          - --default-response=hostname
          ports:
            - name: http
              containerPort: 8888
              protocol: TCP
            - name: https
              containerPort: 8443
              protocol: TCP
---
kind: Service
apiVersion: v1
metadata:
  name: http-echo
  namespace: test
spec:
  ipFamilyPolicy: RequireDualStack
  ports:
    - name: http
      protocol: TCP
      port: 8888
      targetPort: http
    - name: https
      protocol: TCP
      port: 8443
      targetPort: https
  selector:
    app: http-echo
---

```


### HAProxy configuration generated for this TCP CR

#### Frontend sections


```
frontend tcpcr_test_fe-http-echo-8443
  mode tcp
  bind :32766 name v4
  bind [::]:32766 name v4v6 v4v6
  log-format '%{+Q}o %t %s'
  option tcplog
  default_backend test_http-echo_https
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
  server SRV_1 [fd00:10:244::8]:8443 enabled
  server SRV_2 10.244.0.8:8443 enabled
  server SRV_3 127.0.0.1:8443 disabled
  server SRV_4 127.0.0.1:8443 disabled
```


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

```

## Collisions

2 types of collisions are detected and managed:
- collisions on frontend names
- collisions on bind address/port

In case several TCPs (*accross all namespaces*) have this kind of collisions, we only apply the one that was created first based on the older CreationTimestamp of the CR.

For example, with using the previous `http-echo` deployement and service, and the already deplyed TCP `tcp-1` in namespace `test`, if we try to deploy the following TCP (that has a collision on Address/Port with the existing TCP `tcp-1`):
```yaml
apiVersion: ingress.v1.haproxy.org/v1
kind: TCP
metadata:
  name: tcp-2
  namespace: test
spec:
  - name: tcp-http-echo-test2-8443
    frontend:
      name: fe-http-echo-test2-8443
      tcplog: true
      log_format: "%{+Q}o"
      binds:
        - name: v4
          port: 32766
    service:
      name: "http-echo"
      port: 8443
```


There will also be an ERROR log
```
 2024/06/19 13:47:05 ERROR   handler/tcp-cr.go:61 [transactionID=dab63ebf-238d-4e04-b844-af668a86b024] tcp-cr: skipping tcp 'test/tcp-2/tcp-http-echo-test2-8443' due to collision - Colli │
│ stion AddPort :32766 with test/tcp-1/tcp-http-echo-8443
```

explaining that :
- the TCP (in namespace `test`) named `tcp2` that a tcp service specification named `tcp-htt-echo-444`
 will not be applied
 -in favor of the oldest one in namespace `test` in TCP CR `tcp1` named `tcp-http-echo-444`) due a collision on frontend names (`FE.Name`)

*This works accross all namespaces*

## Note on SSL

To setup SSL in a TCP CR (with the same Service and Pod defined above):

```yaml
apiVersion: ingress.v1.haproxy.org/v1
kind: TCP
metadata:
  name: tcp-1
  namespace: test
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
---
kind: Secret
apiVersion: v1
metadata:
  name: tcp-test-cert
  namespace: test
data:
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURvekNDQW91Z0F3SUJBZ0lVY3NtV0pSZ2dtd2hxNjVsMnRUMFBlakZKS1dFd0RRWUpLb1pJaHZjTkFRRUwKQlFBd1lURUxNQWtHQTFVRUJoTUNWVk14RFRBTEJnTlZCQWdNQkU5b2FXOHhFVEFQQmdOVkJBY01DRU52YkhWdApZblZ6TVJJd0VBWURWUVFLREFsTmVVTnZiWEJoYm5reEhEQWFCZ05WQkFNTUUyTnlaSFJqY0MxMFpYTjBMbWhoCmNISnZlSGt3SGhjTk1qUXdOVEl5TURneE1qUXlXaGNOTWpVd05USXlNRGd4TWpReVdqQmhNUXN3Q1FZRFZRUUcKRXdKVlV6RU5NQXNHQTFVRUNBd0VUMmhwYnpFUk1BOEdBMVVFQnd3SVEyOXNkVzFpZFhNeEVqQVFCZ05WQkFvTQpDVTE1UTI5dGNHRnVlVEVjTUJvR0ExVUVBd3dUWTNKa2RHTndMWFJsYzNRdWFHRndjbTk0ZVRDQ0FTSXdEUVlKCktvWklodmNOQVFFQkJRQURnZ0VQQURDQ0FRb0NnZ0VCQU14MnQzdjRvWmRaaVZmVm1mZWVabU5Sc2N5MGowUUgKWDFMSWpzQXgxMGF6RUk3cWxDL3A1TVB1Z04zSElJazFRY1RPVEpvMlNGMGluLzZQODFNUGNtNUFvZ2ZpZUhnSApVSUhkcDF0aDR0bEN1NXEzOTdLT2hHSlZBZnhINUw5WmxyTTcraHFGTnAySGJPTUtrcTU0T29hTTgzL0V5U1lMCnFPZVArdFF0MzlCSEU2eEtCd0M0YWZ1bVAyckJMdWRPNVJ5NjFyZk5SLzBzbmZMUUFYNEhERzl6YVlONHZhSmcKLzF6aVFnR0FVcnY2NFgxS2Z2WlZMTkUxdm55d2M0OHlGYlQ5L1dGQzZKYnplbjFNdzd4YmM1M09sTEhWZVNCWgphSWU4UHkvOUJKSjQvdGtHVWROV2ZKWEFEcTRGM014eDMzczJvVS9xMXhITERDZk5OUGhlVzJzQ0F3RUFBYU5UCk1GRXdIUVlEVlIwT0JCWUVGRXJBWGJBMk1nb1UzZXU1dDJXOVF2OXp5UnRBTUI4R0ExVWRJd1FZTUJhQUZFckEKWGJBMk1nb1UzZXU1dDJXOVF2OXp5UnRBTUE4R0ExVWRFd0VCL3dRRk1BTUJBZjh3RFFZSktvWklodmNOQVFFTApCUUFEZ2dFQkFIU1d2SE9WS3lEUkUrenlIbXVQeTQ0WjlyeHVwRkZUMENROTV4VnJsME9ERWo2eUJZdTlMTEE4CnBNYVNsM1kzK25ZV0U2NGpoaXFQSjdRS3l1Z2wyUHI0MUNLUWRBQ0tjbjUwRlZvRVpsQ2hiZFdPc3ZXWUtZcjQKbDlNdGFHQ0ZKTXNCWkp3SlQyZitXeHllL3U2emhyUjJVWE4zY2tSM3o4TmdhRkdtc20rYXcrUURndm5jclZ4MApmQ09aRmMreEVyTFp5RElrNEhXTXlRV3dDU1dMN1ZLWE4xVDNCMTZNd2x1MzB5OU8wWDc0UG1MNGxYZEU4ZFVrCkNRSXhBT29tT3NRek9pQ2QvRkJZSk83Smx2RC9CNjB0ZjRTZTdESWd2ZFhQZFp6MnIwSW5kS0RsR1pWcVdDdFQKM0h4R3RKZ3MrSnk3Vyt1V21vL1B4V2xkZ1hlSEwvOD0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
  tls.key: LS0tLS1CRUdJTiBQUklWQVRFIEtFWS0tLS0tCk1JSUV2Z0lCQURBTkJna3Foa2lHOXcwQkFRRUZBQVNDQktnd2dnU2tBZ0VBQW9JQkFRRE1kcmQ3K0tHWFdZbFgKMVpuM25tWmpVYkhNdEk5RUIxOVN5STdBTWRkR3N4Q082cFF2NmVURDdvRGR4eUNKTlVIRXpreWFOa2hkSXAvKwpqL05URDNKdVFLSUg0bmg0QjFDQjNhZGJZZUxaUXJ1YXQvZXlqb1JpVlFIOFIrUy9XWmF6Ty9vYWhUYWRoMnpqCkNwS3VlRHFHalBOL3hNa21DNmpuai9yVUxkL1FSeE9zU2djQXVHbjdwajlxd1M3blR1VWN1dGEzelVmOUxKM3kKMEFGK0J3eHZjMm1EZUwyaVlQOWM0a0lCZ0ZLNyt1RjlTbjcyVlN6Uk5iNThzSE9QTWhXMC9mMWhRdWlXODNwOQpUTU84VzNPZHpwU3gxWGtnV1dpSHZEOHYvUVNTZVA3WkJsSFRWbnlWd0E2dUJkek1jZDk3TnFGUDZ0Y1J5d3duCnpUVDRYbHRyQWdNQkFBRUNnZ0VBSGJrOGRLeWJ3VEdSQW5zdVJqaDVlMWdnV0RsVTE4enRQS1JQVnYxbjVhMUQKd1BoZjBUOVlDY0Vkd1hWTFNPY1J4K3lURWtWd2xsUTI4aWpoeWd5NjAvQllIZEZSNEx2S0ptR2ZZaGVFTDVWcgpTZE5vdk8vSnF3bmZTZ1QyNmlKa3JXc1FzWDVLM0VmREQrdmZrR0dHRUpLNUZncWprS3V0UXZkRUV2TVZwYm9NCmwyT3FObTNlVzV6ajhaUDluSDRCTWVFRkRDWjRremh1ZU5WTjZQSTJ1MVNQYndrVVk4NVRJUUg2STlqTW03Z2cKM0RiUS94aU14TFBmck9Rc2tYS2pOK21MSzIxYlRjS0JHOGFLZFU2NWRBdDV5RzArUElFTkM2d0pObExnZ3BZWAozcTdYVm9KU0ZsODhQOTlZTHozaFVwQmtrNnlESEtBWXRHcFZUMVFqSVFLQmdRRCtMQTl0WjYySUV1NmwzN3FBCmFQeU1janNkb1kxNm1ybTVlUUlRS041T0hSbEZHQURkMmdXL0g2Mk1vOUo2YWFQbG9zUGZQZnpFeEdPWDFiYVoKU25pTm5YYmM5Q0p0dmlXSldTbmNlVFBKNVQ4Q2F6WDJpZ2syOU1EYzg3S1BRL1VmdzJYU2pNWHEvQWNEOTUwVQpnZWVUam5JbnNUOGNHWU9KdEMvYjIrRk8wd0tCZ1FETjd5UkJPb2VGUFR3WlY2YitCQTRvbHZNZExGNEZROEVKCjg5R0YrSmp4akZ0U2FnNm1CaWdDOElaYzB5cGlhM2hwbVlKUFlZZ0tzQkR0WE5ZcWNPSjJzMzg3enNJdW9aNVgKT05GYS9KMjZPL3Z6c1FBK0RyRStFMy9QdXZRemF2YVlKUU5xREQ5NkQ4V3duVVVVV24vUjdFRnhwWVV5QzVmagpFQVl6MHlGU0NRS0JnUUNNckhZZFp6UjBDNFpwNTltaEdIb3VnVXFXcThOU0NEQ2lwb2F0eXZDKzZ2d0JjYmVKCkVoSDhKZHczNnJPamJMUjVkQXhVa2twRDNTNEI2eGFVNE5LNERsNnJDN1BDYVdyOUNZeFJxZ012eXVHRXhUR28Kc2QxSHZVN0ErMS9vU3dSd0FBVnE4dDdYbjRXQ2ZKbERzR0lyR0x1MW5EUUJxVjFUNlpaVGFPN2FZUUtCZ0J5dgppQ1JSNjlqQ2UrR24xUW9qTkhtdzlUS0dJSjZwSG5XdGNlMHdnTlY4MEtlOVFFY2VLbXFtYUlEN3BUYktjNTU2CkZLM01EekExOEZXd0RlRWhrbG9vakx1ZkJHdU1kY3IramlNWGR6MGU1K3k5SmlSKzFXK3BOYStSQWowN1ZCaEQKWjZOWkMycU1VZVJWTSs4dTRBazAyTFRrOHBYVENaaEdmaWF2N1Q5SkFvR0JBSkFvNXRhMWZHZVppQ0dMRGQwMwplQS9WMTBXVXFnMUg4NDVtQnN2RjBoSDcxZEQ5aHEvQkh2R1VPKzZHSStZU01wRklvYVBmZEVkZDMrRFhjTUVHCmM0V2FjZGhtL1V6QjV0YW5aRUFyV1JzVXV4S05vU1lMKzY3azFMT3NUOXBESnpvckVXcDRhV3RiTk5VZXIybUsKSTNQRzQ4VTd3NW82djEwNDBjWXlMT0ZaCi0tLS0tRU5EIFBSSVZBVEUgS0VZLS0tLS0K
type: kubernetes.io/tls

```



Note that `ssl_certificate` can be:
- the name of a Kubernetes Secret (**in the same namespace as the TCP CR**) containing the certificated and key
- or a filename/folder on the pod local filesystem

More details below on both use cases

**1. Using a Kubernetes Secret name**

You can use a Secret name in `ssl_certificate`.
Then the cert + key will be written in the Pod filesystem in the below paths and used from there:

| IC in cluster mode     | IC out of cluster mode (external mode) |
|------------------------|----------------------------------------|
| /etc/haproxy/certs/tcp | \<config-dir\>/certs/tcp                 |

where `<config-dir>` is:
- `/tmp/haproxy-ingress/etc` by default
- `--config-dir` IC start argument if set.




**2. Using a Folder/filename**

2-1. In cluster mode (IC Pod) : with a Kubernetes Secret

The recommanded way of using a folder (or a filename) is to mount a secret volume like below in the Ingress Controller Pod (it's possible to use `extraVolumes` and `extraVolumeMounts` in the Helm Charts):

```
spec:
  template:
    spec:
      containers:
        ...
        volumeMounts:
          - mountPath: "/var/certs"
            name: certs
            readOnly: true
      volumes:
        - name: certs
          secret:
            secretName: tcp-test-cert
```

In the TCP CR, reference the volume mount path in `ssl_certificate`:
```
ssl_certificate: /var/certs
```

**Note that storing the certificates in the Pod image and using for `ssl_certificate` a path to it, is NOT recommanded.**


2-2. External mode

Using as `ssl_certificate` with a Kubernetes Secret name as presented above in 1- also works in external mode.
It's also possibe to use a folder/filename in `external mode`, store the certificates there and reference this path as `ssl_certificate`.



### Generated Frontend and Backend configuration:


#### Frontend sections

```
frontend tcpcr_test_fe-http-echo-443
  mode tcp
  bind :32766 name v4 crt /etc/haproxy/certs/tcp/test_tcp-test-cert.pem ssl
  bind [::]:32766 name v4v6 v4v6
  log-format '%{+Q}o %t %s'
  option tcplog
  default_backend test_http-echo_https

```

#### Backend sections

```
backend test_http-echo_https
  mode tcp
  balance roundrobin
  no option abortonclose
  timeout server 50000
  default-server check
  server SRV_1 10.244.0.8:8443 enabled
  server SRV_2 [fd00:10:244::8]:8443 enabled
  server SRV_3 127.0.0.1:8443 disabled
  server SRV_4 127.0.0.1:8443 disabled
```
