# Package Granite 4.0 350M for Ollama

This example uses native Zarf `components.files` to package a pinned IBM Granite 4.0 350M GGUF, its Ollama prompt template, and its Apache-2.0 license. Deployment imports the local GGUF into an existing Ollama runtime; no model weight is downloaded during deployment.

## Prerequisites

- Zarf v1.0.0 or newer
- A POSIX shell with `sha256sum` or `shasum`
- Ollama installed and running on the deployment host
- Network access to `registry.ollama.ai` only while creating the package

The example was developed against Ollama 0.40.15. Other versions must support importing this Granite 4 GGUF with `ollama create`.

## Build on the connected system

Run from this directory:

```bash
zarf package create . --confirm --skip-sbom
```

Zarf downloads three content-addressed Ollama blobs and verifies the SHA-256 checksum declared for each `components.files` entry. The 708,439,456-byte GGUF is included once in the package.

Inspect the package before transfer:

```bash
PACKAGE=$(printf '%s\n' zarf-package-granite4-350m-*.tar.zst)
zarf package inspect definition "$PACKAGE"
zarf package inspect digest "$PACKAGE"
zarf package inspect documentation "$PACKAGE" --output ./documentation
```

The documentation output contains `README.md` and `MODEL_CARD.md` without deploying or extracting the model weight.

## Publish and pull through an OCI registry

When a test registry is available:

```bash
zarf package publish "$PACKAGE" oci://REGISTRY/NAMESPACE
zarf package pull oci://REGISTRY/NAMESPACE/granite4-350m:1.0.0
```

Publishing is optional. The package tarball is already a transferable disconnected artifact. Do not publish this example to a shared registry without authorization.

## Transfer into the disconnected environment

Move `zarf-package-granite4-350m-<architecture>-1.0.0.tar.zst` using approved removable media or another controlled transfer path. The disconnected host does not need access to Hugging Face or the Ollama registry.

Validate the transferred bytes using the digest recorded on the connected system:

```bash
zarf package inspect digest zarf-package-granite4-350m-*.tar.zst
```

## Deploy into Ollama

Start or identify an isolated Ollama runtime. `OLLAMA_HOST` is passed through to the deployment action:

```bash
export OLLAMA_HOST=127.0.0.1:11434
zarf package deploy zarf-package-granite4-350m-*.tar.zst --confirm
```

The deployment action:

1. receives the model, template, license, and import script from the local Zarf package;
2. rechecks their SHA-256 digests;
3. builds a local Ollama `Modelfile` using the pinned GGUF, prompt template, license, and `num_gpu 0` CPU-only parameter;
4. runs `ollama create granite4-zarf:350m`;
5. confirms the imported model with `ollama show`; and
6. removes the temporary staged GGUF after successful import.

Set `ZARF_KEEP_MODEL_SOURCE=1` before deployment to retain the staged GGUF for debugging. Set `GRANITE_MODEL_NAME` to override the imported model name.

## Verify inference

```bash
ollama show granite4-zarf:350m
ollama run granite4-zarf:350m "What is 2 plus 2?"
ollama ps
```

For a non-streaming API check:

```bash
curl --fail --silent --show-error http://${OLLAMA_HOST}/api/generate \
  --header 'Content-Type: application/json' \
  --data '{"model":"granite4-zarf:350m","prompt":"What is 2 plus 2?","stream":false,"options":{"temperature":0,"seed":42,"num_predict":32}}'
```

`ollama ps` identifies whether the runtime actually used CPU, GPU, or a split. This example's generated `Modelfile` explicitly requests CPU-only execution with `PARAMETER num_gpu 0`.

## Clean up

The imported Ollama model is runtime state and is not removed automatically with the Zarf package:

```bash
ollama rm granite4-zarf:350m
```

Zarf manages the remaining staged configuration files under `.zarf/models/granite4-350m/`.

## Package metadata and documentation

The package uses standard Zarf metadata and OCI annotations for identity, source revision, and license. Its `attachments` map exposes this guide and `MODEL_CARD.md` through `zarf package inspect documentation` without introducing a custom package kind or OCI media type.

The Ollama template and import behavior remain deployment configuration. Detailed source, integrity, limitations, and validation context remain in `MODEL_CARD.md`.

## Extending the pattern

- **Larger or sharded LLMs:** add one checksum-pinned `components.files` entry per shard and update the importer to reference all shards. Package splitting with `--max-package-size` can support constrained transfer media. Runtime storage and temporary import duplication become more significant as weights grow.
- **Applied AI and ONNX:** package a checksum-pinned `.onnx` model plus labels and preprocessing configuration. Replace the Ollama import action with the target ONNX Runtime application or service integration.

## Known limitations

- Package creation requires network access unless the pinned files are already cached locally.
- Ollama must already be installed; its binary is intentionally not bundled.
- Import briefly requires space for both the staged GGUF and Ollama's content-addressed copy.
- A file-based Zarf package does not prove how the Ollama GGUF was derived from IBM's original Safetensors revision.
