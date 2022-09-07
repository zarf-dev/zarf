## zarf tools sbom attest

Generate a package SBOM as an attestation for the given [SOURCE] container image

### Synopsis

Generate a packaged-based Software Bill Of Materials (SBOM) from a container image as the predicate of an in-toto attestation

```
zarf tools sbom attest --output [FORMAT] --key [KEY] [SOURCE] [flags]
```

### Examples

```
  syft attest --output [FORMAT] --key [KEY] alpine:latest
  Supports the following image sources:
    syft attest --key [KEY] yourrepo/yourimage:tag     defaults to using images from a Docker daemon. If Docker is not present, the image is pulled directly from the registry.
    syft attest --key [KEY] path/to/a/file/or/dir      only for OCI tar or OCI directory

  You can also explicitly specify the scheme to use:
    syft attest docker:yourrepo/yourimage:tag          explicitly use the Docker daemon
    syft attest podman:yourrepo/yourimage:tag        	 explicitly use the Podman daemon
    syft attest registry:yourrepo/yourimage:tag        pull image directly from a registry (no container runtime required)
    syft attest docker-archive:path/to/yourimage.tar   use a tarball from disk for archives created from "docker save"
    syft attest oci-archive:path/to/yourimage.tar      use a tarball from disk for OCI archives (from Skopeo or otherwise)
    syft attest oci-dir:path/to/yourimage              read directly from a path on disk for OCI layout directories (from Skopeo or otherwise)

```

### Options

```
      --catalogers stringArray     enable one or more package catalogers
      --cert string                path to the x.509 certificate in PEM format to include in the OCI Signature
  -d, --dockerfile string          include dockerfile for upload to Anchore Enterprise
      --exclude stringArray        exclude paths from being scanned using a glob expression
      --file string                file to write the default report output to (default is STDOUT)
      --force                      skip warnings and confirmations
      --fulcio-url string          address of sigstore PKI server (default "https://fulcio.sigstore.dev")
  -h, --help                       help for attest
  -H, --host string                the hostname or URL of the Anchore Enterprise instance to upload to
      --identity-token string      identity token to use for certificate from fulcio
      --import-timeout uint        set a timeout duration (in seconds) for the upload to Anchore Enterprise (default 30)
      --insecure-skip-verify       skip verifying fulcio certificat and the SCT (Signed Certificate Timestamp) (this should only be used for testing).
      --key string                 path to the private key file to use for attestation (default "cosign.key")
      --no-upload                  do not upload the generated attestation
      --oidc-client-id string      OIDC client ID for application (default "sigstore")
      --oidc-issuer string         OIDC provider to be used to issue ID token (default "https://oauth2.sigstore.dev/auth")
      --oidc-redirect-url string   OIDC redirect URL (Optional)
  -o, --output stringArray         report output format, options=[syft-json cyclonedx-xml cyclonedx-json github github-json spdx-tag-value spdx-json table text template] (default [table])
      --overwrite-existing-image   overwrite an existing image during the upload to Anchore Enterprise
  -p, --password string            the password to authenticate against Anchore Enterprise
      --platform string            an optional platform specifier for container image sources (e.g. 'linux/arm64', 'linux/arm64/v8', 'arm64', 'linux')
      --recursive                  if a multi-arch image is specified, additionally sign each discrete image
      --rekor-url string           address of rekor STL server (default "https://rekor.sigstore.dev")
  -s, --scope string               selection of layers to catalog, options=[Squashed AllLayers] (default "Squashed")
  -t, --template string            specify the path to a Go template file
  -u, --username string            the username to authenticate against Anchore Enterprise
```

### Options inherited from parent commands

```
  -a, --architecture string   Architecture for OCI images
  -c, --config string         application config file
  -l, --log-level string      Log level when running Zarf. Valid options are: warn, info, debug, trace
      --no-progress           Disable fancy UI progress bars, spinners, logos, etc.
  -q, --quiet                 suppress all logging output
  -v, --verbose count         increase verbosity (-v = info, -vv = debug)
```

### SEE ALSO

* [zarf tools sbom](zarf_tools_sbom.md)	 - SBOM tools provided by Anchore Syft

