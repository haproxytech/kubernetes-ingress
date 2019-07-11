# ![HAProxy](../assets/images/haproxy-weblogo-210x49.png "HAProxy")

## HAProxy kubernetes ingress controller

you can run image with arguments:

- `--configmap`
  - mandatory, must be in format `namespace/name`
  - default `default/haproxy-configmap`
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
  - usage: same as whitellisting
