/*
Copyright The ORAS Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/oras-project/oras-go/v3/errdef"
	"github.com/oras-project/oras-go/v3/registry/remote/properties"
)

// Actions used in scopes.
// Reference: https://distribution.github.io/distribution/spec/auth/scope/
const (
	// ActionPull represents generic read access for resources of the repository
	// type.
	ActionPull = "pull"

	// ActionPush represents generic write access for resources of the
	// repository type.
	ActionPush = "push"

	// ActionDelete represents the delete permission for resources of the
	// repository type.
	ActionDelete = "delete"
)

// Scope is a resource scope for a bearer token request.
//
// The zero value names no resource and is dropped by [CleanScopes].
//
// A scope usually has all three parts, as in `repository:hello-world:pull,push`.
// A registry may also issue a scope outside that grammar — public.ecr.aws
// challenges with `aws` — which is kept opaque: held in ResourceType alone,
// carrying neither a resource name nor actions, and forwarded to the token
// endpoint as it arrived.
//
// Reference: https://distribution.github.io/distribution/spec/auth/scope/
type Scope struct {
	// ResourceType is the type of the resource, such as "repository" or
	// "registry", or the whole scope when it is opaque.
	ResourceType string

	// ResourceName is the name of the resource, such as "hello-world" for a
	// repository or "catalog" for a registry. It is empty for an opaque scope.
	ResourceName string

	// Actions are the actions requested on the resource, such as
	// [ActionPull] and [ActionPush]. A wildcard action "*" absorbs the others.
	// They are empty for an opaque scope.
	//
	// The actions produced by this package are de-duplicated and sorted in
	// ascending order, which is what makes the wire form of a cleaned scope
	// deterministic. Do not mutate the Actions of a Scope obtained from this
	// package; it may be shared.
	Actions []string
}

// ParseScope parses a scope in its wire form,
// "<resource type>:<resource name>:<action>[,<action>...]".
//
// The resource name may itself contain colons; the actions are taken from after
// the last one. The parsed actions are de-duplicated and sorted, and a wildcard
// action "*" absorbs the others.
//
// A scope outside that grammar is kept as an opaque scope rather than rejected,
// since a registry may challenge with one and it is better forwarded to the
// token endpoint as it arrived than turned into a failed request:
// public.ecr.aws challenges with `aws`. Opaque is the fallback for every token
// the grammar does not cover — a single token with no colon, a scope naming no
// resource type, a two-part scope with no action list, a scope with an empty
// resource name or an empty action list, and a scope carrying a comma outside
// the action list.
//
// Only a scope that cannot round trip through the space-separated wire form is
// rejected: the empty string, and a scope containing white space. The returned
// error wraps [errdef.ErrInvalidScope].
func ParseScope(scope string) (Scope, error) {
	if scope == "" || strings.ContainsAny(scope, " \t\r\n") {
		return Scope{}, fmt.Errorf("%w: %q", errdef.ErrInvalidScope, scope)
	}
	// the fallback for anything outside the grammar: the whole token is held in
	// ResourceType and forwarded as it arrived
	opaque := Scope{ResourceType: scope}
	resourceType, rest, ok := strings.Cut(scope, ":")
	if !ok || resourceType == "" || strings.Contains(resourceType, ",") {
		return opaque, nil
	}
	i := strings.LastIndex(rest, ":")
	if i <= 0 {
		// no action list, or an empty resource name
		return opaque, nil
	}
	resourceName, actions := rest[:i], rest[i+1:]
	if strings.Contains(resourceName, ",") {
		return opaque, nil
	}
	actionList := cleanActions(strings.Split(actions, ","))
	if len(actionList) == 0 {
		return opaque, nil
	}
	return Scope{
		ResourceType: resourceType,
		ResourceName: resourceName,
		Actions:      actionList,
	}, nil
}

// String returns the wire form of s,
// "<resource type>:<resource name>:<action>[,<action>...]", or just the resource
// type for an opaque scope that carries neither a resource name nor actions.
//
// The empty string is returned for a scope that names no resource type, and for
// one that names only one of a resource name and an action, so that such a scope
// is omitted from a token request rather than serialized as a malformed one.
func (s Scope) String() string {
	if s.ResourceType == "" {
		return ""
	}
	if s.ResourceName == "" && len(s.Actions) == 0 {
		return s.ResourceType
	}
	if s.ResourceName == "" || len(s.Actions) == 0 {
		return ""
	}
	return s.ResourceType + ":" + s.ResourceName + ":" + strings.Join(s.Actions, ",")
}

// ScopeRegistryCatalog returns the scope for registry catalog access.
func ScopeRegistryCatalog() Scope {
	return Scope{
		ResourceType: "registry",
		ResourceName: "catalog",
		Actions:      []string{"*"},
	}
}

// ScopeRepository returns a repository scope with the given actions.
// The zero [Scope] is returned if the repository or the action list is empty.
//
// Reference: https://distribution.github.io/distribution/spec/auth/scope/
func ScopeRepository(repository string, actions ...string) Scope {
	actions = cleanActions(slices.Clone(actions))
	if repository == "" || len(actions) == 0 {
		return Scope{}
	}
	return Scope{
		ResourceType: "repository",
		ResourceName: repository,
		Actions:      actions,
	}
}

// AppendRepositoryScope returns a new context containing scope hints for the
// auth client to fetch bearer tokens with the given actions on the repository
// addressed by ref. If called multiple times for the same repository, the new
// actions are merged into the existing scope.
//
// For example, uploading blob to the repository "hello-world" does HEAD request
// first then POST and PUT. The HEAD request will return a challenge for scope
// `repository:hello-world:pull`, and the auth client will fetch a token for
// that challenge. Later, the POST request will return a challenge for scope
// `repository:hello-world:push`, and the auth client will fetch a token for
// that challenge again. By invoking AppendRepositoryScope with the actions
// [ActionPull] and [ActionPush] for the repository `hello-world`,
// the auth client with cache is hinted to fetch a token via a single token
// fetch request for all the HEAD, POST, PUT requests.
func AppendRepositoryScope(ctx context.Context, ref properties.Reference, actions ...string) context.Context {
	if len(actions) == 0 {
		return ctx
	}
	scope := ScopeRepository(ref.Repository, actions...)
	return AppendScopesForResource(ctx, ref.Resource(), scope)
}

// scopesForRegistryContextKey is the context key for the scope hints of one
// registry. The value is a scopeHints map.
type scopesForRegistryContextKey string

// scopeHints holds the scope hints registered for a registry, keyed by the
// resource path they were registered for. The empty path holds the hints
// registered for the registry as a whole.
//
// The hints of one registry share a single context value, rather than taking a
// context key per resource, because a lookup has to walk the registered paths:
// a request is made against a repository, while a hint may have been registered
// against a namespace prefix of it, and a context cannot be enumerated.
type scopeHints map[string][]Scope

// WithScopesForResource returns a context with the scope hints for the given
// registry resource replaced by scopes. Scopes are de-duplicated and sorted.
//
// Scopes are used as hints for the auth client to fetch bearer tokens with
// larger scopes. A hint registered for a resource applies to every request
// against that resource and anything below it, so a hint registered for the
// whole registry applies to every request to it, while a hint registered for
// the namespace `example.com/myspace` applies to `myspace/app` but not to
// `othernamespace/app`.
//
// For example, uploading blob to the repository "hello-world" does HEAD request
// first then POST and PUT. The HEAD request will return a challenge for scope
// `repository:hello-world:pull`, and the auth client will fetch a token for
// that challenge. Later, the POST request will return a challenge for scope
// `repository:hello-world:push`, and the auth client will fetch a token for
// that challenge again. By invoking WithScopesForResource with the scope
// returned by ScopeRepository("hello-world", ActionPull, ActionPush), the auth
// client with cache is hinted to fetch a token via a single token fetch request
// for all the HEAD, POST, PUT requests.
//
// Passing an empty list of scopes removes the scope hints registered in the
// context for this exact resource, without affecting the hints registered for
// any other resource. Hints inherited from the registry or from a namespace
// above the resource cannot be removed this way; they keep applying.
//
// Reference: https://distribution.github.io/distribution/spec/auth/scope/
func WithScopesForResource(ctx context.Context, resource properties.Resource, scopes ...Scope) context.Context {
	resource = canonicalResource(resource)
	// Copy on write: the map held by the parent context must not change.
	hints := maps.Clone(getScopeHints(ctx, resource.Registry))
	if hints == nil {
		hints = make(scopeHints, 1)
	}
	if cleaned := CleanScopes(scopes); len(cleaned) > 0 {
		hints[resource.Path] = cleaned
	} else {
		delete(hints, resource.Path)
	}
	return context.WithValue(ctx, scopesForRegistryContextKey(resource.Registry), hints)
}

// AppendScopesForResource appends additional scopes to the scopes already
// registered in the context for the given registry resource and returns a new
// context. The resulting scopes are de-duplicated and sorted.
//
// Only the scopes registered for this exact resource are appended to. Hints
// inherited from the registry or from a namespace above it keep applying, and
// are not copied down into the narrower resource.
//
// The append operation does not modify the scopes in the context passed in.
func AppendScopesForResource(ctx context.Context, resource properties.Resource, scopes ...Scope) context.Context {
	if len(scopes) == 0 {
		return ctx
	}
	resource = canonicalResource(resource)
	oldScopes := getScopeHints(ctx, resource.Registry)[resource.Path]
	return WithScopesForResource(ctx, resource, append(slices.Clone(oldScopes), scopes...)...)
}

// GetScopesForResource returns the scope hints in the context that apply to the
// given registry resource: those registered for the resource itself, plus those
// registered for the registry or for any namespace above it. The result is
// de-duplicated and sorted, and shares no memory with the context.
//
// Paths are matched by segment, so `ns` covers `ns/app` but not `nsother/app`.
// Path does not distinguish a namespace from a repository, so hints registered
// for the repository `ns/app` also apply to the repository `ns/app/sub`.
func GetScopesForResource(ctx context.Context, resource properties.Resource) []Scope {
	resource = canonicalResource(resource)
	hints := getScopeHints(ctx, resource.Registry)
	if len(hints) == 0 {
		return nil
	}
	var scopes []Scope
	for path, pathScopes := range hints {
		if scopeHintApplies(path, resource.Path) {
			scopes = append(scopes, pathScopes...)
		}
	}
	// CleanScopes allocates the returned actions, so the caller cannot reach
	// the slices held in the context through the result.
	return CleanScopes(scopes)
}

// getScopeHints returns the scope hints registered in ctx for the given
// canonical registry name, or nil if there are none.
func getScopeHints(ctx context.Context, registry string) scopeHints {
	hints, _ := ctx.Value(scopesForRegistryContextKey(registry)).(scopeHints)
	return hints
}

// scopeHintApplies reports whether hints registered for the resource path
// hintPath apply to a request against reqPath. The empty hintPath covers the
// whole registry and so applies to everything; otherwise hintPath must equal
// reqPath or be a path-segment prefix of it, so that the namespace `ns` covers
// `ns/app` but not `nsother/app`.
func scopeHintApplies(hintPath, reqPath string) bool {
	if hintPath == "" {
		return true
	}
	rest, ok := strings.CutPrefix(reqPath, hintPath)
	return ok && (rest == "" || rest[0] == '/')
}

// canonicalResource normalizes resource so that a resource registered by a
// caller compares equal to one derived from a request by requestResource.
// Without it every lookup would miss and the hints would silently stop working.
func canonicalResource(resource properties.Resource) properties.Resource {
	resource.Registry = canonicalRegistry(resource.Registry)
	return resource
}

// CleanScopes merges the actions of the scopes that share a resource type and
// resource name, de-duplicating and sorting the actions in ascending order, and
// returns the merged scopes sorted by resource type then resource name. In
// other words, the scopes passed in are de-duplicated and sorted, so the output
// of this function is deterministic.
//
// If there is a wildcard `*` in the actions of a resource, the other actions on
// that resource are ignored. An opaque scope has no actions to merge and is
// de-duplicated on its resource type alone. Scopes with an empty wire form are
// dropped — the zero [Scope], and one naming only a resource name or only an
// action — as is a named scope whose actions are all empty.
//
// The returned scopes share no memory with the scopes passed in.
func CleanScopes(scopes []Scope) []Scope {
	if len(scopes) == 0 {
		return nil
	}

	// merge the actions of the scopes sharing a resource
	type resourceKey struct {
		resourceType string
		resourceName string
	}
	merged := make(map[resourceKey]map[string]struct{}, len(scopes))
	for _, scope := range scopes {
		if scope.String() == "" {
			// drops the zero Scope and any half-named one
			continue
		}
		key := resourceKey{scope.ResourceType, scope.ResourceName}
		actionSet := merged[key]
		if actionSet == nil {
			actionSet = make(map[string]struct{}, len(scope.Actions))
			merged[key] = actionSet
		}
		// Clone before cleaning: cleanActions sorts in place, and the slice may
		// be one the caller or the context still holds.
		for _, action := range cleanActions(slices.Clone(scope.Actions)) {
			actionSet[action] = struct{}{}
		}
	}
	// reconstruct the scopes
	result := make([]Scope, 0, len(merged))
	for key, actionSet := range merged {
		if len(actionSet) == 0 {
			if key.resourceName == "" {
				// an opaque scope, which carries no actions to merge
				result = append(result, Scope{ResourceType: key.resourceType})
			}
			// otherwise a named resource whose actions were all empty
			continue
		}
		var actions []string
		for action := range actionSet {
			if action == "*" {
				actions = []string{"*"}
				break
			}
			actions = append(actions, action)
		}
		slices.Sort(actions)
		result = append(result, Scope{
			ResourceType: key.resourceType,
			ResourceName: key.resourceName,
			Actions:      actions,
		})
	}
	if len(result) == 0 {
		return nil
	}
	slices.SortFunc(result, compareScopes)
	return result
}

// compareScopes orders scopes by resource type then resource name. The actions
// are not compared: [CleanScopes] merges the scopes that agree on that pair, so
// no two scopes in a cleaned slice share one.
func compareScopes(a, b Scope) int {
	if c := strings.Compare(a.ResourceType, b.ResourceType); c != 0 {
		return c
	}
	return strings.Compare(a.ResourceName, b.ResourceName)
}

// joinScopes serializes scopes into the space-separated form used both on the
// wire and as the bearer token cache key. Scopes with an empty wire form are
// omitted.
func joinScopes(scopes []Scope) string {
	parts := make([]string, 0, len(scopes))
	for _, scope := range scopes {
		if s := scope.String(); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " ")
}

// cleanActions removes the duplicated actions and sort in ascending order.
// If there is a wildcard `*` in the action, other actions are ignored.
func cleanActions(actions []string) []string {
	// fast paths
	switch len(actions) {
	case 0:
		return nil
	case 1:
		if actions[0] == "" {
			return nil
		}
		return actions
	}

	// slow path
	slices.Sort(actions)
	n := 0
	for i := range len(actions) {
		if actions[i] == "*" {
			return []string{"*"}
		}
		if actions[i] != actions[n] {
			n++
			if n != i {
				actions[n] = actions[i]
			}
		}
	}
	n++
	if actions[0] == "" {
		if n == 1 {
			return nil
		}
		return actions[1:n]
	}
	return actions[:n]
}
