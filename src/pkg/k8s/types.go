package k8s

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type K8sLog func(string, ...any)

type K8sLabels map[string]string

type K8sClient struct {
	Clientset  *kubernetes.Clientset
	RestConfig *rest.Config
	Log        K8sLog
	Labels     K8sLabels
}

type K8sPodLookup struct {
	Namespace string `json:"namespace" jsonschema:"description=The namespace to target for data injection"`
	Selector  string `json:"selector" jsonschema:"description=The K8s selector to target for data injection"`
	Container string `json:"container" jsonschema:"description=The container to target for data injection"`
}
