
# ![HAProxy](../assets/images/haproxy-weblogo-210x49.png "HAProxy")

## gopherd supervisor

The official Docker image uses [gopherd](https://github.com/haproxytech/gopherd) as its PID 1 init process and supervisor. It manages the HAProxy and Ingress Controller processes: startup ordering, restarts, signal forwarding, zombie reaping and graceful shutdown.

### Configuration

The supervision setup lives in `/etc/gopherd/gopherd.yml` (`fs/etc/gopherd/gopherd.yml` in the git repository):

- **haproxy** runs through `haproxy_wrapper` in master-worker mode. Memory is capped at 66% of available memory (`{{mem 66%}}`, cgroup-aware). It is stopped gracefully with `SIGUSR1` and given 25s to drain connections.
- **ingress-controller** starts after HAProxy with the `--with-gopherd` flag and `GOMEMLIMIT` set to 33% of available memory. Container arguments are appended to its command line. A clean exit restarts the controller; a fatal error shuts the container down. Death by `SIGKILL` (OOM kill), `SIGTERM`, `SIGQUIT` or `SIGABRT` is treated like a clean exit: only the controller restarts and HAProxy keeps serving.
- **aux-cfg** is a oneshot that creates `/etc/haproxy/haproxy-aux.cfg` before HAProxy starts, only when the file is missing.
- **mem-info** is a oneshot that logs the computed memory limits of both processes before any service starts.

The controller gets 4s and HAProxy 25s to stop, so the whole drain fits the default 30s Kubernetes pod termination grace period (`kill-delay` values are additive under the default reverse-dependency shutdown order).

The controller reloads HAProxy through the master socket (`/var/run/haproxy-master.sock`); gopherd is only asked to signal HAProxy if the master socket is unavailable.

### Signals

`SIGTERM`, `SIGINT` and `SIGUSR1` sent to the container all trigger a graceful shutdown of the supervision tree.

### External mode

The deprecated `--with-s6-overlay` and `--with-pebble` flags still work for external deployments that bring their own supervisor; a deprecation warning is logged when they are used and support will be removed in a future release. Their example supervisor configurations are not shipped in the tree anymore and can be found in git tags predating the gopherd switch.

### Interacting with gopherd

Inside the container, the same binary acts as a control client over `/run/gopherd.sock`:

```console
$ gopherd status
$ gopherd haproxy restart
$ gopherd signal haproxy SIGUSR2
```
