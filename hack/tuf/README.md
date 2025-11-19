# TUF Trusted Root Management

This directory contains tooling for managing Zarf's embedded Sigstore trusted root.

## Overview

Zarf embeds the Sigstore public trusted root to enable **offline/air-gapped signature verification** without requiring network access to `tuf-repo-cdn.sigstore.dev`.

The embedded trusted root is fetched via **TUF (The Update Framework)**, which provides:
- Cryptographic verification of the trusted root
- Protection against rollback attacks
- Secure key rotation
- Supply chain security guarantees

## How It Works

1. **At Build Time**: The trusted root is fetched via TUF and embedded in the Zarf binary
2. **At Runtime**: Zarf uses the embedded root for signature verification (no network calls)
3. **Custom Roots**: Users can override with custom trusted roots for private Sigstore deployments

## Usage

### Updating the Embedded Trusted Root

Run this command to fetch the latest trusted root via TUF:

```bash
go run hack/tuf/main.go
```

This will:
- Connect to `tuf-repo-cdn.sigstore.dev`
- Cryptographically verify the trusted root using TUF
- Write to `src/pkg/utils/data/trusted_root.json`

Then commit the updated file:

```bash
git add src/pkg/utils/data/trusted_root.json
git commit -m "chore: update embedded Sigstore trusted root"
```

### When to Update

Update the embedded trusted root:
- **Before major releases** (recommended)
- **Monthly or quarterly** (good practice)
- **When Sigstore announces trust root updates**
- **After Sigstore key rotations**

### Using Custom Trusted Roots

For private Sigstore deployments, users can provide a custom trusted root:

```bash
# Create custom trusted root for private deployment
cosign trusted-root create \
  --rekor-url https://private-rekor.example.com \
  --fulcio-url https://private-fulcio.example.com \
  --output custom_trusted_root.json

# Use with Zarf
zarf package verify my-package.tar.zst \
  --key cosign.pub \
  --trusted-root custom_trusted_root.json
```

## Architecture

### File Structure

```
hack/tuf/
├── main.go           # Tool to fetch trusted root via TUF
└── README.md         # This file

src/pkg/utils/
├── data/
│   └── trusted_root.json    # Embedded trusted root (committed to git)
├── trustedroot.go           # Trusted root selection logic
├── trustedroot_test.go      # Tests
└── cosign.go                # Verification using trusted root
```

### Priority Order

When verifying signatures, Zarf uses this priority order:

1. **Custom Path**: If `--trusted-root` flag is provided, use that file
2. **Embedded Root**: Otherwise, use the embedded trusted root (no network access)

### Air-Gap Compatibility

The embedded approach ensures **zero network calls** during verification:
- ✅ TUF fetching happens at **build time** (developer's machine or CI/CD)
- ✅ Trusted root is **embedded** in the binary
- ✅ Verification works **completely offline**
- ✅ No dependency on external TUF servers at runtime

## CI/CD Integration

### Manual Updates (Simple)

Add to your release checklist:
```bash
go run hack/tuf/main.go
git add src/pkg/utils/data/trusted_root.json
git commit -m "chore: update embedded Sigstore trusted root"
```

## Security Considerations

### Why Embed vs. Fetch at Runtime?

**Embedding provides better security for air-gapped environments:**

| Approach | Air-Gap Compatible | Supply Chain Verified | Reproducible Builds |
|----------|-------------------|----------------------|---------------------|
| Runtime TUF fetch | ❌ No (requires network) | ✅ Yes | ❌ No (non-deterministic) |
| Embedded (this approach) | ✅ Yes | ✅ Yes (at build time) | ✅ Yes |

### Trust Model

- **Build Time**: Developer/CI verifies trusted root via TUF
- **Distribution**: Trusted root embedded in binary
- **Runtime**: Users trust the embedded root (or provide their own)

This follows the same model as:
- Cosign embedding the Sigstore TUF root
- Package managers embedding distribution keys
- Operating systems embedding CA certificates

## Troubleshooting

### "failed to parse embedded trusted root"

The embedded file may be corrupted. Re-fetch it:
```bash
go run hack/tuf/main.go
```

### "custom trusted root not found"

Verify the path is correct and the file exists:
```bash
ls -la /path/to/custom_trusted_root.json
```

### Network Errors During Update

The TUF fetch requires internet access. Ensure:
- You can reach `tuf-repo-cdn.sigstore.dev`
- Firewall/proxy settings allow HTTPS
- DNS resolution is working

## References

- [Sigstore TUF Repository](https://github.com/sigstore/root-signing)
- [The Update Framework (TUF)](https://theupdateframework.io/)
- [Cosign Trusted Root Documentation](https://docs.sigstore.dev/cosign/overview/)
