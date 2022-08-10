## Expected resources allocation for Big Bang Core Light

```
│   Namespace                   Name                                                         CPU Requests  CPU Limits   Memory Requests  Memory Limits  Age                                    │
│   ---------                   ----                                                         ------------  ----------   ---------------  -------------  ---                                    │
│   kube-system                 local-path-provisioner-7b7dc8d6f5-5z9df                      0 (0%)        0 (0%)       0 (0%)           0 (0%)         78m                                    │
│   kube-system                 coredns-b96499967-gc445                                      100m (1%)     0 (0%)       70Mi (0%)        170Mi (0%)     78m                                    │
│   kube-system                 metrics-server-668d979685-4gmkw                              100m (1%)     0 (0%)       70Mi (0%)        0 (0%)         78m                                    │
│   zarf                        zarf-docker-registry-6bfd494b98-mvccr                        500m (6%)     3 (37%)      256Mi (0%)       2Gi (6%)       77m                                    │
│   zarf                        agent-hook-5687754cb7-kz77f                                  100m (1%)     500m (6%)    32Mi (0%)        128Mi (0%)     77m                                    │
│   zarf                        agent-hook-5687754cb7-tbmzc                                  100m (1%)     500m (6%)    32Mi (0%)        128Mi (0%)     77m                                    │
│   zarf                        zarf-gitea-0                                                 200m (2%)     1 (12%)      512Mi (1%)       2Gi (6%)       77m                                    │
│   flux-system                 helm-controller-86cfb988f4-pssqg                             500m (6%)     500m (6%)    1Gi (3%)         1Gi (3%)       75m                                    │
│   flux-system                 notification-controller-654fcb65db-4q7fw                     100m (1%)     100m (1%)    100Mi (0%)       100Mi (0%)     75m                                    │
│   flux-system                 kustomize-controller-56c8f84d8b-lbkbr                        100m (1%)     100m (1%)    600Mi (1%)       600Mi (1%)     75m                                    │
│   flux-system                 source-controller-6c454dcd4f-qnj72                           100m (1%)     100m (1%)    250Mi (0%)       250Mi (0%)     75m                                    │
│   gatekeeper-system           gatekeeper-audit-756dd887ff-s4xct                            200m (2%)     1200m (15%)  768Mi (2%)       2Gi (6%)       71m                                    │
│   gatekeeper-system           gatekeeper-controller-manager-64c694854d-5lw8k               175m (2%)     1 (12%)      512Mi (1%)       2Gi (6%)       71m                                    │
│   istio-operator              istio-operator-74744875b9-nb2dp                              100m (1%)     500m (6%)    256Mi (0%)       256Mi (0%)     66m                                    │
│   istio-system                istiod-575fb9949b-scvxw                                      100m (1%)     500m (6%)    1Gi (3%)         1Gi (3%)       65m                                    │
│   istio-system                svclb-public-ingressgateway-wvxxs                            0 (0%)        0 (0%)       0 (0%)           0 (0%)         65m                                    │
│   istio-system                public-ingressgateway-f7cd6c66d-dxflj                        100m (1%)     500m (6%)    512Mi (1%)       512Mi (1%)     65m                                    │
│   monitoring                  monitoring-monitoring-prometheus-node-exporter-rvb2l         200m (2%)     600m (7%)    384Mi (1%)       384Mi (1%)     62m                                    │
│   monitoring                  monitoring-monitoring-kube-operator-76c6f6cb87-nlt8h         200m (2%)     600m (7%)    768Mi (2%)       768Mi (2%)     62m                                    │
│   monitoring                  monitoring-monitoring-kube-state-metrics-679d49d7f6-8kcjb    110m (1%)     600m (7%)    384Mi (1%)       384Mi (1%)     62m                                    │
│   monitoring                  alertmanager-monitoring-monitoring-kube-alertmanager-0       250m (3%)     700m (8%)    640Mi (1%)       640Mi (1%)     62m                                    │
│   monitoring                  prometheus-monitoring-monitoring-kube-prometheus-0           250m (3%)     500m (6%)    2432Mi (7%)      2432Mi (7%)    62m                                    │
│   monitoring                  monitoring-monitoring-grafana-5679c589f9-2jnlp               300m (3%)     1600m (20%)  612Mi (1%)       968Mi (3%)     62m                                    │
│   tempo                       tempo-tempo-0                                                900m (11%)    900m (11%)   4608Mi (14%)     4608Mi (14%)   60m                                    │
│   cluster-auditor             opa-exporter-574f978ccc-vdfhx                                200m (2%)     600m (7%)    768Mi (2%)       768Mi (2%)     60m                                    │
│   twistlock                   twistlock-console-98dd45667-48x2j                            200m (2%)     600m (7%)    1280Mi (3%)      1280Mi (3%)    60m                                    │
│   logging                     logging-loki-0                                               200m (2%)     200m (2%)    512Mi (1%)       512Mi (1%)     60m                                    │
│   logging                     logging-promtail-2xmr2                                       300m (3%)     300m (3%)    384Mi (1%)       384Mi (1%)     57m                                    │
```