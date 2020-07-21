# ![HAProxy](../assets/images/haproxy-weblogo-210x49.png "HAProxy")

## HAProxy kubernetes ingress controller

you can run image with arguments:

- `--configmap`
  - optional, must be in format `namespace/name`
  - default `default/haproxy-configmap`
- `--configmap-tcp-services`
  - optional, must be in format `namespace/name`
  - Example:
   ```
   apiVersion: v1
   kind: ConfigMap
   metadata:
     name: tcp
     namespace: default
   data:
     3306:              # Port where the frontend is going to listen to.
       tcp/mysql:3306   # Kuberntes service to use for the backend.
     389:
       tcp/ldap:389:ssl # ssl option will enable ssl offloading for target service.
     6379:
       tcp/redis:6379
   ```
  - Ports of TCP services should be exposed on the controller's kubernetes service
- `--configmap-errorfile`
  - optional, must be in format `{errorcode}.http`
  - Example:
    ```yaml
    apiVersion: v1
    kind: ConfigMap
    metadata:
      name: errorfile
      namespace: default
    data:
      503.http: |-
        HTTP/1.0 503 Service Unavailable
        Cache-Control: no-cache
        Connection: close
        Content-Type: text/html
    
        <html><body><h1>Oops, that's embarassing!</h1>
        There are no servers available to handle your request.
        </body></html>
    ```
  - The controller will ensure to write these custom error file in the location
    `/etc/haproxy/errors`, no need to mount the _ConfigMap_ into the Pod
    filesystem.
- `--default-backend-service`
  - must be in format `namespace/name`
- `--default-ssl-certificate`
  - optional, must be in format `namespace/name`
  - default: ""
- `--ingress.class`
  - default: ""
  - class of ingress object to monitor in multiple controllers environment
- `--namespace-whitelist`
  - optional, if listed only selected namespaces will be monitored
  - namespace of the configmap is always on the list of selected/whitelisted namespaces.
  - :information_source: `namespace-whitelist` has priority over blacklisting.
  - if we need to monitor more than one namespace add it multiple times:
  
    ```bash
    --namespace-whitelist=default
    --namespace-whitelist=namespace1
    --namespace-whitelist=namespace2
    ```

- `--namespace-blacklist`
  - optional, if listed selected namespaces will be excluded
  - usage: same as whitelisting

- `--publish-service`
  - optional, must be in format `namespace/name`
  - The controller mirrors the address of the service's endpoints to the load-balancer status of all Ingress objects it satisfies.

- `--cache-resync-period`
  - optional (must adhere to [`time.Duration`](https://golang.org/pkg/time/#ParseDuration) format),
    sets the default re-synchronization period at which the controller will re-apply the desired state
  - fine tuning parameter useful for large scale deployments, as reported in the [issue #216](https://github.com/haproxytech/kubernetes-ingress/issues/216)
  - default value to `10m` (_10 minutes_)

- `--sync-period`
  - optional (must adhere to [`time.Duration`](https://golang.org/pkg/time/#ParseDuration) format),
    sets the synchronization period at which the controller executes the configuration sync
  - default value to `5s` (_5 seconds_)

- `--log`
  - optional
  - select log level that application outputs
  - default: `info`
  - available options: `error`, `warning`, `info`, `debug`, `trace`
    
