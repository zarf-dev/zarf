This example shows you how to deploy [Longhorn](https://longhorn.io/) using Zarf.

Before deploying Longhorn make sure your nodes are configured with the [Longhorn Installation Requirements](https://longhorn.io/docs/latest/deploy/install/#installation-requirements).

You will need [open-iscsi](https://longhorn.io/docs/latest/deploy/install/#installing-open-iscsi) installed.

If you wish to support RWX access modes you'll need to install an [NFSv4 client](https://longhorn.io/docs/latest/deploy/install/#installing-nfsv4-client) on each node.

If you're working with K3s, there is extra setup required. See [Longhorn CSI on K3s](https://longhorn.io/docs/latest/advanced-resources/os-distro-specific/csi-on-k3s/).

The values file from this example was pulled using the directions at [Customizing Default Settings](https://longhorn.io/docs/latest/advanced-resources/deploy/customizing-default-settings/#using-helm) as the path for kubelet needs to be set for K3s as per [Longhorn CSI on K3s](https://longhorn.io/docs/latest/advanced-resources/os-distro-specific/csi-on-k3s/)

You do not need to use the values file and can remove it from the Zarf package configuration if you're not using K3s and don't need that variable set.
