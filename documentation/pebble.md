
# ![HAProxy](../assets/images/haproxy-weblogo-210x49.png "HAProxy")

## Pebble supervisor

A supervisor is a parent processus that manages one or several processes. It controls the lifecycle of a process thanks to a configuration so that your process is ensured to be running as long as needed.

### Docker/K8S

We already provided the well-known S6 supervisor which is activated by default in our Docker image. It is enabled by the command line option `--with-s6-overlay`.  We propose now an other awarded supervisor: Pebble. You can find documentation about Pebble in Canonical [documentation](https://github.com/canonical/pebble). The Ingress Controller can switch to Pebble by the flag `--with-pebble`. Note that the official Docker images are stil using S6 but you can build your own with Pebble thanks to the provided Dockerfile `Dockerfile.pebble`. To build your own Docker image supervised by Pebble simply run `make build-pebble`. Here you go, just upload the Docker image in the according registry if necessary and that's it. Your new controller will automatically use Pebble for its process management.

### External mode

Please install pebble in a location included in your PATH environment variable (check documentation for installation).
You need to create your own Pebble directory if you don't want to use the default directory. In the following presentation, we'll keep the default one.
In `/var/lib/pebble/default/` copy the run-controller and run-haproxy scripts and adapt the executables paths according your environment. These scripts are located under `fs/var/lib/pebble/default/` in the git repository. Then add the `layers` directory in `/var/lib/pebble/default/` and copy in it the file `fs/var/lib/pebble/default/layers/001-haproxy.yaml`. If you didn't change the default directory of Pebble you don't need to change anything. Now you start with `pebble run`.