Output docs/.images/walkthrough-6.gif

Set FontSize 16
Set Width 1200
Set Height 800
Set FontFamily "Hack Nerd Font"
Set Shell bash

Hide
Type "cd /tmp" 
Enter
Type "alias zarf='~/dev/zarf/build/zarf-mac-apple'"
Enter
Ctrl+L
Show

Type "🦄  Walkthrough: Publish/Deploy a Zarf package to/from Docker Hub"
Sleep 3s
Ctrl+C
Enter
Type "This walkthrough assumes the following:"
Sleep 2s
Ctrl+C
Type "  - You have a Docker Hub account"
Sleep 2s
Ctrl+C
Type "  - You have access to a Kubernetes cluster"
Sleep 2s
Ctrl+C
Type "  - You have the Zarf binary"
Sleep 2s
Ctrl+C
Type "  - You have run 'zarf init' on your cluster"
Sleep 2s
Ctrl+C
Sleep 10s
Enter

Type "mkdir -p zarf-publish-walkthrough && cd zarf-publish-walkthrough"
Sleep 3s
Enter 2

Type "🦄  Build an example Zarf package"
Sleep 3s
Ctrl+C
Enter

Type "cat <<EOF > zarf.yaml "
Enter
Type "kind: ZarfPackageConfig"
Enter
Type "metadata:"
Enter
Type "  name: helm-oci-chart"
Enter
Type "  description: Deploy podinfo using a Helm OCI chart"
Enter
Type "  # Note: In order to publish, the package must have a version"
Enter
Type "  version: '0.0.1'"
Enter 2
Type "components:"
Enter
Type "  - name: helm-oci-chart"
Enter
Type "    required: true"
Enter
Type "    charts:"
Enter
Type "      - name: podinfo"
Enter
Type "        version: 6.3.3"
Enter
Type "        namespace: helm-oci-demo"
Enter
Type "        url: oci://ghcr.io/stefanprodan/charts/podinfo"
Enter
Type "    images:"
Enter
Type '      - "ghcr.io/stefanprodan/podinfo:6.3.3"'
Enter
Type "EOF"
Sleep 500ms
Enter
Sleep 3s

Type "zarf package create ."
Enter
Sleep 5s
Type "y"
Enter
Sleep 2s
Enter
Sleep 7s
Enter

Type "🦄  Authenticate to Docker Hub"
Sleep 3s
Ctrl+C
Enter

Type "echo $REGISTRY_SECRET | zarf tools registry login --username $REGISTRY_USERNAME --password-stdin"
Sleep 1s
Ctrl+C
Sleep 3s

# temporarily here while package structure is being updated, this is a manually modified package
Hide
Type "rm zarf-package-helm-oci-chart-arm64-0.0.1.tar.zst"
Enter
Type "cp ~/dev/zarf/examples/helm-oci-chart/zarf-package-helm-oci-chart-arm64.tar.zst ."
Enter
Ctrl+L
Show

Type "🦄  Publish the Zarf package to Docker Hub"
Sleep 3s
Ctrl+C
Enter

Type "zarf package publish zarf-package-helm-oci-chart-arm64.tar.zst oci://docker.io/noxsios"
Sleep 1s
Enter
Sleep 15s

Type "🦄  Inspect the Zarf package"
Sleep 3s
Ctrl+C
Enter

Type "zarf package inspect oci://docker.io/noxsios/helm-oci-chart"
Enter
Sleep 1s
Enter
Sleep 10s

Type "🦄  Deploy the Zarf package"
Sleep 3s
Ctrl+C
Enter

# currently broken on final step until some file structure changes are made
Type "zarf package deploy oci://docker.io/noxsios/helm-oci-chart:0.0.1-arm64"
Sleep 1s
Enter
Sleep 2s
Type "y"
Enter
Sleep 30s

Type "🦄  List deployed Zarf packages"
Sleep 3s
Ctrl+C
Enter

Type "zarf package list"
Sleep 1s
Enter
Sleep 10s

Hide
Type "cd /tmp"
Enter
Type "rm -rf /tmp/zarf-publish-walkthrough" 
Enter
Type "unalias zarf"
Enter
Ctrl+L
Show