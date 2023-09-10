// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

// Package k8s provides a client for interacting with a Kubernetes cluster.
package k8s

import (
	"context"
	"errors"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GetDeployment returns a single deployment by namespace and name.
func (k *K8s) GetDeployment(namespace, name string) (*appsv1.Deployment, error) {
	metaOptions := metav1.GetOptions{}
	return k.Clientset.AppsV1().Deployments(namespace).Get(context.Background(), name, metaOptions)
}

// UpdateDeployment updates the given deployment in the cluster.
func (k *K8s) UpdateDeployment(deployment *appsv1.Deployment) (*appsv1.Deployment, error) {
	metaOptions := metav1.UpdateOptions{}
	return k.Clientset.AppsV1().Deployments(deployment.Namespace).Update(context.Background(), deployment, metaOptions)
}

// WaitForDeploymentReady waits for all replicas of a deployment to be ready.
// It will wait up to 90 seconds for the deployment to be ready
// If the timeout is reached an error will be returned.
func (k *K8s) WaitForDeploymentReady(deployment *appsv1.Deployment) error {
	for count := 0; count < waitLimit; count++ {
		deployment, err := k.GetDeployment(deployment.Namespace, deployment.Name)
		if err != nil {
			return err
		}
		if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
			return nil
		}
		time.Sleep(3 * time.Second)
	}
	return errors.New("timed out waiting for deployment to be ready")
}
