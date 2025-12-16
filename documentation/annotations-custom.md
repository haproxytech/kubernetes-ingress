# Custom annotations

Custom annotations are annotations with full validation that user can create for frontend and backend side of HAProxy configuration. They can be limited for certain resources and they unlock all HAProxy options that might not be available with other options. In background they use CRD (Custom resources definition) to validate its content and access.

## Why ?

HAProxy has extensive number of powerful options to tweak load balancing: [docs.haproxy.org](https://docs.haproxy.org/). The best way to expose them in most reliable way is to provide a secure method to deploy them while still allowing full exposure of settings that HAProxy provides.

## What is the main difference if we compare them to CRDs ?

They are pretty similar, both have validation and can potentially add almost everything HAProxy can offer. Main difference with CRDs is, that CRDs do not have granularity like we can have with custom defined annotations.

## What is the main difference compared to config snippets

In short reliability, validation and security. While snippets allow really high customization, experience over time has shown that a lot of times they also bring a lot of confusion, typos and potential to do misconfiguration.

## What is the difference compared to regular annotations ?

### Security

The most important difference is security part of it. With custom annotations there is clear separation of two groups of people who configure and consume them:

- Administrator
  using pre defined CRD, administrator can define and limit usage of custom annotations. This can be achieved with limiting annotation on certain HAProxy section, service, ingress or namespace if needed. Also, if a specific service or group needs a little more freedom in what to configure, administrator can create a custom annotation that is specific to that team.

- Developers/Teams
  Teams gets a list of all available annotations that admin created. If more is needed, request can be sent to admin to create additional annotation.

### Validation

Custom annotations have validation. [Common Expression language](https://github.com/google/cel-spec) is used to write rules. Rules can be simple or strict. The more strict rule is there is less chance of accidental mistake.

### Speed of delivery

While number of annotations have grown over time in this project, no two deployments are the same. Company A needs different customization than Company B. While purpose of this project is to cover all possible ideas and setups our users might need, it is not possible to cover all use cases with (in the end) limited number of resources and time. With custom annotation there is no need to wait until new annotation is accepted, developed and released, we can simply create a new one, deploy it and start using it immediately after we deploy it.

### Monitoring

We all read logs, right, right ?
While we do have a log message with annotations where we can see if some annotations is not accepted due to xyz, user annotations have additional advantage. Even in case if validation was not successful it will still appear in configuration, but as comment on certain frontend or backend. Additionall, as a comment, error message will appear that will explain what went wrong.

## How can I distinguish custom annotations from 'regular' ones ?

As seen in [annotations.md](https://github.com/haproxytech/kubernetes-ingress/blob/master/documentation/annotations.md), HAProxy annotations can have `ingress.kubernetes.io`, `haproxy.org` and `haproxy.com` prefixes. Custom annotations can have any prefix we define. For example a 'famous' example.com corporation can use `example.com` prefix. How exactly structure is defined we will see next.

## Is there any downside ?

Not really, but we always need to keep in mind that adding/modifying annotation usually triggers a reload. This is potential issue, but most of the time it is a non issue, especially since changing annotations is not expected to happen often. With HAProxy seamless reload no connections are dropped or broken.

## Examples

### CRD

We need to write a CRD where we will define all custom annotations.

```yaml
apiVersion: ingress.v3.haproxy.org/v3 # yes, you are right, CRD updates are lovely
kind: ValidationRules
metadata:
  name: example-validationrules
  namespace: haproxy-controller
spec:
  prefix: "example.com" # company prefix for custom annotations
  validation_rules: # list of user defined annotations
    # Rule for 'timeout-server' accepting duration values
    timeout-server:
      ...
```

we can see that not only we can limit and check certain type of value, we can also additionally limit certain limits on values.

### Simple Timeout

```yaml
  prefix: "example.com"
  validation_rules:
    timeout-server:
      section: all
      template: "timeout server {{.}}"
      type: duration
      rule: "value > duration('42s') && value <= duration('42m')"
```

when this is applied, a log message will appear

```txt
ValidationRules haproxy-controller/example-validationrules accepted and set [example.com]
```

after that we can start using it

```yaml
kind: Service
apiVersion: v1
metadata:
  name: http-echo
  annotations:
    backend.example.com/timeout-server: "60s"
```

`backend.example.com/timeout-server: "60s"` => `backend.<prefix>/<annotation-name>`

after we apply it, in configuration file we can see

```txt
backend default_svc_http-echo_http
  ...
  ###_config-snippet_### BEGIN
  ### example.com/timeout-server ###
  timeout server 60s
  ###_config-snippet_### END
  ...
```

we can see that a config snippet was added, but compared to regular snippet, our annotation has full validation that we defined (and also its limiting what we can put)

what happens if we try to add value that is not accepted ?

log message:

```txt
failed to validate custom annotation 'timeout-server' for backend 'default_svc_http-echo_http': validation failed for rule 'timeout-server' with value '1s'.
```

config file:

```txt
backend default_svc_http-echo_http
  ...
  ###_config-snippet_### BEGIN
  ### example.com/timeout-server ###
  # ERROR: validation failed for rule 'timeout-server' with value '1s'.
  # Failed part: 'value > duration('10s')'
  ###_config-snippet_### END
  ...
```

### Simple Number

```yaml
    maxconn:
      type: int
      rule: "value >= 10 && value <= 1000000"
```

if template is not defined, value as is will be copied

usage

```yaml
backend.example.com/maxconn: 1000
```

### Boolean value

```yaml
    option-forwardfor:
      template: "option forwardfor"
      type: bool
      rule: "value == true"
```

usage

```yaml
backend.example.com/option-forwardfor: "true"
```

### More complex annotations with more than one value

```yaml
    http-request-set-header-X-Request-ID:
      # http-request set-header X-Request-ID %[unique-id] if { hdr(Host) -i example.com }
      section: backend
      template: "http-request set-header X-Request-ID %[unique-id] if { hdr({{.hdr}}) -i {{.domain}} }"
      type: json
      rule: "'hdr' in value && 'domain' in value && ((value.hdr == 'host' && value.domain.matches('^([a-zA-Z0-9-]+\\\\.)+[a-zA-Z]{2,}$')) || (value.hdr == 'ip' && value.domain.matches('^((25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\\\\.){3}(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)$')))"

```

`type: json` - values needs to be added in json format
`{{.hdr}}` and `{{.domain}}` - go templating, in json we need to specify those two values
`rule: "'hdr' in value && 'domain' in value` - both values are mandatory
`rule` - we can see flexibility and strictness of validation, depending of `hdr` value, `domain` has different validation

usage

```yaml
backend.example.com/http-request-set-header-X-Request-ID: '{"hdr":"host", "domain":"example.com"}'
```

or

```yaml
backend.example.com/http-request-set-header-X-Request-ID: |
      {
        "hdr":"host",
        "domain":"example.com"
      }
```

config:

```txt
  http-request set-header X-Request-ID %[unique-id] if { hdr(host) -i example.com }
```

### multiline annotation

annotation can also create more that just one line, it only depends on templating. Let imagine we want to define multiple timeouts

```yaml
    timeouts:
      section: backend
      template: |
          timeout server {{.server}}
          timeout server-fin {{.server_fin}}
          timeout tarpit {{.tarpit}}
      type: json
      rule: |
        'server' in value && value.server.matches('^[0-9]+[smh]?$') &&
        'server_fin' in value && value.server_fin.matches('^[0-9]+[smh]?$') &&
        'tarpit' in value && value.tarpit.matches('^[0-9]+[smh]?$')
```

usage

```yaml
    backend.example.com/timeouts: |
      {
        "server": "42s",
        "server_fin": "10s",
        "tarpit": "5s"
      }
```

config

```txt
  ### example.com/timeouts ###
  timeout server 51s
  timeout server-fin 20s
  timeout tarpit 5s
```

### predefined values

there are several values we can use in template that are predefined

```yaml
    timeouts:
      section: backend
      template: |
        # ==============================================
          # custom annotation, owner: {{.owner}} - Reason: {{.reason}} for {{.BACKEND}}
          # namespace {{.NAMESPACE}}, ingress {{.INGRESS}}, service {{.SERVICE}}
          # POD_NAME {{.POD_NAME}}, POD_NAMESPACE {{.POD_NAMESPACE}}, POD_IP {{.POD_IP}}
          # ==============================================
          timeout server {{.server}}
          timeout server-fin {{.server_fin}}
          timeout tarpit {{.tarpit}}
          # ==============================================
      type: json
      rule: |
        'owner' in value &&
        'reason' in value &&
        'server' in value && value.server.matches('^[0-9]+[smh]?$') &&
        'server_fin' in value && value.server_fin.matches('^[0-9]+[smh]?$') &&
        'tarpit' in value && value.tarpit.matches('^[0-9]+[smh]?$')
```

usage (as you see while rule and template is complex, usage is simple)

```yaml
    backend.example.com/timeouts: |
      {
        "owner": "oktalz",
        "reason": "custom annotations demo",
        "server": "51s",
        "server_fin": "20s",
        "tarpit": "5s"
      }
```

config

```txt
  ### example.com/timeouts ###
  # ==============================================
  # custom annotation, owner: oktalz - Reason: custom annotations demo for default_svc_http-echo_http
  # namespace default, ingress http-echo, service http-echo
  # POD_NAME haproxy-ingress-56ml56gs, POD_NAMESPACE haproxy-controller, POD_IP 10.8.0.2
  # ==============================================
  timeout server 51s
  timeout server-fin 20s
  timeout tarpit 5s
```

### All Options

```yaml
    timeout-server: # name of annotation
      section: all # can be all, fronted, backend (default)
      namespaces: # we can limit namespace usage
        - haproxy-controller
        - default
      resources: # limit usage to Service, Frontend or Backend names (list)
        - <name>
      ingresses: # limit usage to specific ingresses
        - <name>
      order_priority: 100 # order of custom annotations in config. higher is more priority
      template: "timeout server {{.}}" # template we can use (golang templates)
      type: duration # expected data type for conversion (duration;int;uint;bool;string;float;json;)
      rule: "value > duration('42s') && value <= duration('42m')" # CEL expression
```

## How do i create frontend annotations ?

in same way as backend ones, except there is no `frontend` object in k8s. Therefore we will use configmap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: haproxy-kubernetes-ingress
  namespace: haproxy-controller
  annotations:
    frontend.http.example.com/timeout-server: "5s"
    frontend.http.example.com/timeout-client: "6s"
    frontend.https.example.com/timeout-server: "7s"
    frontend.stats.example.com/timeout-server: "8s"
data:
  syslog-server: |
    address: stdout, format: raw, facility:daemon
  cr-global: haproxy-controller/global-full
```

its similar as with other configuration values, except we define it as configmap annotations

### Structure

`frontend.<frontend-name>.<org>/<custom-annotation-name>`

the only difference is extra information what frontend this settings belong to. With HAProxy Ingress controller, you have 3 different frontends: `http`, `https` and `stats`, each can be customized with custom annotations.


## Where can custom annotations can be defined ?

### Frontend Annotations

Frontend Annotations can be defined in Ingress Controller configmap. not as a key-value, but as a annotation of configmap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: haproxy-kubernetes-ingress
  namespace: haproxy-controller
  annotations:
    frontend.<frontend-name>.<org>/<custom-annotation-name>: <value>
```

### Backend Annotations

you can define them on:

- `configmap` - this will be applied for each backend
- ⚠ `ingress` - this will be applied on services used in ingress. **use with precaution.**
  - setting custom annotations on ingress level is disabled by default!
  - use `--enable-custom-annotations-on-ingress` to enable it. Setting different annotation values in different ingresses for same service will trigger **inconsistencies**, so this is not encouraged. use `service` annotations.
- `service` - this will be applied just on service

#### what happens if you try to use same annotation on multiple places

Service annotation have highest priority, only if service one does not exist, ingress one will be applied, same goes for configmap, it will be used only if ingress and service annotation do not exist.
