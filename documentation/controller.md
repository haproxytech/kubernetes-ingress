# ![HAProxy](../assets/images/haproxy-weblogo-210x49.png "HAProxy")

## HAProxy kubernetes ingress controller

you can run image with arguments:

- `--default-backend-service`
  - optional, must be in format `namespace/name`
  - by default HAProxy serves http 400 bad request status
- `--default-ssl-certificate`
  - optional, must be in format `namespace/name`
  - default `default/tls-secret`
- `--configmap=`
  - optional, must be in format `namespace/name`
  - default `default/haproxy-configmap`