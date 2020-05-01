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

- `--sync-period`
  - optional (must adhere to [`time.Duration`](https://golang.org/pkg/time/#ParseDuration) format),
    sets the synchronization period at which the controller executes the configuration sync
  - default value to `5s` (_5 seconds_)
