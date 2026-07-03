// Copyright 2022 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

package watcher

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

type DynamicInformerFactory struct {
	Client       dynamic.Interface
	ResyncPeriod time.Duration
	Indexers     cache.Indexers
	Filters      *Filters
}

func NewDynamicInformerFactory(client dynamic.Interface, resyncPeriod time.Duration) *DynamicInformerFactory {
	return &DynamicInformerFactory{
		Client:       client,
		ResyncPeriod: resyncPeriod,
		Indexers:     DefaultIndexers(),
	}
}

func (f *DynamicInformerFactory) NewInformer(ctx context.Context, mapping *meta.RESTMapping, namespace string) cache.SharedIndexInformer {
	// Unstructured example output need `"apiVersion"` and `"kind"` set.
	example := &unstructured.Unstructured{}
	example.SetGroupVersionKind(mapping.GroupVersionKind)
	return cache.NewSharedIndexInformer(
		NewFilteredListWatchFromDynamicClient(
			ctx,
			f.Client,
			mapping.Resource,
			namespace,
			f.Filters,
		),
		example,
		f.ResyncPeriod,
		f.Indexers,
	)
}

// DefaultIndexers returns the default set of cache indexers, namely the
// namespace indexer.
func DefaultIndexers() cache.Indexers {
	return cache.Indexers{
		cache.NamespaceIndex: cache.MetaNamespaceIndexFunc,
	}
}

// Filters are optional selectors for list and watch
type Filters struct {
	Labels labels.Selector
	Fields fields.Selector
}

// NewFilteredListWatchFromDynamicClient creates a new ListWatch from the
// specified client, resource, namespace, and optional filters.
func NewFilteredListWatchFromDynamicClient(
	ctx context.Context,
	client dynamic.Interface,
	resource schema.GroupVersionResource,
	namespace string,
	filters *Filters,
) *cache.ListWatch {
	optionsModifier := func(options *metav1.ListOptions) error {
		if filters == nil {
			return nil
		}
		if filters.Labels != nil {
			selector := filters.Labels
			// Merge label selectors, if both were provided
			if options.LabelSelector != "" {
				var err error
				selector, err = labels.Parse(options.LabelSelector)
				if err != nil {
					return fmt.Errorf("parsing label selector: %w", err)
				}
				selector = andLabelSelectors(selector, filters.Labels)
			}
			options.LabelSelector = selector.String()
		}
		if filters.Fields != nil {
			selector := filters.Fields
			// Merge field selectors, if both were provided
			if options.FieldSelector != "" {
				var err error
				selector, err = fields.ParseSelector(options.FieldSelector)
				if err != nil {
					return fmt.Errorf("parsing field selector: %w", err)
				}
				selector = fields.AndSelectors(selector, filters.Fields)
			}
			options.FieldSelector = selector.String()
		}
		return nil
	}
	return NewModifiedListWatchFromDynamicClient(ctx, client, resource, namespace, optionsModifier)
}

// NewModifiedListWatchFromDynamicClient creates a new ListWatch from the
// specified client, resource, namespace, and options modifier.
// Options modifier is a function takes a ListOptions and modifies the consumed
// ListOptions. Provide customized modifier function to apply modification to
// ListOptions with field selectors, label selectors, or any other desired options.
func NewModifiedListWatchFromDynamicClient(
	ctx context.Context,
	client dynamic.Interface,
	resource schema.GroupVersionResource,
	namespace string,
	optionsModifier func(*metav1.ListOptions) error,
) *cache.ListWatch {
	listFunc := func(options metav1.ListOptions) (runtime.Object, error) {
		if err := optionsModifier(&options); err != nil {
			return nil, fmt.Errorf("modifying list options: %w", err)
		}
		return client.Resource(resource).
			Namespace(namespace).
			List(ctx, options)
	}
	watchFunc := func(options metav1.ListOptions) (watch.Interface, error) {
		options.Watch = true
		if err := optionsModifier(&options); err != nil {
			return nil, fmt.Errorf("modifying watch options: %w", err)
		}
		return client.Resource(resource).
			Namespace(namespace).
			Watch(ctx, options)
	}
	return &cache.ListWatch{ListFunc: listFunc, WatchFunc: watchFunc}
}

func andLabelSelectors(selectors ...labels.Selector) labels.Selector {
	var s labels.Selector
	for _, item := range selectors {
		if s == nil {
			s = item
		} else {
			reqs, selectable := item.Requirements()
			if !selectable {
				return item // probably the nothing selector
			}
			s = s.Add(reqs...)
		}
	}
	return s
}
