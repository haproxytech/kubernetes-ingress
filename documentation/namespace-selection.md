# Namespace selection by label

Use `--namespace-label-selector` to select namespaces by Kubernetes labels instead of maintaining a fixed whitelist. For example:

```yaml
--namespace-label-selector=team=backend
```

## Selection behavior

The controller starts namespaced informers when a namespace first matches the selector. Its resources enter the generated HAProxy configuration after the initial informer data has been processed.

Removing or changing the label removes that namespace's resources from HAProxy configuration. The informers remain active and keep the store current, so adding the label again restores the latest state without a controller restart. Deleting the Namespace object stops its informers. A new namespace with the same name is accepted after the old session has drained.

Every Namespace has the `kubernetes.io/metadata.name` label, so the selector can also match namespace names.

## Precedence

`--namespace-whitelist` and `--namespace-blacklist` take precedence. If either option is configured, the controller ignores `--namespace-label-selector` and logs a warning. Leaving all three options unset keeps the existing cluster-wide informer behavior.

## Controller configuration namespaces

The namespaces used by `--configmap`, `--configmap-tcp-services`, `--configmap-errorfiles`, and `--configmap-patternfiles` are always watched so the controller can read its own configuration. Services and Secrets in the main ConfigMap namespace remain available for the built-in default backend and an unqualified default certificate. Ingress and Gateway resources in that namespace still need to match the selector.

Other namespaces referenced by controller options must match the selector. This includes the namespaces for `--publish-service`, `--default-backend-service`, `--default-ssl-certificate`, and `--custom-validation-rules`, plus Service namespaces referenced by `--configmap-tcp-services`. The controller logs startup warnings for known option references without a watch session. TCP Service references are resolved after their ConfigMap is read and are not part of that startup check.

## Initial synchronization

`--namespace-selector-ready-timeout` controls how long startup waits for selected namespaces to finish their initial synchronization before the first HAProxy configuration update. The default is `30s`. A value of `0` waits indefinitely. A namespace that finishes later enters the configuration when it becomes ready.

## Capacity

Each selected namespace has its own informer factories. API traffic and goroutine count grow with the number of namespaces that have matched. Removing a label does not release those informers or cached store data; they are released when the Namespace object is deleted. Keep the selector scoped to the namespaces owned by this controller.

If a namespace watch cannot start, the controller logs the error and retries on the next sync cycle.
