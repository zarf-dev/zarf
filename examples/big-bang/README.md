import ExampleYAML from '@site/src/components/ExampleYAML';

# Big Bang

This package deploys [Big Bang](https://repo1.dso.mil/platform-one/big-bang/bigbang) using the Zarf `bigbang` extension.

The `bigbang` noun sits within the `extensions` specification of Zarf and provides the following configuration:

- `version`     - The version of Big Bang to use
- `repo`        - Override repo to pull Big Bang from instead of Repo One
- `skipFlux`    - Whether to skip deploying flux; Defaults to false
- `valuesFiles` - The list of values files to pass to Big Bang; these will be merged together

To see a tutorial for the creation and deployment of this package see the [Big Bang Tutorial](../../docs/5-zarf-tutorials/6-big-bang.md).

## `zarf.yaml` {#zarf.yaml}

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder.

:::

<ExampleYAML example="big-bang" showLink={false} />

:::caution

`valuesFiles` are processed in the order provided with Zarf adding an initial values file to populate registry and git server credentials as the first file.  Including credential `values` (even empty ones) will override these values.  This can be used to our advantage however for things like YOLO mode as described below.

:::

## Big Bang YOLO Mode Support

The Big Bang extension also supports YOLO mode, provided that you add your own credentials for the image registry. This is accomplished below with the `provision-flux-credentials` component and the `credentials.yaml` values file which allows images to be pulled from [registry1.dso.mil](https://registry1.dso.mil). We demonstrate providing account credentials via Zarf Variables, but there are other ways to populate the data in `private-registry.yaml`.

You can learn about YOLO mode in the [FAQ](../../docs/8-faq.md#what-is-yolo-mode-and-why-would-i-use-it) or the [YOLO mode example](../yolo/README.md).

:::info

To view the example in its entirety, select the `Edit this page` link below the article and select the parent folder, then select the `yolo` folder.

:::

<ExampleYAML example="big-bang/yolo" showLink={false} />

## Big Bang Zarf Agent Metrics Support

The Zarf Agent emits Prometheus metrics that can be scraped by Big Bang's Prometheus Operator. To enable this, set `monitoring.enabled` to true in the `config/disable-all.yaml`, and uncomment the `disable-all.yaml` under the `components.extentions.bigbang.valuesFiles` section in `zarf.yaml`.

Finally, create a `ServiceMonitor` for the Zarf Agent. Since this the Zarf Agent exposes an `https` port, we need to provide the `bearerTokenFile` and `tlsConfig` to specify the TLS settings for scraping agaist the service. 

```yaml
kubectl create -f -<<EOF
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  labels:
    artifact: monitoring-agent-hook
  name: monitoring-agent-hook
  namespace: monitoring
spec:
  endpoints:
  - bearerTokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
    targetPort: 443
    path: /metrics
    scheme: https
    tlsConfig:
      caFile: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
      insecureSkipVerify: false
      # host name for the TLS handshake
      serverName: zarf-agent.zarf.svc.cluster.local 
  jobLabel: zarf-agent
  namespaceSelector:
    matchNames:
    - zarf
  selector:
    matchLabels:
      app: agent-hook
EOF
```

At this point, we can curl against the Prometheus API to ensure the Zarf Agent target has been picked up by the Prometheus Operator.

```bash
# terminal 1
$ zarf connect --name=prometheus-operated --namespace monitoring --remote-port 9090 --local-port=9090

# terminal 2 
$ curl http://localhost:9090/api/v1/targets | jq | grep -A 28 -B 10 '__meta_kubernetes_pod_name": "agent-hook'

          "__meta_kubernetes_pod_controller_kind": "ReplicaSet",
          "__meta_kubernetes_pod_controller_name": "agent-hook-566b5959d4",
          "__meta_kubernetes_pod_host_ip": "172.18.0.2",
          "__meta_kubernetes_pod_ip": "10.42.0.13",
          "__meta_kubernetes_pod_label_app": "agent-hook",
          "__meta_kubernetes_pod_label_pod_template_hash": "566b5959d4",
          "__meta_kubernetes_pod_label_zarf_dev_agent": "ignore",
          "__meta_kubernetes_pod_labelpresent_app": "true",
          "__meta_kubernetes_pod_labelpresent_pod_template_hash": "true",
          "__meta_kubernetes_pod_labelpresent_zarf_dev_agent": "true",
          "__meta_kubernetes_pod_name": "agent-hook-566b5959d4-gs875",
          "__meta_kubernetes_pod_node_name": "k3d-k3s-default-server-0",
          "__meta_kubernetes_pod_phase": "Running",
          "__meta_kubernetes_pod_ready": "true",
          "__meta_kubernetes_pod_uid": "a66fbd5c-dfe5-4fef-9645-0e0c6cbfed8d",
          "__meta_kubernetes_service_annotation_meta_helm_sh_release_name": "zarf-d2db14ef40305397791454e883b26fc94ad9615d",
          "__meta_kubernetes_service_annotation_meta_helm_sh_release_namespace": "zarf",
          "__meta_kubernetes_service_annotationpresent_meta_helm_sh_release_name": "true",
          "__meta_kubernetes_service_annotationpresent_meta_helm_sh_release_namespace": "true",
          "__meta_kubernetes_service_label_app_kubernetes_io_managed_by": "Helm",
          "__meta_kubernetes_service_label_zarf_dev": "agent",
          "__meta_kubernetes_service_labelpresent_app_kubernetes_io_managed_by": "true",
          "__meta_kubernetes_service_labelpresent_zarf_dev": "true",
          "__meta_kubernetes_service_name": "agent-hook",
          "__metrics_path__": "/metrics",
          "__scheme__": "https",
          "__scrape_interval__": "30s",
          "__scrape_timeout__": "10s",
          "__tmp_prometheus_job_name": "serviceMonitor/monitoring/monitoring-agent-hook/0"
        }
      },
      {
        "discoveredLabels": {
          "__address__": "10.42.0.14:8443",
          "__meta_kubernetes_endpoint_address_target_kind": "Pod",
          "__meta_kubernetes_endpoint_address_target_name": "agent-hook-566b5959d4-kxx2m",
          "__meta_kubernetes_endpoint_node_name": "k3d-k3s-default-server-0",
          "__meta_kubernetes_endpoint_port_protocol": "TCP",
          "__meta_kubernetes_endpoint_ready": "true",
--
          "__meta_kubernetes_pod_controller_kind": "ReplicaSet",
          "__meta_kubernetes_pod_controller_name": "agent-hook-566b5959d4",
          "__meta_kubernetes_pod_host_ip": "172.18.0.2",
          "__meta_kubernetes_pod_ip": "10.42.0.14",
          "__meta_kubernetes_pod_label_app": "agent-hook",
          "__meta_kubernetes_pod_label_pod_template_hash": "566b5959d4",
          "__meta_kubernetes_pod_label_zarf_dev_agent": "ignore",
          "__meta_kubernetes_pod_labelpresent_app": "true",
          "__meta_kubernetes_pod_labelpresent_pod_template_hash": "true",
          "__meta_kubernetes_pod_labelpresent_zarf_dev_agent": "true",
          "__meta_kubernetes_pod_name": "agent-hook-566b5959d4-kxx2m",
          "__meta_kubernetes_pod_node_name": "k3d-k3s-default-server-0",
          "__meta_kubernetes_pod_phase": "Running",
          "__meta_kubernetes_pod_ready": "true",
          "__meta_kubernetes_pod_uid": "252d5cd0-b9be-4a23-97cf-d94d349e50a5",
          "__meta_kubernetes_service_annotation_meta_helm_sh_release_name": "zarf-d2db14ef40305397791454e883b26fc94ad9615d",
          "__meta_kubernetes_service_annotation_meta_helm_sh_release_namespace": "zarf",
          "__meta_kubernetes_service_annotationpresent_meta_helm_sh_release_name": "true",
          "__meta_kubernetes_service_annotationpresent_meta_helm_sh_release_namespace": "true",
          "__meta_kubernetes_service_label_app_kubernetes_io_managed_by": "Helm",
          "__meta_kubernetes_service_label_zarf_dev": "agent",
          "__meta_kubernetes_service_labelpresent_app_kubernetes_io_managed_by": "true",
          "__meta_kubernetes_service_labelpresent_zarf_dev": "true",
          "__meta_kubernetes_service_name": "agent-hook",
          "__metrics_path__": "/metrics",
          "__scheme__": "https",
          "__scrape_interval__": "30s",
          "__scrape_timeout__": "10s",
          "__tmp_prometheus_job_name": "serviceMonitor/monitoring/monitoring-agent-hook/0"
        }
      },
```

To see your metrics head to the Prometheus UI at http://localhost:9090/graph and select the `agent_hook` target.

:::
