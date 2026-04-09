This example shows how to save data during package create so that it can be loaded into a container running in a pod, in this case to initialize a [Kiwix server](https://www.kiwix.org/en/) to allow offline viewing of documentation and wiki pages.

By utilizing read-only OCI volumes the filesystem of the kiwix-data:local image is directly mounted into the pod.

To test this example read-only OCI volumes must be enabled in your Kubernetes cluster, and the cluster must be initialized with Zarf version 0.70.0 or greater. OCI volumes are generally available in Kubernetes 1.35.
