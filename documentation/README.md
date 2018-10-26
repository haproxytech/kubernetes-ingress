# ![HAProxy](../assets/images/haproxy-weblogo-210x49.png "HAProxy")

## HAProxy kubernetes ingress controller

Options for starting controller can be found in [controller.md](controller.md)

### Avaliable anotations

> :information_source: Ingress and service annotations can have `ingress.kubernetes.io`, `haproxy.org` and `haproxy.com` prefixes
>
> Example: `haproxy.com/ssl-redirect` and `haproxy.org/ssl-redirect are same annotation`

| Anotation | Type | Default | Dependencies | Config map | Ingress | Service |
| - |:-:|:-:|:-:|:-:|:-:|:-:|
| [check](#backend-checks) | ["enabled"] | "enabled" |  |:large_blue_circle:|:white_circle:|:large_blue_circle:|
| :construction: [check-port](#backend-checks) | [port](#port) |  | [check](#backend-checks) |:white_circle:|:white_circle:|:large_blue_circle:|
| :construction: [check-interval](#backend-checks) | [time](#time) |  | [check](#backend-checks) |:large_blue_circle:|:white_circle:|:large_blue_circle:|
| [healthz](#healthz-check) | ["enabled"] | "enabled" | |:large_blue_circle:|:white_circle:|:white_circle:|
| [healthz-port](#healthz-check) | ["enabled"] | "1042" | [healtz](#healthz-check) |:large_blue_circle:|:white_circle:|:white_circle:|
| [load-balance](#balance-algorithm) | string | "roundrobin" |  |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|
| [maxconn](#maximum-concurent-connections) | number | "2000" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [pod-maxconn](#maximum-concurent-backend-connections) | number | "2000" |  |:white_circle:|:white_circle:|:large_blue_circle:|
| [servers-increment](#servers-slots-increment) | number | "42" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [servers-increment-max-disabled](#servers-slots-increment) | number | "66" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| :construction: [ssl-redirect](#force-https) | bool | "true" | [tls-secret](#tls-secret) |:large_blue_circle:|:white_circle:|:white_circle:|
| [forwarded-for](#x-forwarded-for) | ["enabled", "disabled"] | "enabled" |  |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|

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
- :construction: Annotation: `check-port` - port to use when checking [`check` must be "enabled"]
- :construction: Annotation: `check-interval` - interval between checks [`check` must be "enabled"]
- use in format  `haproxy.org/load-balance: <algorithm> [ <arguments> ]`

#### Force Https

- Annotation: `ssl-redirect`
- by default this is activated if tls key is provided

#### Healthz Check

- Annotation: `healthz` - enable or disable service
- Annotation: `healthz-port` - port where HAProxy responds to checks
- default "enabled"- haproxy responds to health checks

#### Maximum Concurent Connections

- Annotation: `maxconn`
- by default this is set to 2000

#### Maximum Concurent Backend Connections

- Annotation: `pod-maxconn`
- related to backend servers (pods)
- by default this is set to 2000 for every backend server (pod)

### Servers slots increment
- Annotation `servers-increment`- determines how much backend servers should we 
        put in `maintenance` mode so controller can 
        dynamically insert new pods without hitless reload
- Annotation `servers-increment-max-disabled` - maximum allowed number of 
        disabled servers in backend. Greater number triggers HAProxy reload

#### X-Forwarded-For

- Annotation: `forwarded-for`
- by default enabled, can be disabled per service or globally

### Secrets

#### tls-secret

- tls-secret contains two items:
  - tls.crt
  - tls.key

### Data types

#### Port

- value between <0, 65535]

#### Time

- number + type
- in miliseconds, "s" suffix denotes seconds
- example: "1s"