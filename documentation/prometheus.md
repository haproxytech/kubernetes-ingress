# Prometheus

## Activation

To enable prometheus, simply add the `--prometheus` flag to the command line arguments of the ingress controller. This will activate the prometheus endpoints in the ingress controller. Its address is the same as the ingress controller and the path is `/metrics`. The listen port is defined by the option `--controller-port` which is set to 6060 by default. So ,the expected URL is:
```
http(s)://<ingress controller ip address or DNS name>:<controller port>/metrics
```

## Security

You can enable a basic authentication access to restrict the access to the prometheus endpoint. To this aim, create a Secret with the pairs of authorized users and their passwords. These passwords must first have been encrypted with a tool like `mkpasswd`. Example:
```
$ mkpasswd -m SHA-256
```
Then point the controller to this secret by adding the `prometheus-endpoint-auth-secret` data to the controller configmap. Example:
```
prometheus-endpoint-auth-secret: haproxy-controller/prometheus-credentials
```
