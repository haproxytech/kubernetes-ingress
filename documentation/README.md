# ![HAProxy](../assets/images/haproxy-weblogo-210x49.png "HAProxy")

## HAProxy kubernetes ingress controller

Options for starting controller can be found in [controller.md](controller.md)

### Available annotations

> :information_source: Ingress and service annotations can have `ingress.kubernetes.io`, `haproxy.org` and `haproxy.com` prefixes
>
> Example: `haproxy.com/ssl-redirect` and `haproxy.org/ssl-redirect` are same annotation

| Annotation | Type | Default | Dependencies | Config map | Ingress | Service |
| - |:-:|:-:|:-:|:-:|:-:|:-:|
| [blacklist](#access-control) | [IPs or CIDRs](#access-control) | "" |  |:large_blue_circle:|:large_blue_circle:|:white_circle:|
| [check](#backend-checks) | ["true", "false"] | "true" |  |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|
| [check-http](#backend-checks) | string |  | [check](#backend-checks) |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|
| [check-interval](#backend-checks) | [time](#time) |  | [check](#backend-checks) |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|
| [cookie-persistance](#cookie-persistance) | string | "" |  |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|
| [dontlognull](#logging) | ["true", "false"] | "true" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [forwarded-for](#x-forwarded-for) | ["true", "false"] | "true" |  |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|
| [ingress.class](#ingress-class) | string | "" |  |:white_circle:|:large_blue_circle:|:white_circle:|
| [http-keep-alive](#http-options) | ["true", "false"] | "true" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [http-server-close](#http-options) | ["true", "false"] | "false" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [load-balance](#balance-algorithm) | string | "roundrobin" |  |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|
| [log-format](#log-format) | string |  |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [logasap](#logging) | ["true", "false"] | "false" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [maxconn](#maximum-concurent-connections) | number |  |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [nbthread](#number-of-threads) | number | |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [path-rewrite](#path-rewrite) | string |  |  |:large_blue_circle:|:large_blue_circle:|:white_circle:|
| [pod-maxconn](#maximum-concurent-backend-connections) | number |  |  |:white_circle:|:white_circle:|:large_blue_circle:|
| [proxy-protocol](#proxy-protocol) | [IPs or CIDRs](#proxy-protocol) |   |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [rate-limit-period](#rate-limit) | [time](#time)| 1s |  |:large_blue_circle:|:large_blue_circle:|:white_circle:|
| [rate-limit-requests](#rate-limit) | number |  |  |:large_blue_circle:|:large_blue_circle:|:white_circle:|
| [rate-limit-size](#rate-limit) | string | "100k" | [rate-limit](#rate-limit) |:large_blue_circle:|:white_circle:|:white_circle:|
| [request-capture](#request-capture) | [sample expression](#sample-expression) |  |  |:large_blue_circle:|:large_blue_circle:|:white_circle:|
| [request-capture-len](#request-capture) | number | 128 |  |:large_blue_circle:|:large_blue_circle:|:white_circle:|
| [request-set-header](#request-set-header) | string |  |  |:large_blue_circle:|:large_blue_circle:|:white_circle:|
| [response-set-header](#response-set-header) | string |  |  |:large_blue_circle:|:large_blue_circle:|:white_circle:|
| [server-ssl](#server-ssl) | ["true", "false"] | "false" |  |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|
| [set-host](#set-host) | string |  |  |:large_blue_circle:|:large_blue_circle:|:white_circle:|
| [servers-increment](#servers-slots-increment) | number | "42" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [ssl-certificate](#tls-secret) | string |  |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [ssl-passthrough](#https) | ["true", "false"] | "false" |  |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|
| [ssl-redirect](#https) | "true"/"false" | "false" | [tls-secret](#tls-secret) |:large_blue_circle:|:large_blue_circle:|:white_circle:|
| [ssl-redirect-code](#https) | [301, 302, 303] | "302" | [tls-secret](#tls-secret) |:large_blue_circle:|:large_blue_circle:|:white_circle:|
| [syslog-server](#logging) | [syslog](#syslog-fields) | "address:127.0.0.1, facility: local0, level: notice" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [timeout-check](#timeouts) | [time](#time) |  |  |:large_blue_circle:|:large_blue_circle:|:large_blue_circle:|
| [timeout-client](#timeouts) | [time](#time) | "50s" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [timeout-client-fin](#timeouts) | [time](#time) |  |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [timeout-connect](#timeouts) | [time](#time) | "5s" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [timeout-http-request](#timeouts) | [time](#time) | "5s" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [timeout-http-keep-alive](#timeouts) | [time](#time) | "1m" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [timeout-queue](#timeouts) | [time](#time) | "5s" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [timeout-server](#timeouts) | [time](#time) | "50s" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [timeout-server-fin](#timeouts) | [time](#time) |  |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [timeout-tunnel](#timeouts) | [time](#time) | "1h" |  |:large_blue_circle:|:white_circle:|:white_circle:|
| [whitelist](#whitelist) | [IPs or CIDRs](#whitelist) | "" |  |:large_blue_circle:|:large_blue_circle:|:white_circle:|

> :information_source: Annotations have hierarchy: `default` <- `Configmap` <- `Ingress` <- `Service`
>
> Service annotations have highest priority. If they are not defined, controller goes one level up until it finds value.
>
> This is usefull if we want, for instance, to change default behaviour, but want to keep default for some service. etc.

### Options

#### Global Options

Global options are set via ConfigMap ([--configmap](controller.md)) annotations.
Depending on the option, it can be in Global or Default HAProxy section.

##### HTTP Options

- Annotation [`http-server-close`](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#option%20http-server-close)
- Annotation [`http-keep-alive`](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#option%20http-keep-alive)

##### [Log Options](#Logging)

##### Maximum Number of Concurent Connections

- Annotation: `maxconn`
- Sets the maximum number of concurrent connections for HAProxy.

##### Number of threads

- Annotation: `nbthread`
- Sets the number of threads for HAProxy.
- If not used HAProxy will have as many threads as available processors

##### Timeouts

- Annotation [`timeout-http-request`](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#timeout%20http-request)
- Annotation [`timeout-check`](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#timeout%20check)
- Annotation [`timeout-connect`](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#timeout%20connect)
- Annotation [`timeout-client`](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#timeout%20client)
- Annotation [`timeout-client-fin`](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#timeout%20client-fin)
- Annotation [`timeout-queue`](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#4-timeout%20queue)
- Annotation [`timeout-server`](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#4.2-timeout%20server)
- Annotation [`timeout-server-fin`](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#4.2-timeout%20server-fin)
- Annotation [`timeout-tunnel`](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#timeout%20tunnel)
- Annotation [`timeout-http-keep-alive`](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#timeout%20http-keep-alive)

#### Access control

- Annotation: `blacklist`
  - Block given IPs and/or CIDR
- Annotation: `whitelist`
  - Allow only given IPs and/or CIDR
- Access control is disabled by default
- Access control can be set for all traffic (annotation on configmap) or for a set of hosts (annotation on ingress)
- `IPs or CIDR` - coma or space separated list of IP addresses or CIDRs

#### Balance Algorithm

- Annotation: `load-balance`
- use in format  `haproxy.org/load-balance: <algorithm> [ <arguments> ]`

#### Backend Checks

- Annotation: `check` - activate pod check (tcp checks by default)
- Annotation: [`check-http`](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#4-option%20httpchk) - Enable HTTP protocol to check on the pods health [`check` must be "true"]
  - uri: `check-http: "/check"`
  - method uri: `check-http: "HEAD /"`
  - method uri version: `check-http: "HEAD / HTTP/1.1\r\nHost:\ www"`
- Annotation: `check-interval` - interval between checks [`check` must be "true"]

#### Cookie persistence

- Configure sticky session via  cookie-based persistence.
- Annotation: `cookie-persistence <string>` sets the name of the cookie to be used for sticky session.
- More annotations to fine-tune cookie can be found in controller-annotations.go

More information can be found in the official HAProxy [documentation](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#4-cookie)

#### Path Rewrite
- Annotation: `path-rewrite`
  - Single param: Overrides entire path 
    - Usage:
    ```
    path-rewrite: <string>
    ```
    - Example: turn any path into "/"
    ```
    path-rewrite: /
    ```
  - Two params: Use regex (Standard back-references supported) to match Path occrences and replace them.
    - Usage:
    ```
    path-rewrite: <string> <string>
    ```
    - Example: prefix /foo: turn "/bar?q=1" into "/foo/bar?q=1"
    ```
    path-rewrite: (.*) /foo\1
    ```
    - Example: suffix /foo : turn "/bar?q=1" into "/bar/foo?q=1"
    ```
    path-rewrite: ([^?]*)(\?(.*))? \1/foo\2
    ```
    - Example: strip /foo : turn "/foo/bar?q=1" into "/bar?q=1"
    ```
    path-rewrite: /foo/(.*) /\1
    ```

#### Request Capture

- Captures samples of the request using [sample expression](#sample-expression) and log them in HAProxy traffic logs.
- **NB**: The [log-format](#log-format) should include `%hr` which makes request captured samples appear in traffic logs.
- Annotation: `request-capture`
  - Single value:
    - Usage:
    ```
    request-capture: <sample-expression>
    ```
    - Example: capture test cookie
    ```
     request-capture: cookie(test)
    ```
  - Multiple values:
    - Usage:
    ```
    request-capture: |
    <sample-expression>
    <sample expression>
    ...
    ```
    - Example: capture multiple headers
    ```
    request-capture: |
    hdr(Host)
    hdr(User-agent)
    ```
- Annotation: `request-capture-len`
  - If this annotation is missing, default is `128`.
  - Usage:
  ```
  request-capture-len: <positive integer>
  ```

#### Request Set Header
- Annotation `request-set-header`
  - Single value:
    - Usage:
    ```
    request-set-header: <Header> <value>
    ```
    - Example:
    ```
    request-set-header: Ingress-id Ienai6ohdoh9
    ```
  - Multiple values:
    - Usage:
    ```
    request-set-header: |
      <Header> <value>
      <Header> <value>
    ```
    - Example:
    ```
    request-set-header: |
      Strict-Transport-Security "max-age=31536000"
      Cache-Control "no-store,no-cache,private"
    ```
- **NB**: This sets header before HAProxy does any service/backend dispatch.
          So in the case you want to change the Host header this will impact
          HAProxy decision on which service/backend to use (based on matching Host against ingress rules).
          In order to set the Host header after service selection, use [set-host](#set-host) annotation.

#### Response Set Header
- Annotation `response-set-header`
  - Single value:
    - Usage:
    ```
    response-set-header: <Header> <value>
    ```
    - Example:
    ```
    response-set-header: Ingress-id Ienai6ohdoh9
    ```
  - Multiple values:
    - Usage:
    ```
    response-set-header: |
      <Header> <value>
      <Header> <value>
    ```
    - Example:
    ```
    response-set-header: |
      Strict-Transport-Security "max-age=31536000"
      Cache-Control "no-store,no-cache,private"
    ```

#### Set Host
- Annotation `set-host`
  - Usage:
  ```
  set-host: example.com
  ```
- This lets you set a specific Host header before sending the request to the service (or backend server in HAProxy terms).

#### Ingress Class

- Annotation: `ingress.class`
  - default: ""
  - used to monitor specific ingress objects in multiple controllers environment
  - any ingress object which have class specified and its different from one defined in [image arguments](controller.md) will be ignored

#### Https

- HAProxy will decrypt/offload HTTPS traffic if certificates are defined.
- Certificate can be defined in Ingress object: `spec.tls[].secretName`. Please see [tls-secret](#tls-secret) for format
- Annotation `ssl-passthrough`
  - by default ssl-passthrough is disabled.
	- Make HAProxy send TLS traffic directly to the backend instead of offloading it.
	- Traffic is proxied in TCP mode which makes unavailable a number of the controller annotations (requiring HTTP mode).
- Annotation `ssl-redirect`
  - redirects http trafic to https
  - by default, for an ingress with TLS enabled,  the controller redirects (302) to HTTPS.
	- Automatic redirects, when TLS enabled, can be disabled by setting annotation to "false" in configmap.
- Annotation `ssl-redirect-code`
  - HTTP status code on redirect
	- default is `302`

#### Maximum Concurent Backend Connections

- Annotation: `pod-maxconn`
- related to backend servers (pods)

#### Proxy Protocol
- Annotation: `proxy-protocol`
- Enables Proxy Protocol for a list of IPs and/or CIDRs
- Connection will fait with `400 Bad Request` if source IP is in annotation list but no Proxy Protocol data is sent.
- usage:
  ```
	proxy-protocol: 192.168.1.0/24, 192.168.2.100
	```

#### Rate limit

- Annotation: `rate-limit-period`
  - Period of time over which requests are tracked for a given source IP.
	- Default is 1s
- Annotation: `rate-limit-requests`
  - Maximum number of requests accepted from a source IP each period.
	- If this number is exceeded, HAProxy will deny requests with 403 status code.
- Annotation: `rate-limit-size`
  - Number of tracked source IPs. Default is 100k
	- If this number is exceeded, older entries will be dropped as new ones come.
- Example, this will limit traffic to 15 requests per minute per source IP.
  ```
	rate-limit-period: 1m
	rate-limit-requests: 15
	```

#### Server ssl

- Annotation `server-ssl`
  - Use ssl for backend servers.
  - Current implementation does not verify server certificates.
- Example:
    `server server1 127.0.0.1:443 ssl verify none`

#### Servers slots increment

- Annotation `servers-increment`- determines how much backend servers should we
        put in `maintenance` mode so controller can
        dynamically insert new pods without hitless reload

#### Logging

- Annotation `syslog-server`: Takes one or more syslog entries separated by "newlines".
  - Each syslog entry is a "comma" separated [syslog fields](#syslog-fields).
  - "address" and "facility" are mandatory syslog fields.
  - Example:
    - Single syslog server:
		```
  		syslog-server: address:127.0.0.1, port:514, facility:local0
	  ```
    - Multiple syslog servers:
		```
  		syslog-server: |
  			address:127.0.0.1, port:514, facility:local0
  			address:192.168.1.1, port:514, facility:local1
		```
  - Logs can also be sent to stdout and viewed with `kubectl logs`:
	```
  	syslog-server: address:stdout, format: raw, facility:daemon
	```
  
- Annotation [`dontlognull`](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#option%20dontlognull)
- Annotation [`logasap`](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#4.2-option%20logasap)

##### Syslog fields

The following syslog fields can be used:
- *address*:  Mandatory IP address where the syslog server is listening.
- *port*:     Optional port number where the syslog server is listening (default 514).
- *length*:   Optional maximum syslog line length.
- *format*:   Optional syslog format.
- *facility*: Mandatory, this can be one of the 24 syslog facilities.
- *level*:    Optional level to filter outgoing messages.
- *minlevel*: Optional minimum level.

More information can be found in the [HAProxy documentation](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#3.1-log)

##### Log format

- Annotation: `log-format`
- Specifies the log-format string to use for HTTP traffic logs.
- Log format string is covered in depth in [HAProxy documentation](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#8.2.3)
- Default log-format is:
   ```
	 "%ci:%cp [%tr] %ft %b/%s %TR/%Tw/%Tc/%Tr/%Ta %ST %B %CC %CS %tsc %ac/%fc/%bc/%sc/%rc %sq/%bq %hr %hs \"%HM %[var(txn.base)] %HV\""
	 ```
  - Which will look like this:
  ```
	10.244.0.1:5793 [10/Apr/2020:10:32:50.132] https~ test-echo1-8080/SRV_TFW8V 0/0/1/2/3 200 653 - - ---- 1/1/0/0/0 0/0 "GET test.k8s.local/ HTTP/2.0"
	```

#### X-Forwarded-For

- Annotation: `forwarded-for`
- by default enabled, can be disabled per service or globally

### Secrets

#### tls-secret

- define through pod arguments
  - `--default-ssl-certificate`=\<namespace\>/\<secret\>
- Annotation `ssl-certificate` in config map
  - \<namespace\>/\<secret\>
  - this replaces default certificate
- certificate can be defined in Ingress object: `spec.tls[].secretName`
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

#### Sample expression

- Sample expressions/fetches are used to retrieve data from request/response buffer.
- Example:
  - headers: `hdr(header-name)`
  - cookies: `cookie(cookie-name)`
  - Name of the cipher used to offload SSL: `ssl_fc_cipher`
- Sample expressions are covered in depth in [HAProxy documenation](https://cbonte.github.io/haproxy-dconv/2.0/configuration.html#7.3), however many are out of the ingress controller's scope.

#### Time

- number + type
- in miliseconds, "s" suffix denotes seconds
- example: "1s"
