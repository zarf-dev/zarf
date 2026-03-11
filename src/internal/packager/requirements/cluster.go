// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors

package requirements

import (
	"context"
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/zarf-dev/zarf/src/pkg/cluster"
	apiextclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

func validateClusterRequirements(ctx context.Context, c *cluster.Cluster, req clusterRequirements) error {
	var failures []string

	// CRDs
	if len(req.CRDs) > 0 {
		ae, err := apiextclient.NewForConfig(c.RestConfig)
		if err != nil {
			return fmt.Errorf("failed to create apiextensions client: %w", err)
		}

		for _, crdReq := range req.CRDs {
			if err := validateCRD(ctx, ae, crdReq); err != nil {
				if crdReq.Optional {
					continue
				}
				failures = append(failures, err.Error())
			}
		}
	}

	// Generic resources
	if len(req.Resources) > 0 {
		dc, err := dynamic.NewForConfig(c.RestConfig)
		if err != nil {
			return fmt.Errorf("failed to create dynamic client: %w", err)
		}

		httpClient, err := rest.HTTPClientFor(c.RestConfig)
		if err != nil {
			return fmt.Errorf("failed to create http client for rest mapper: %w", err)
		}
		rm, err := apiutil.NewDynamicRESTMapper(c.RestConfig, httpClient)
		if err != nil {
			return fmt.Errorf("failed to create rest mapper: %w", err)
		}

		for _, r := range req.Resources {
			if err := validateObjectExists(ctx, dc, rm, r); err != nil {
				if r.Optional {
					continue
				}
				failures = append(failures, err.Error())
			}
		}
	}

	// Zarf packages
	for _, p := range req.Packages {
		if err := validateDeployedPackage(ctx, c, p); err != nil {
			if p.Optional {
				continue
			}
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return &requirementsValidationError{Failures: failures}
	}
	return nil
}

func validateDeployedPackage(ctx context.Context, c *cluster.Cluster, r packageRequirement) error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("cluster package requirement has empty name")
	}

	// This is the same mechanism Zarf already uses elsewhere to read a deployed package from the cluster.
	deployed, err := c.GetDeployedPackage(ctx, r.Name)
	if err != nil {
		msg := fmt.Sprintf("cluster package %q is not deployed", r.Name)
		if r.Reason != "" {
			msg += fmt.Sprintf(" (reason: %s)", r.Reason)
		}
		return fmt.Errorf("%s: %w", msg, err)
	}

	// Presence-only requirement
	if strings.TrimSpace(r.Version) == "" {
		return nil
	}

	// Expect deployed package metadata to contain a version string (metadata.version).
	// If it's missing/unparseable, we can't validate the constraint.
	deployedVersion := strings.TrimSpace(deployed.Data.Metadata.Version) // adjust field name to match actual type
	if deployedVersion == "" {
		return fmt.Errorf("cluster package %q is deployed but has no version metadata to validate constraint %q",
			r.Name, r.Version)
	}

	v, err := semver.NewVersion(strings.TrimPrefix(deployedVersion, "v"))
	if err != nil {
		return fmt.Errorf("cluster package %q has non-semver version %q (cannot validate constraint %q): %w",
			r.Name, deployedVersion, r.Version, err)
	}

	constraint, err := semver.NewConstraint(r.Version)
	if err != nil {
		return fmt.Errorf("invalid semver constraint for cluster package %q: %q: %w", r.Name, r.Version, err)
	}

	if !constraint.Check(v) {
		msg := fmt.Sprintf("cluster package %q at %q does not satisfy constraint %q", r.Name, v.Original(), r.Version)
		if r.Reason != "" {
			msg += fmt.Sprintf(" (reason: %s)", r.Reason)
		}
		return fmt.Errorf("%s", msg)
	}

	return nil
}

func validateCRD(ctx context.Context, ae apiextclient.Interface, r crdRequirement) error {
	if strings.TrimSpace(r.Name) == "" {
		return fmt.Errorf("cluster crd requirement has empty name")
	}

	crd, err := ae.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, r.Name, metav1.GetOptions{})
	if err != nil {
		msg := fmt.Sprintf("cluster CRD %q is missing", r.Name)
		if r.Reason != "" {
			msg += fmt.Sprintf(" (reason: %s)", r.Reason)
		}
		return fmt.Errorf("%s: %w", msg, err)
	}

	// Optional version constraint: check served versions list (names like "v1", "v1beta1").
	// If you want true semver, you'll need conventions; here we treat "v1" as "1.0.0".
	if strings.TrimSpace(r.Version) == "" {
		return nil
	}

	constraint, err := semver.NewConstraint(r.Version)
	if err != nil {
		return fmt.Errorf("invalid semver constraint for CRD %q: %q: %w", r.Name, r.Version, err)
	}

	var served []*semver.Version
	for _, v := range crd.Spec.Versions {
		if !v.Served {
			continue
		}
		sv, err := k8sAPIVersionToSemver(v.Name)
		if err == nil {
			served = append(served, sv)
		}
	}

	for _, sv := range served {
		if constraint.Check(sv) {
			return nil
		}
	}

	msg := fmt.Sprintf("cluster CRD %q served versions do not satisfy constraint %q", r.Name, r.Version)
	if r.Reason != "" {
		msg += fmt.Sprintf(" (reason: %s)", r.Reason)
	}
	return fmt.Errorf("%s", msg)
}

// validateObjectExists checks existence of a specific object by GVK+name(/namespace)
// using REST mapping + dynamic client.
func validateObjectExists(
	ctx context.Context,
	dc dynamic.Interface,
	rm meta.RESTMapper,
	sel k8sResourceSelector,
) error {
	gv, err := schema.ParseGroupVersion(sel.APIVersion)
	if err != nil {
		return fmt.Errorf("invalid apiVersion %q for %s/%s: %w", sel.APIVersion, sel.Kind, sel.Name, err)
	}
	gvk := gv.WithKind(sel.Kind)

	mapping, err := rm.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return fmt.Errorf("unable to map GVK %s for %s/%s: %w", gvk.String(), sel.Kind, sel.Name, err)
	}

	var ri dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		ns := sel.Namespace
		if ns == "" {
			return fmt.Errorf("resource %s/%s is namespaced but namespace is empty", sel.Kind, sel.Name)
		}
		ri = dc.Resource(mapping.Resource).Namespace(ns)
	} else {
		ri = dc.Resource(mapping.Resource)
	}

	_, err = ri.Get(ctx, sel.Name, metav1.GetOptions{})
	if err != nil {
		msg := fmt.Sprintf("cluster resource %s %q (apiVersion=%s) is missing",
			sel.Kind, qualifiedName(sel.Namespace, sel.Name), sel.APIVersion)
		if sel.Reason != "" {
			msg += fmt.Sprintf(" (reason: %s)", sel.Reason)
		}
		return fmt.Errorf("%s: %w", msg, err)
	}

	return nil
}

func qualifiedName(ns, name string) string {
	if ns == "" {
		return name
	}
	return ns + "/" + name
}

// k8sAPIVersionToSemver converts k8s-style api versions ("v1", "v2beta1") into
// a semver-ish version. This is intentionally conservative.
// - v1 => 1.0.0
// - v2 => 2.0.0
// - v1beta1 => 1.0.0-beta.1
func k8sAPIVersionToSemver(v string) (*semver.Version, error) {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "v") {
		return nil, fmt.Errorf("version does not start with v: %q", v)
	}
	v = strings.TrimPrefix(v, "v")

	// v1
	if !strings.ContainsAny(v, "abcdefghijklmnopqrstuvwxyz") {
		return semver.NewVersion(v + ".0.0")
	}

	// v1beta1 / v1alpha2
	// split digits prefix
	i := 0
	for i < len(v) && v[i] >= '0' && v[i] <= '9' {
		i++
	}
	if i == 0 {
		return nil, fmt.Errorf("no major version digits in %q", v)
	}
	major := v[:i]
	rest := v[i:] // e.g. "beta1"
	// find trailing digits
	j := len(rest) - 1
	for j >= 0 && rest[j] >= '0' && rest[j] <= '9' {
		j--
	}
	stage := rest[:j+1] // beta/alpha
	num := rest[j+1:]   // 1/2
	if stage == "" || num == "" {
		return nil, fmt.Errorf("cannot parse stage/num from %q", v)
	}

	return semver.NewVersion(fmt.Sprintf("%s.0.0-%s.%s", major, stage, num))
}
