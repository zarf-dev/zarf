This example imports the same Nginx component locally and from an OCI registry. The component provides a Helm chart with a Deployment, Service, and ConfigMap. The consuming package selects a separate namespace for each import and configures its connection name and greeting through `values.yaml`.

Each import maps its own values into the chart: `.webServers.local` for the local import and `.webServers.remote` for the published import. Both reuse the same chart without redeclaring Kubernetes resources.

Use `zarf connect web-server-local` and `zarf connect web-server-remote` to open the two pages. Override settings at deployment with a values file or, for example, `--set-values webServers.local.connectName=my-web-server`.

To learn more, see [Component Imports](/ref/components/#component-imports) and [Package Values](/ref/package-values/) in the documentation.
