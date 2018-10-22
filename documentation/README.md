# ![HAProxy](../assets/images/haproxy-weblogo-210x49.png "HAProxy")

## HAProxy kubernetes ingress controller

Options for starting controller can be found in [controller.md](controller.md)

### Avaliable anotations

> :information_source: Ingress and service annotations can have `ingress.kubernetes.io`, `haproxy.org` and `haproxy.com` prefixes
>
> Example: `haproxy.com/ssl-redirect` and `haproxy.org/ssl-redirect are same annotation`

| Anotation | Type | Default | Dependencies | Config map | Ingress | Service | Example |
| - |:-:|:-:|:-:|:-:|:-:|:-:|:-:|
| [load-balance](#balance-algorithm) | string | "roundrobin" |  |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:||
| [maxconn](#maximum-concurent-connections) | number | "2000" |  |:large_blue_circle:|:white_circle:|:white_circle:||
| [pod-maxconn](#maximum-concurent-backend-connections) | number | "2000" |  |:white_circle:|:white_circle:|:large_blue_circle:||
| [ssl-redirect](#force-https) | bool | "true" | [tls-secret](#tls-secret) |:large_blue_circle:|:white_circle:|:white_circle:|:construction:|
| [forwarded-for](#x-forwarded-for) | ["enabled", "disabled"] | "enabled" |  |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:||

### Options

#### Balance Algorithm

- Annotation: `load-balance`
- use in format  `haproxy.org/load-balance: <algorithm> [ <arguments> ]`

#### Force Https

- Annotation: `ssl-redirect`
- by default this is activated if tls key is provided

#### Maximum Concurent Connections

- Annotation: `maxconn`
- by default this is set to 2000

#### Maximum Concurent Backend Connections

- Annotation: `pod-maxconn`
- related to backend servers (pods)
- by default this is set to 2000 for every backend server (pod)

#### X-Forwarded-For

- Annotation: `forwarded-for`
- by default enabled, can be disabled per service or globally

### Secrets

#### tls-secret

- tls-secret contains two items:
  - tls.crt
  - tls.key