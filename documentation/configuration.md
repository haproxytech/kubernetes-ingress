# ![HAProxy](../assets/images/haproxy-weblogo-210x49.png "HAProxy")

## HAProxy kubernetes ingress controller

### Avaliable options

> :information_source: Ingress and service annotations can have `ingress.kubernetes.io`, `haproxy.org` and `haproxy.com` prefixes
>
> Example: `haproxy.com/ssl-redirect` and `haproxy.org/ssl-redirect are same annotation`

| Option        | Anotation | Type | Default | Dependencies | Config map | Ingress | Service | Example |
| - | - |:-:|:-:|:-:|:-:|:-:|:-:|:-:|
| Balance Algorithm | [load-balance](#load-balance) | string | "roundrobin" |  |:large_blue_circle:|:white_circle:|:large_blue_circle:||
| Force Https | [ssl-redirect](#ssl-redirect) | bool | "true" | [tls-secret](#tls-secret) |:large_blue_circle:|:white_circle:|:white_circle:||
| X-Forwarded-For | [forwarded-for](#forwarded-for) | ["enabled", "disabled"] | "enabled" |  |:large_blue_circle:|:white_circle:|:large_blue_circle:||

### Options

#### forwarded-for

- by default enabled, can be disabled per service

#### load-balance

- use in format  `haproxy.org/load-balance: <algorithm> [ <arguments> ]`

#### ssl-redirect

- by default this is activated if tls key is provided

### Secrets

#### tls-secret

- tls-secret contains two items:
  - tls.crt
  - tls.key