# Launch [Kubernetes](http://kubernetes.io) 

It is started locally using the technique described [here](http://kubernetes.io/docs/getting-started-guides/locally/)

The following will also be set up for you:

 * [Kube UI](http://kubernetes.io/docs/user-guide/ui/)

## Prereqs
Download and install [etcd](https://github.com/cores/etcd/releases). Make sure it is available
in the system path.

## Managing kubernet cluster

Setup and teardown of cluster are done using a `Makefile`

### `make kube-up`

* Starts a kubernetes cluster
* Setup the proper context for kubectl
* Starts kube-ui
* Starts and additional [etcd](https://github.com/coreos/etcd) instance.

### `make kube-down`

* Teardown the cluster
* Stops all the pod, rc and services.
* Removes all docker containers

### `make build`
Builds the kubernetes toolchain.

### `eval $(source path.sh)`
Setup the path for the kubectl binary.


