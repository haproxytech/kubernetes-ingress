# ![HAProxy](../assets/images/haproxy-weblogo-210x49.png "HAProxy")

## HAProxy kubernetes ingress controller

Options for starting controller can be found in [controller.md](controller.md)

### Available annotations

> :information_source: Ingress and service annotations can have `ingress.kubernetes.io`, `haproxy.org` and `haproxy.com` prefixes
>
> Example: `haproxy.com/ssl-redirect` and `haproxy.org/ssl-redirect` are same annotation

| Annotation | Type | Default | Dependencies | Config map | Ingress | Service |
| - |:-:|:-:|:-:|:-:|:-:|:-:|
| [check](#backend-checks) | ["enabled"] | "enabled" |  |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|
| [check-interval](#backend-checks) | [time](#time) |  | [check](#backend-checks) |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|
| [forwarded-for](#x-forwarded-for) | ["enabled", "disabled"] | "enabled" |  |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|
| [ingress.class](#ingress-class) | string | "" |  |:white_circle:|:large_blue_circle:|:white_circle:|
| [load-balance](#balance-algorithm) | string | "roundrobin" |  |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|
| [maxconn](#maximum-concurent-connections) | number |  |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [nbthread](#number-of-threads) | number | |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [pod-maxconn](#maximum-concurent-backend-connections) | number |  |  |:white_circle:|:white_circle:|:large_blue_circle:|
| [rate-limit](#rate-limit) | "ON"/"OFF" | "OFF" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [rate-limit-expire](#rate-limit) | string | "30m" | [rate-limit](#rate-limit) |:large_blue_circle:|:white_circle:|:white_circle:|
| [rate-limit-interval](#rate-limit) | string | "10s" | [rate-limit](#rate-limit) |:large_blue_circle:|:white_circle:|:white_circle:|
| [rate-limit-size](#rate-limit) | string | "100k" | [rate-limit](#rate-limit) |:large_blue_circle:|:white_circle:|:white_circle:|
| [servers-increment](#servers-slots-increment) | number | "42" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [ssl-certificate](#tls-secret) | string |  |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [ssl-redirect](#https) | "ON"/"OFF" | "ON" | [tls-secret](#tls-secret) |:large_blue_circle:|:white_circle:|:white_circle:|
| [ssl-redirect-code](#https) | [301, 302, 303] | "302" | [tls-secret](#tls-secret) |:large_blue_circle:|:white_circle:|:white_circle:|
| [timeout-http-request](#timeouts) | [time](#time) | "5s" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [timeout-check](#timeouts) | [time](#time) |  |  |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|
| [timeout-connect](#timeouts) | [time](#time) | "5s" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [timeout-client](#timeouts) | [time](#time) | "50s" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [timeout-queue](#timeouts) | [time](#time) | "5s" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [timeout-server](#timeouts) | [time](#time) | "50s" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [timeout-tunnel](#timeouts) | [time](#time) | "1h" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [timeout-http-keep-alive](#timeouts) | [time](#time) | "1m" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [whitelist](#whitelist) | [IPs or CIDRs](#whitelist) | "" |  |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|
| [whitelist-with-rate-limit](#whitelist) | "ON"/"OFF" | "OFF" |  |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|

> :information_source: Annotations have hierarchy: `default` <- `Configmap` <- `Ingress` <- `Service`
>
> Service annotations have highest priority. If they are not defined, controller goes one level up until it finds value.
>
> This is usefull if we want, for instance, to change default behaviour, but want to keep default for some service. etc.

### Options

#### Balance Algorithm

- Annotation: `load-balance`
- use in format  `haproxy.org/load-balance: <algorithm> [ <arguments> ]`

#### Backend Checks

- Annotation: `check` - activate pod check
- Annotation: `check-interval` - interval between checks [`check` must be "enabled"]

#### Ingress Class

- Annotation: `ingress.class`
  - default: ""
  - used to monitor specific ingress objects in multiple controllers environment
  - any ingress object which have class specified and its different from one defined in [image arguments](controller.md) will be ignored

#### Https

- Annotation `ssl-redirect`
  - by default this is activated if tls key is provided
  - redirects http trafic to https
  - default `ON`, can be set to "OFF" to disable
- Annotation `ssl-redirect-code`
  - HTTP status code on redirect

#### Maximum Concurent Connections

- Annotation: `maxconn`

#### Maximum Concurent Backend Connections

- Annotation: `pod-maxconn`
- related to backend servers (pods)

#### Number of threads

- Annotation: `nbthread`
- default value is number of procesors available

#### Rate limit

Keep in mind this setting is global and will applied to all your traffic.
The number of requests a client can do per `rate-limit-interval` is **10**.

- Annotation: `rate-limit`
  - `ON` / `OFF` - enable or disable rate limiting
- Annotation: `rate-limit-expire`
  - Table entries expire after `rate-limit-expire` of inactivity.
- Annotation: `rate-limit-interval`
  - request rate for the last `rate-limit-interval`
- Annotation: `rate-limit-size`
  - number of ip entries in table

#### Servers slots increment

- Annotation `servers-increment`- determines how much backend servers should we
        put in `maintenance` mode so controller can
        dynamically insert new pods without hitless reload

#### Timeouts

- Annotation `timeout-http-request`
- Annotation [`timeout-check`](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#timeout%20check)
- Annotation `timeout-connect`
- Annotation `timeout-client`
- Annotation `timeout-queue`
- Annotation `timeout-server`
- Annotation `timeout-tunnel`
- Annotation `timeout-http-keep-alive`

#### X-Forwarded-For

- Annotation: `forwarded-for`
- by default enabled, can be disabled per service or globally

#### Whitelist

- Annotation: `whitelist`
- by default disabled
- `IPs or CIDR` - coma or space separated list of IP addresses or CIDRs
- :information_source: service annotation will override ingress one that overrides config map annotation
- Annotation: `whitelist-with-rate-limit`
  - apply rate-limiting, but exclude addresses from whitelist

### Secrets

#### tls-secret

- define through pod arguments
  - `--default-ssl-certificate`=\<namespace\>/\<secret\>
- Annotation `ssl-certificate` in config map
  - \<namespace\>/\<secret\>
- single certificate secret can contain two items:
  - tls.key
  - tls.crt
- certificate secret with `rsa` and `ecdsa` certificates:
  - :information_source: only one certificate is also acceptable setup
  - rsa.key
  - rsa.crt
  - ecdsa.key
  - ecdsa.crt

### Data types

#### Port

- value between <0, 65535]

#### Time

- number + type
- in miliseconds, "s" suffix denotes seconds
- example: "1s"
