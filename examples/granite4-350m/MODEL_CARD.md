# IBM Granite 4.0 350M

## Summary

Granite-4.0-350M is a compact instruction-tuned language model published by IBM under Apache-2.0. This example packages the GGUF used by Ollama's `granite4:350m` entry and imports it into an existing Ollama runtime without downloading model weights during deployment.

## Identity and immutable sources

- **IBM model:** `ibm-granite/granite-4.0-350m`
- **IBM Hugging Face revision:** `bd8a1497065c0d6ba1ef19af6b0d2b14bacf71c2`
- **IBM model card:** <https://huggingface.co/ibm-granite/granite-4.0-350m>
- **Ollama model name:** `granite4:350m`
- **Observed Ollama manifest digest:** `sha256:5eee845b49c4b72ef9e385463a1e2ea9f6f937e08b019f3bbecec12bc1927221`
- **Packaged GGUF digest:** `sha256:431e956cac90a7e4d36a9b37bb086c413491c598bb51f90ada8e0c75d0219645`
- **Packaged GGUF size:** 708,439,456 bytes
- **Prompt-template digest:** `sha256:0f6ec9740c765c3fb0dc38fe25f4d283c3bc1ede32b76f81c3a971daef20f76c`
- **License-file digest:** `sha256:cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30`

The mutable Ollama tag is not used as package input. `components.files` downloads each content-addressed blob directly and verifies its SHA-256 checksum during package creation.

## Model and runtime

- **Primary task:** Text generation
- **Packaged format:** GGUF
- **Runtime:** Ollama
- **Deployment mode:** CPU-only
- **License:** Apache-2.0

CPU-only deployment avoids accelerator VRAM requirements but does not mean the model requires no system RAM. Runtime-specific prompt configuration is supplied by the pinned Ollama template and converted into a local `Modelfile` with `PARAMETER num_gpu 0` during deployment.

## Intended uses

IBM describes the model as suitable for compact and on-device language-model use cases including summarization, text classification, extraction, question answering, retrieval-augmented generation, code-related tasks, function calling, and multilingual dialogue. This example verifies only basic local text generation.

## Limitations

- Generated output can be inaccurate and requires review.
- The example packages an Ollama GGUF, not IBM's original Safetensors files.
- The GGUF blob and prompt template are pinned independently from the upstream Hugging Face revision; no claim is made that the GGUF is reproducibly derived from that revision.
- Zarf verifies transported bytes but does not establish model quality, safety, or fitness for a use case.
- Importing a GGUF into Ollama creates a content-addressed copy in Ollama's model store. The deployment action removes the temporary staged GGUF after a successful import unless `ZARF_KEEP_MODEL_SOURCE=1` is set.
- Removing the Zarf package does not remove the imported Ollama model. Run `ollama rm granite4-zarf:350m` when desired.

## License

The package includes the Apache License 2.0 text from the pinned Ollama model manifest. Consumers remain responsible for confirming that the model and all intended uses comply with applicable terms and policy.

## Validation scope

The package flow was validated through local creation, package inspection, OCI publish/pull, deployment into an isolated Ollama runtime without external network access, and CPU-only inference.
