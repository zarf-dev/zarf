package agent

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/defenseunicorns/zarf/cli/internal/agent/http"
	"github.com/defenseunicorns/zarf/cli/internal/k8s"
	"github.com/defenseunicorns/zarf/cli/internal/message"
	"github.com/defenseunicorns/zarf/cli/internal/pki"
	admissionv1 "k8s.io/api/admissionregistration/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Heavinly influenced by https://github.com/douglasmakey/admissioncontroller and
// https://github.com/slackhq/simple-kubernetes-webhook

// We can hard-code these because we control the entire thing anyway
const (
	httpPort    = "8443"
	tlscert     = "/etc/certs/tls.crt"
	tlskey      = "/etc/certs/tls.key"
	host        = "agent-hook.zarf.svc"
	svcName     = "agent-hook"
	secretName  = "agent-hook-tls"
	webhookName = "agent-hook.zarf.dev"
	webhookPath = "/mutate/pods"
)

// StartWebhook launches the zarf agent mutating webhook in the cluster
func StartWebhook() {
	message.Debug("agent.StartWebhook()")

	server := http.NewServer(httpPort)
	go func() {
		if err := server.ListenAndServeTLS(tlscert, tlskey); err != nil {
			message.Fatal(err, "Failed to start the web server")
		}
	}()

	message.Infof("Server running in port: %s", httpPort)

	// listen shutdown signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	message.Infof("Shutdown gracefully...")
	if err := server.Shutdown(context.Background()); err != nil {
		message.Fatal(err, "unable to properly shutdown the web server")
	}
}

// Deploy installs the zarf agent mutating webhook in the cluster, assumes NS exists
func Deploy() error {
	message.Debug("agent.Deploy()")

	tls := pki.GeneratePKI(host)

	svc := k8s.GenerateService(k8s.ZarfNamespace, svcName)
	svc.Spec.Selector = map[string]string{"app": "agent-hook"}
	svc.Spec.Ports = append(svc.Spec.Ports, v1.ServicePort{
		Port:       443,
		TargetPort: intstr.FromInt(8443),
	})
	k8s.ReplaceService(svc)

	if err := k8s.ReplaceTLSSecret(k8s.ZarfNamespace, secretName, tls); err != nil {
		return fmt.Errorf("unable to add the Zarf Agent secret %s/%s to the cluster: %w", k8s.ZarfNamespace, secretName, err)
	}

	noSideEffectsV1 := admissionv1.SideEffectClassNone
	webhookPath := "/mutate/pods"
	timeout := int32(300)

	createRule := admissionv1.RuleWithOperations{
		Operations: []admissionv1.OperationType{admissionv1.Create},
		Rule: admissionv1.Rule{
			APIVersions: []string{"v1"},
			Resources:   []string{"pods"},
		},
	}

	// todo: deploy the webhook with the tls.CA value populated
	webhook := k8s.GenerateMutatingWebhook(k8s.ZarfNamespace, k8s.ZarfNamespace)
	webhook.Webhooks = append(webhook.Webhooks, admissionv1.MutatingWebhook{
		Name: webhookName,
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{
				// Only operate on zarf-managed namespaces until the webhook can create secrets
				"app.kubernetes.io/managed-by": "zarf",
			},
			MatchExpressions: []metav1.LabelSelectorRequirement{
				{
					Key:      "name",
					Operator: metav1.LabelSelectorOpNotIn,
					Values:   []string{"kube-system"},
				},
				{
					Key:      "zarf.dev/agent",
					Operator: metav1.LabelSelectorOpNotIn,
					Values:   []string{"skip", "ignore"},
				},
			},
		},

		ObjectSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key:      "zarf.dev/agent",
				Operator: metav1.LabelSelectorOpNotIn,
				Values:   []string{"skip", "ignore"},
			}},
		},

		ClientConfig: admissionv1.WebhookClientConfig{
			Service: &admissionv1.ServiceReference{
				Namespace: k8s.ZarfNamespace,
				Name:      svcName,
				Path:      &webhookPath,
			},
			CABundle: tls.CA,
		},
		Rules:                   []admissionv1.RuleWithOperations{createRule},
		SideEffects:             &noSideEffectsV1,
		TimeoutSeconds:          &timeout,
		AdmissionReviewVersions: []string{"v1beta1", "v1"},
	})

	return nil
}

// dev-build:
// 	# Skaffoled would be totally fine for this except that it seems that M1 docker go builds are insanely slow vs native making skaffold less valuable here
// 	$(eval tag := $(shell date +%s))
// 	cd ../../../ && \
// 	CGO_ENABLED=0 GOOS=linux go build -o build/zarf cli/main.go && \
// 	docker build --tag zarf-agent:$(tag) --file Dockerfile.dev . && \
// 	kind load docker-image zarf-agent:$(tag) && \
// 	sed -e 's@###ZARF_REGISTRY###\/defenseunicorns\/zarf\-agent\:v0.15@'"zarf-agent:$(tag)"'@g' < "assets/manifests/agent/deployment.yaml" | kubectl apply -f -
