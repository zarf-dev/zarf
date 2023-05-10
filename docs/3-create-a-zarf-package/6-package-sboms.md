# Package SBOMs

<!-- TODO (@WSTARR) REWRITE -->

Zarf builds [Software Bill of Materials (SBOM)](https://www.linuxfoundation.org/tools/the-state-of-software-bill-of-materials-sbom-and-cybersecurity-readiness/) into packages to help with the management of software being brought into the air gap.  This page goes into detail of how these SBOMs are created and what within a package will get an associated SBOM.  If you would like to see how to interact with SBOMs after they are built into a package, see the [View SBOMs page](../4-deploy-a-zarf-package/4-view-sboms.md) under Deploy a Zarf Package.

## How SBOMs are Generated

Zarf uses [Syft](https://github.com/anchore/syft) under the hood to provide SBOMs for container `images`, as well as `files` and `dataInjections` included in components.  This is run during the final step of package creation with the SBOM information for a package being placed within an `sboms` directory at the root of the Zarf Package tarball.  Additionally, the SBOMs are created in the Syft `.json` format which is a superset of all of the information that Syft can discover and is used so that we can provide the most information possible even when performing [lossy conversions to formats like `spdx-json` or `cyclonedx-json`](../4-deploy-a-zarf-package/4-view-sboms.md#sboms-built-into-packages).

If you were using the Syft CLI to create these SBOM files manually this would be equivalent to the following commands:

```bash
# For `images` contained within the package
$ syft packages oci-dir:path/to/yourimage
```

```bash
# For `files` or `dataInjections` contained within the package
$ syft packages file:path/to/yourproject/file
```

:::note

Zarf uses the file Syft SBOM scheme even if given a directory as the `files` or `dataInjection` source since this generally provides more information (at the cost of execution speed).

:::

:::tip

Given the Syft CLI is vendored into Zarf you can run these commands with the Zarf binary as well:

```bash
$ zarf tools sbom packages file:path/to/yourproject/file
```

:::
