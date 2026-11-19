# Migration Guide

ORAS Go v3 changes the module path and reorganizes remote-registry
configuration so that a registry owns settings shared by its repositories. It
also moves credentials out of the `auth` package and makes several interface
and authentication changes.

This guide covers migration from v2 to v3. For the older v1 to v2 migration,
see the [v2 branch migration guide](https://github.com/oras-project/oras-go/blob/v2/MIGRATION_GUIDE.md).

## Upgrade the module

Replace the v2 module requirement and imports:

```diff
-oras.land/oras-go/v2
+github.com/oras-project/oras-go/v3
```

Then update the module graph:

```sh
go get github.com/oras-project/oras-go/v3
go mod tidy
```

The v2 and v3 module paths are distinct, so they can coexist temporarily while
an application is migrated package by package.

## Breaking changes

### Registry and repository configuration

In v2, `remote.Repository` duplicated configuration such as the HTTP client,
plain-HTTP setting, warning handler, and metadata limit. In v3, a
repository points to its parent `remote.Registry`, which owns those shared
settings. Per-repository overrides such as media types and tag/referrer page
sizes remain on `remote.Repository`.

| v2 | v3 |
| --- | --- |
| `repo.Client` | `repo.Registry.Client` |
| `repo.PlainHTTP` | `repo.Registry.PlainHTTP` |
| `repo.MaxMetadataBytes` | `repo.Registry.MaxMetadataBytes` |
| `repo.HandleWarning` | `repo.Registry.HandleWarning` |
| `repo.Reference` | `repo.Reference()` |
| `repo.Reference.Repository` | `repo.RepositoryName` |
| `repo.Reference.Registry` | `repo.Registry.Reference.Registry` |
| `remote.Registry.RepositoryOptions` | fields directly on `remote.Registry` |

The standalone `remote.RepositoryOptions` type is also gone. Code that named
that type directly, or converted it with `remote.Repository(opts)`, should
configure the corresponding fields on `remote.Registry` and
`remote.Repository` instead.

Note that `Reference()` is a read-only getter; the reference can no longer be
set through it. The v2 struct-literal pattern
`&remote.Repository{Reference: ref, Client: c}` does not compile in v3;
construct the repository with `remote.NewRepository`, then configure its
shared client through `repo.Registry.Client`, as shown below.

For example:

```go
// v2
repo, err := remote.NewRepository("localhost:5000/example")
if err != nil {
	return err
}
repo.Client = client
repo.PlainHTTP = true
ref := repo.Reference
```

becomes:

```go
// v3
repo, err := remote.NewRepository("localhost:5000/example")
if err != nil {
	return err
}
repo.Registry.Client = client
repo.Registry.PlainHTTP = true
ref := repo.Reference()
```

`repo.Reference` → `repo.Reference()` is not a pure rename. In v2 the field
held the full parsed reference, including any tag or digest passed to
`remote.NewRepository`; in v3 a `remote.Repository` identifies a repository
only, so `Reference()` returns just the registry and repository name, and
`remote.NewRepository` rejects a reference carrying a tag or digest with an
error wrapping `errdef.ErrInvalidReference`. Parse such input with
`properties.NewReference` and keep the tag or digest for the operation that
needs it.

`remote.ClientBuilder.NewRegistry` and `remote.ClientBuilder.NewRepository`
are the preferred constructors when configuration comes from `registries.conf`,
Docker configuration, certificates, or policy files. They apply the builder's
client and transport rules to the primary registry and its mirrors.

### Authentication and credentials

Credential values and functions moved from `registry/remote/auth` to
`registry/remote/credentials`. The auth client field was renamed to make its
function type explicit.

| v2 | v3 |
| --- | --- |
| `auth.Credential` | `credentials.Credential` |
| `auth.EmptyCredential` | `credentials.EmptyCredential` |
| `auth.CredentialFunc` | `credentials.CredentialFunc` |
| `auth.StaticCredential` | `credentials.StaticCredentialFunc` |
| `auth.Client.Credential` | `auth.Client.CredentialFunc` |
| `credentials.Credential(store)` | `remote.NewCredentialFunc(store)` |
| `credentials.Login` / `credentials.Logout` | `remote.Login` / `remote.Logout` |
| `credentials.ServerAddressFromHostname` | `remote.ServerAddressFromHostname` |
| `credentials.ServerAddressFromRegistry` | `remote.ServerAddressFromRegistry` |
| `credentials.ErrClientTypeUnsupported` | `remote.ErrClientTypeUnsupported` |
| `auth.WithScopes(ctx, scopes...)` | `auth.WithScopesForResource(ctx, properties.Resource, scopes...)` |
| `auth.AppendScopes(ctx, scopes...)` | `auth.AppendScopesForResource(ctx, properties.Resource, scopes...)` |
| `auth.GetScopes(ctx)` | `auth.GetScopesForResource(ctx, properties.Resource)` |
| `auth.GetAllScopesForHost(ctx, host)` | `auth.GetScopesForResource(ctx, properties.Resource)` |
| `auth.WithScopesForHost(ctx, host, scopes...)` | `auth.WithScopesForResource(ctx, properties.Resource, scopes...)` |
| `auth.AppendScopesForHost(ctx, host, scopes...)` | `auth.AppendScopesForResource(ctx, properties.Resource, scopes...)` |
| `auth.GetScopesForHost(ctx, host)` | `auth.GetScopesForResource(ctx, properties.Resource)` |
| `auth.AppendRepositoryScope(ctx, registry.Reference, actions...)` | `auth.AppendRepositoryScope(ctx, properties.Reference, actions...)` |
| `string` scope, e.g. `"repository:hello-world:pull,push"` | `auth.Scope`, built with `auth.ScopeRepository` or `auth.ParseScope` |
| `auth.ScopeRepository(repo, actions...) string` | `auth.ScopeRepository(repo, actions...) auth.Scope` |
| `auth.ScopeRegistryCatalog` (const) | `auth.ScopeRegistryCatalog()` (function) |
| `auth.CleanScopes([]string) []string` | `auth.CleanScopes([]auth.Scope) []auth.Scope` |
| `auth.TokenParams.Scopes []string` | `auth.TokenParams.Scopes []auth.Scope` |

`credentials.CredentialFunc` changed as well as moving packages: its second
argument is now `properties.Resource`, rather than a `hostport string`.
Implementations should use `resource.Registry` as the canonical registry name,
not necessarily the host that was dialed.

For example, a hand-written credential function changes from:

```go
func(ctx context.Context, hostport string) (auth.Credential, error) {
	return credentialFor(hostport)
}
```

to:

```go
func(ctx context.Context, resource properties.Resource) (credentials.Credential, error) {
	return credentialFor(resource.Registry)
}
```

`remote.Logout` now takes a `properties.Resource` rather than a
registry-name string; `remote.Login` still takes a `*remote.Registry`.
`remote.Login` accepts a `credentials.Putter` and `remote.Logout` accepts a
`credentials.Deleter`; existing full stores satisfy those narrower interfaces.

Targets implementing the legacy `ParseReference(string) (registry.Reference,
error)` signature continue to receive authentication scope hints through
`oras.Tag` and `oras.TagN`. This compatibility path, together with the
deprecated `registry.Reference` type, will be removed in a future major
release. Migrate targets to `registry.ReferenceParser`; otherwise they will
eventually stop receiving those hints.

#### Scopes are values, not strings

A scope is an `auth.Scope` in v3 rather than a `"<type>:<name>:<actions>"`
string. Build one with `auth.ScopeRepository`, `auth.ScopeRegistryCatalog` or,
for a scope that arrives as a string, `auth.ParseScope`, which returns an error
wrapping `errdef.ErrInvalidScope`:

```go
// v2
scope := auth.ScopeRepository("hello-world", auth.ActionPull, auth.ActionPush)

// v3
scope := auth.ScopeRepository("hello-world", auth.ActionPull, auth.ActionPush)
scope, err := auth.ParseScope("repository:hello-world:pull,push") // from a string
```

An implementation of `auth.TokenFetcher` receives `[]auth.Scope` and serializes
them with `Scope.String()`; it no longer has to parse scope strings apart.

A malformed `scope` parameter in a registry's `WWW-Authenticate` challenge now
fails the request instead of being forwarded to the token server verbatim. A
single token with no colon is not malformed: `public.ecr.aws` challenges with
`scope="aws"`, which is parsed as an opaque scope and forwarded unchanged.

#### Scope hints are keyed by resource

Scope hints were keyed by host in v2 and in `v3.0.0-rc.1`, so every namespace of
one registry shared a single bucket. They are keyed by `properties.Resource` in
v3, matching the credential lookup. Pass the resource the request addresses,
normally from `properties.Reference.Resource()`:

```go
// hints for a whole registry, equivalent to the v2 per-host behavior
ctx = auth.WithScopesForResource(ctx, properties.Resource{Registry: "registry.example.com"}, scopes...)

// hints for one namespace, which do not ride along on requests to another
ctx = auth.WithScopesForResource(ctx, properties.Resource{
	Registry: "registry.example.com",
	Path:     "myspace",
}, scopes...)
```

A hint applies to the resource it was registered for and to everything below it,
so registry-wide hints still reach every request. Unlike the host key, the
resource `Registry` is the canonical registry name rather than the dialed host:
use `docker.io`, not `registry-1.docker.io`.

For example:

```go
// v2
client := &auth.Client{
	Credential: auth.StaticCredential("registry.example.com", auth.Credential{
		Username: "user",
		Password: "password",
	}),
}
```

becomes:

```go
// v3
client := &auth.Client{
	CredentialFunc: credentials.StaticCredentialFunc(
		"registry.example.com",
		credentials.Credential{
			Username: "user",
			Password: "password",
		},
	),
}
```

The `credentials.Store` methods now accept and return
`credentials.Credential`. Custom stores must update their method signatures.

#### Native credential-store detection

`credentials.StoreOptions.DetectDefaultNativeStore` was replaced by the
inverse `IgnoreDefaultNativeStore` option. Detection is enabled by default in
v3:

```go
// Preserve the v2 zero-value behavior, which did not detect a native store.
store, err := credentials.NewStore(path, credentials.StoreOptions{
	IgnoreDefaultNativeStore: true,
})
```

When a native store is detected, v3 uses it for the current `DynamicStore` but
does not write the detected `credsStore` value back to the Docker configuration
file.

### Bearer token flow

`auth.Client.ForceAttemptOAuth2` was replaced by `auth.Client.TokenFetcher`.
With username/password credentials, the v3 default uses the OAuth2 password
grant. If the token server rejects it with 400, 401, 404 or 405, which
registries without an OAuth2 token endpoint do, the client retries with the
distribution-spec token flow and remembers that for the realm. To preserve the
v2 default and use the distribution-spec token flow from the start, configure a
composite fetcher in legacy mode:

```go
client.TokenFetcher = auth.NewCompositeTokenFetcher(
	client.Client,
	client.Header,
	client.ClientID,
	true,
)
```

To use OAuth2 only, with no fallback, set `StrictOAuth2` on a composite
fetcher.

When using `remote.ClientBuilder`, the same choices can be made through
`properties.TokenFlowDistribution` and `properties.TokenFlowOAuth2`. A caller-supplied
`ClientBuilder.TokenFetcher` takes precedence over the registry properties.

### Registry references

`registry.Reference` now records a tag and digest separately. This preserves
both parts of a reference such as `repository:tag@digest` instead of dropping
the tag. Its `Digest()` method was renamed to `GetDigest()` because `Digest` is
now a field. For references produced by the parsers, `GetReference()` returns
the digest when present or the tag otherwise.

The legacy `registry.Reference` type and `registry.ParseReference` function are
deprecated in v3. New code should use
`registry/remote/properties.Reference` and `properties.NewReference`:

```go
ref, err := properties.NewReference("registry.example.com/team/app:v1")
if err != nil {
	return err
}
fmt.Println(ref.Registry, ref.Repository, ref.Tag)
```

The parsers also accept `oci://`, `http://`, and `https://` prefixes.

The exported reference surfaces in `registry/remote` use the properties type:

| Deprecated type | Replacement |
| --- | --- |
| `remote.Registry.Reference registry.Reference` | `remote.Registry.Reference properties.Reference` |
| `remote.Repository.Reference() registry.Reference` | `remote.Repository.Reference() properties.Reference` |
| `remote.Repository.ParseReference(string) (registry.Reference, error)` | `remote.Repository.ParseReference(string) (properties.Reference, error)` |

Custom targets that expose a `ParseReference` method should update its return
type to `properties.Reference`. ORAS temporarily recognizes the legacy method
signature when adding authentication scope hints, so existing implementations
retain that behavior during migration.

### Repository interfaces, predecessors, and untagging

`registry.Repository` now embeds `content.PredecessorFinder`. Applications
that only consume a `registry.Repository` are unaffected, but custom
implementations must add:

```go
Predecessors(ctx context.Context, node ocispec.Descriptor) ([]ocispec.Descriptor, error)
```

This method returns the descriptors that directly point to `node`.

`registry.ManifestStore` now embeds `content.Untagger`. Implementations must
add:

```go
Untag(ctx context.Context, reference string) error
```

The concrete `remote.Repository` implements this method using the OCI
Distribution Specification tag-deletion API. Untagging a digest is rejected;
the reference must be a tag.

### Referrers capability

`remote.Repository.SetReferrersCapability` no longer returns an error. The
first value wins and later conflicting calls are ignored. Code that inspected
`remote.ErrReferrersCapabilityAlreadySet` should set the value and then read
back the effective state with the new `ReferrersCapability()` getter, which
returns a `properties.ReferrersAPI` (`ReferrersAPISupported`,
`ReferrersAPIUnsupported`, or `ReferrersAPIUnknown`) rather than a `bool`:

```go
repo.SetReferrersCapability(true)
effective := repo.ReferrersCapability()
```

`remote.ErrReferrersCapabilityAlreadySet` was removed.

### File store constructors

`file.NewWithFallbackLimit` and `file.NewWithFallbackStorage` were removed.
`file.New` is unchanged; every other configuration goes through
`file.NewWithOptions`:

| v2 | v3 |
| --- | --- |
| `file.NewWithFallbackLimit(dir, limit)` | `file.NewWithOptions(dir, file.StoreOptions{FallbackLimit: limit})` |
| `file.NewWithFallbackStorage(dir, storage)` | `file.NewWithOptions(dir, file.StoreOptions{FallbackStorage: storage})` |

`FallbackStorage` and `FallbackLimit` cannot both be set, and a negative
`FallbackLimit` is rejected; both return `file.ErrInvalidStoreOptions`.
For an unlimited fallback, set `FallbackStorage: memory.New()`.

`StoreOptions.DigestAlgorithm` selects the algorithm for the digests that
`Store.Add` computes (SHA-256 by default, SHA-384 or SHA-512 otherwise).

### File store directory annotation keys

The file store marks a packed directory with two annotations: a digest of the
uncompressed tar and a flag that triggers unpacking. In v3 these keys move from
the `io.deis.oras.content` namespace, which the project does not control, to
`land.oras.content` under a domain owned by the project. The `io.deis` values
remain available as deprecated constants:

| v2 | v3 |
| --- | --- |
| `file.AnnotationDigest` = `io.deis.oras.content.digest` | `file.AnnotationDigest` = `land.oras.content.digest` |
| `file.AnnotationUnpack` = `io.deis.oras.content.unpack` | `file.AnnotationUnpack` = `land.oras.content.unpack` |
| — | `file.AnnotationDigestLegacy` = `io.deis.oras.content.digest` (deprecated) |
| — | `file.AnnotationUnpackLegacy` = `io.deis.oras.content.unpack` (deprecated) |

The existing constant names and types are unchanged, so code that references
them still compiles. However, their values have changed. Code that hardcodes
the `io.deis` strings, or that compares a descriptor's annotations against
those strings, must be updated.

The dual-key format maintains compatibility with older clients while allowing v3 to read legacy artifacts:

- `Store.Add` writes **both** key pairs for a directory. Artifacts created by
  v3 can still be unpacked by oras-go v2 and the oras CLI, which only read the
  `io.deis` keys. Without this, an older client would pull a v3 directory
  artifact and silently save it as a single `.tar.gz`.
- On unpack, the `land.oras` keys take precedence when their values are
  non-empty. For both the unpack flag and the digest, an empty value falls
  back to the other key. Artifacts pushed by earlier releases continue to
  work.
- If both unpack keys are present with non-empty values that disagree, such as
  `"true"` and `"false"`, or `"TRUE"` and `"true"`, the push fails rather than
  letting v2 and v3 unpack the same artifact differently.
- If both digest keys are set to different values and the unpack annotation is
  `true`, the push fails rather than picking one. This validation applies
  whether or not `Store.SkipUnpack` is set.
- `Store.SkipUnpack` still overrides the unpack annotation, for both the new and
  the deprecated keys. It controls extraction, not digest-annotation conflict
  validation.

Two consequences are worth planning for:

- **Manifest digests change for directory layers.** The extra annotations are
  part of the manifest that references the layer. The layer digest itself, which
  covers the compressed blob, does not change. Code that pins or compares
  manifest digests for artifacts containing a directory, including users of
  `Store.TarReproducible` and `Store.TarModTime`, will see different values
  across v2 and v3.
- **Descriptors for a directory now carry four annotations** instead of two
  (five with `org.opencontainers.image.title`). Consumers that assume a fixed
  annotation set should be checked.

The legacy annotation keys will continue to be read. Removing the legacy keys,
and eventually dropping the deprecated constants, is deferred to a later major
release.

### Removed v3 development APIs

Applications that tested against pre-release v3 snapshots may also need these
changes:

- Remove calls to `credentials.SetDefaultConfigLoader`. The credentials
  package now loads configuration directly.
- Remove uses of `credentials.ConfigFileLoader` and
  `credentials.ErrNoConfigLoader`; neither has a replacement.
- Remove the `SetCredentialsStore` and `Save` methods from custom
  implementations of `credentials.ConfigFile` (an interface introduced during
  v3 development); both were dropped from the interface.
- Return a second value from `GetCredentialHelper` in custom implementations
  of `credentials.ConfigFile`: whether the config has an entry for the server.
  An entry that exists but is empty selects the config file, not `credsStore`.
- Remove the `force-basic-auth` key from `registries.conf` and uses of
  `properties.Attributes.ForceBasicAuth`. The option was never consumed. The
  registry's `WWW-Authenticate` challenge selects the authentication scheme;
  `token-flow` selects how a bearer token is acquired.
- Replace `remote.NewRegistryWithProperties(props, builder)` and
  `remote.NewRepositoryWithProperties(props, builder)` with
  `builder.NewRegistry(props)` and `builder.NewRepository(props)`. A nil
  builder used to mean `remote.NewClientBuilder()`; call it explicitly.
- Remove uses of `oras.CopyOptions.PolicyCheck` and run the check before
  calling `oras.Copy` instead.
- Remove uses of `remote.RepositoryMiddleware` and `remote.Compose`; neither
  has a replacement.
- Remove uses of `remote.WithPolicyEnforcement`. Configure policy through
  `remote.ClientBuilder` and `remote.Registry.Policy` instead.
- `remote.ClientBuilder.CredentialStore` narrowed from `credentials.Store` to
  `credentials.Getter`. Existing stores remain valid; a read-only credential
  provider can now be used as well.
- Update custom `auth.TokenFetcher` implementations:
  `auth.TokenParams.Registry string` became
  `auth.TokenParams.Resource properties.Resource`.
- `properties.Registry.Credential` changed type from `credentials.Credential`
  to `properties.Credential`.
- The pre-GA forms of `auth.ScopeRepository`, `auth.ScopeRegistryCatalog`,
  `auth.CleanScopes`, and `auth.TokenParams.Scopes` also changed before v3 GA,
  and the `*ScopesForHost` helpers were replaced by `*ScopesForResource`. See
  the authentication migration table above when updating code written against
  a release candidate.

## Observable behavior changes

- `content/file` writes the directory annotation keys under both `land.oras` and
  `io.deis`, and reads the former in preference to the latter. Manifests that
  reference a directory layer therefore have a different digest in v3 than in
  v2. See [File store directory annotation keys](#file-store-directory-annotation-keys).
- `remote.NewRepository` returns an error wrapping `errdef.ErrInvalidReference`
  when the reference contains a tag or digest (e.g.
  `localhost:5000/example:v1`). In v2 such a reference was accepted and the
  tag or digest was kept on `repo.Reference`.
- `oras.CopyError` includes a `Descriptor` when a copy operation had already
  selected content. Its error string includes the digest in those cases. Use
  `errors.As` and the structured fields rather than comparing error strings.
- Registry repository listing can be bounded with `RepositoryListMaxPages`.
  Its zero value is unlimited; exceeding a configured limit wraps
  `errdef.ErrTooManyPages`. The existing tag and referrer page limits now also
  serve as defaults on `remote.Registry`, with repository-level overrides.
- A detected native credential store is no longer persisted to a shared Docker
  configuration file as a side effect of `DynamicStore.Put`.
- `oras.ExtendedCopy` and `oras.ExtendedCopyGraph` look up predecessors
  concurrently, up to `Concurrency` lookups at a time, so
  `ExtendedCopyGraphOptions.FindPredecessors` and the source storage it queries
  may be called from several goroutines. Implementations that keep state must
  synchronize access to it. In v2 the lookups ran one node at a time.
- The default retry policy honors the `Retry-After` response header of any
  retryable response (including 429 and 503), in both delta-seconds and
  HTTP-date forms, up to the new `retry.GenericPolicy.MaxRetryAfter`
  (60 seconds by default). `MaxWait` is raised from 3 seconds to 10 seconds,
  so the worst-case total wait rises from ~7 seconds to ~5 minutes for
  callers without a context deadline. Callers who want the old fail-fast
  behavior can set `Transport.Policy` or `ClientBuilder.RetryPolicy` to a
  `retry.GenericPolicy` literal with the old values.
- `oci.Store.GC` now keeps unreachable blobs modified within the last hour,
  matching the grace period already applied to `ingest/` and temporary metadata
  files so in-flight multi-step copies do not lose freshly pushed layers. What
  a kept manifest pushed through the same `Store` references is kept with it,
  whatever its modification time, so a copy that skipped pushing a layer
  already on disk does not lose it.
  Unreachable blobs modified less than an hour before `GC` are reclaimed on a
  subsequent `GC` after the grace window expires.

## Major additions in v3

The headline additions are:

- **Unified configuration:** `registry/remote/config.LoadConfigs` loads Docker
  credentials, containers `auth.json`, `registries.conf`, `policy.json`,
  `registries.d`, and certificate directories.
- **Configured clients and mirrors:** `remote.ClientBuilder` builds clients,
  registries, and repositories from that configuration. Reads try applicable
  mirrors and fall back to the primary.
- **Policy and signatures:** `registry/remote/policy` enforces allow/deny
  policy, and `registry/remote/signature` verifies OpenPGP simple signatures.
- **Content verification and new push modes:** verify content without writing
  it, and push streams of unknown digest or size, in chunks, or in a single
  request.

The [v3.0.0 release notes](https://github.com/oras-project/oras-go/releases/tag/v3.0.0)
hold the complete list.

### ORAS-specific `registries.conf` keys

Besides the keys defined by [containers/image](https://github.com/containers/image/blob/main/docs/containers-registries.conf.5.md), ORAS reads optional per-`[[registry]]` fields such as `token-flow`, `referrers-api`, and `blocked-realm-prefixes`. Other tools may ignore them.

`blocked-realm-prefixes` sets the bearer realm address denylist for that registry entry (see `auth.Client.BlockedRealmPrefixes`). Each value is a CIDR prefix, a bare IP address (`/32` or `/128`), or a named set:

- `default` — `auth.DefaultBlockedRealmPrefixes()`
- `private` — `auth.PrivateRealmPrefixes()`
- `none` — block nothing; must be the only entry

When the key is **absent**, the library default denylist applies. When it is present as an **empty array** (`[]`) or `["none"]`, nothing is blocked. Values replace the defaults; combine explicit prefixes with `default` or `private` when you want those lists included.

`mergeRegistriesConfig` replaces a whole `[[registry]]` row when a drop-in uses the same `prefix`. A drop-in that only sets `insecure = true` does not merge field-by-field, so a `blocked-realm-prefixes` list from the base file is dropped unless the drop-in sets it again.

Mirror clients inherit `blocked-realm-prefixes` from the primary's properties, like `token-flow` and `referrers-api`.

For current API documentation and examples, see
[pkg.go.dev](https://pkg.go.dev/github.com/oras-project/oras-go/v3).

## FAQs

### Can v2 and v3 be imported by the same application?

Yes. They have different module paths. This can make an incremental migration
easier, but values from the two majors have distinct Go types and must not be
mixed without explicit conversion.

### Where is the v1 to v2 guide?

It remains on the
[`v2` branch](https://github.com/oras-project/oras-go/blob/v2/MIGRATION_GUIDE.md),
next to the version it documents.

## Community Support

If you encounter challenges during migration, seek assistance from the
community by [submitting a GitHub issue](https://github.com/oras-project/oras-go/issues/new)
or asking in the [#oras](https://cloud-native.slack.com/archives/CJ1KHJM5Z)
Slack channel.
