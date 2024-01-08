# Custom Resources

- In order to use custom resources, you will need to apply/update resource [definitions](../crs/definition/)
- Custom Resources are used by Ingress Controller to implement HAProxy concepts like (backend, frontend, http rules, etc) which are all available under the `core.haproxy.org` API.
- Current implementation relies on the [client-native](https://github.com/haproxytech/client-native) library and its [models](https://github.com/haproxytech/client-native/tree/master/models) to [configure HAProxy](https://cbonte.github.io/haproxy-dconv/2.4/configuration.html#4.1).
- Custom resources are meant to **replace annotations** when possible. So they will have **precedance** when used.
  *Example:* if the backend resource is used no backend annotation will be processed which means a backend cannot be configured by mixing both the backend resource and backend annotations.

## HAProxy concepts
- Only HAProxy directives available in the resource [definitions](../crs/definition/) are supported, contributions and github requests to support new directives are welcome.
- All timeout fields are integer input interpreted as time in **ms**.

### Global
The Global resource is used to configure the HAProxy global section by referencing the resouce via the `cr-global` annotation in the Ingress Controller ConfigMap.

*Example:*

1. Define a global resource
```yaml
apiVersion: "core.haproxy.org/v1alpha1"
kind: Global
metadata:
  name: myglobal
  namespace: haproxy-controller
spec:
  config:
    maxconn: 1000
    stats_timeout: 36000
    tune_ssl_default_dh_param: 2048
    ssl_default_bind_options: "no-sslv3 no-tls-tickets no-tlsv10"
    ssl_default_bind_ciphers:
    ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-ECDSA-AES256-GCM-SHA384:ECDHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-AES256-GCM-SHA384:DHE-RSA-AES128-GCM-SHA256:DHE-DSS-AES128-GCM-SHA256:kEDH+AESGCM:ECDHE-RSA-AES128-SHA256:ECDHE-ECDSA-AES128-SHA256:ECDHE-RSA-AES128-SHA:ECDHE-ECDSA-AES128-SHA:ECDHE-RSA-AES256-SHA384:ECDHE-ECDSA-AES256-SHA384:ECDHE-RSA-AES256-SHA:ECDHE-ECDSA-AES256-SHA:DHE-RSA-AES128-SHA256:DHE-RSA-AES128-SHA:DHE-DSS-AES128-SHA256:DHE-RSA-AES256-SHA256:DHE-DSS-AES256-SHA:DHE-RSA-AES256-SHA:AES256-GCM-SHA384:AES128-SHA256:AES256-SHA256:AES128-SHA:AES256-SHA:AES:CAMELLIA:!aNULL:!eNULL:!EXPORT:!DES:!RC4:!MD5:!PSK:!aECDH:!EDH-DSS-DES-CBC3-SHA:!EDH-RSA-DES-CBC3-SHA:!KRB5-DES-CBC3-SHA:!3DES
    hard_stop_after: 30000
    server_state_base: /tmp/haproxy-ingress/state
    runtime_apis:
      - address: "0.0.0.0:31024"
```

2. Apply it:
```
$ kubectl apply -f myglobal.yaml
```

3. Update the ConfigMap
```
apiVersion: v1
kind: ConfigMap
metadata:
  name: kubernetes-ingress
  namespace: haproxy-controller
data:
  cr-global: haproxy-controller/myglobal
```

### Global CRD: Note of versions compatibility between CRD and haproxy

The `ingress.v1.haproxy.org/Global` CRD `version v1`  is using client-native v5 that contains `haproxy 2.9`` keywords.

An annototation in the CRD is available to specify the version of client-native used: `haproxy.org/client-native`

Ingress Controller is deployed with `haproxy 2.8`.
Note that the following fields of the CRD are `haproxy 2.9` keywords and cannot be used with this version of Ingress Controller, even if defined the `Globals` CRD:
- `runtime_api.quic-socket`
- `tune_options.events_max_events_at_once`
- `tune_options.max_checks_per_thread`
- `tune_options.rcvbuf_backend`
- `tune_options.rcvbuf_frontend`
- `tune_options.sndbuf_backend`
- `tune_options.sndbuf_frontend`
- `tune_options.zlib_memlevel`
- `tune_options.zlib_windowsize`


### Defaults
The Defaults resource is used to configure the HAProxy defaults section by referencing the resouce via the `cr-defaults` annotation in the Ingress Controller ConfigMap.

*Example:*

1. Define a defaults resource
```yaml
apiVersion: "core.haproxy.org/v1alpha1"
kind: Defaults
metadata:
  name: mydefaults
  namespace: default
spec:
  config:
    log_format: "'%ci:%cp [%tr] %ft %b/%s %TR/%Tw/%Tc/%Tr/%Ta %ST %B %CC %CS %tsc %ac/%fc/%bc/%sc/%rc %sq/%bq %hr %hs \"%HM %[var(txn.base)] %HV\"'"
    redispatch:
      enabled: enabled
      interval: 0
    dontlognull: enabled
    http_connection_mode: http-keep-alive
    http_request_timeout: 5000
    connect_timeout: 5000
    client_timeout: 50000
    queue_timeout: 5000
    server_timeout: 50000
    tunnel_timeout: 3600000
    http_keep_alive_timeout: 60000
```

2. Apply it:
```
$ kubectl apply -f mydefaults.yml
```

3. Update the ConfigMap
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: kubernetes-ingress
  namespace: haproxy-controller
data:
  cr-global: haproxy-controller/myglobal
  cr-defaults: haproxy-controller/mydefaults
```


### Backend
The Backend resource is used to configure the HAProxy backend section by referencing the resouce via the `cr-backend` annotation in corresponding backend service.
`cr-backend` annotation can be used also at the ConfigMap level (as default backend config for all services) or Ingress level (as a default backend config for the underlying services)

*Example:*

1. Define a backend resource
```yaml
apiVersion: "core.haproxy.org/v1alpha1"
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
    default_server:
      verify: none
      resolve-prefer: ipv4
      check-sni: example.com
      sni: str(example.com)
```

2. Apply it:
```
$ kubectl apply -f mybackend.yaml
```

3. Annotate the corresponding service
```yaml
apiVersion: v1
kind: Service
metadata:
  name: example
  namespace: external
  annotations:
    cr-backend: haproxy-controller/mybackend
spec:
  type: ExternalName
  externalName: example.com
  ports:
  - protocol: TCP
    port: 443
    name: https
    targetPort: 443
```
