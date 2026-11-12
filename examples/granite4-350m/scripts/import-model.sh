#!/bin/sh
set -eu

MODEL_SHA256=431e956cac90a7e4d36a9b37bb086c413491c598bb51f90ada8e0c75d0219645
TEMPLATE_SHA256=0f6ec9740c765c3fb0dc38fe25f4d283c3bc1ede32b76f81c3a971daef20f76c
LICENSE_SHA256=cfc7749b96f63bd31c3c42b5c471bf756814053e847c10f3eb003417bc523d30
MODEL_NAME=${GRANITE_MODEL_NAME:-granite4-zarf:350m}

SCRIPT_DIR=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
MODEL_PATH="$SCRIPT_DIR/granite4-350m.gguf"
TEMPLATE_PATH="$SCRIPT_DIR/template.tmpl"
LICENSE_PATH="$SCRIPT_DIR/LICENSE"
MODELFILE_PATH="$SCRIPT_DIR/Modelfile.generated"

sha256_file() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d ' ' -f 1
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d ' ' -f 1
  else
    echo "sha256sum or shasum is required" >&2
    exit 1
  fi
}

verify_file() {
  path=$1
  expected=$2
  actual=$(sha256_file "$path")
  if [ "$actual" != "$expected" ]; then
    echo "checksum mismatch for $path: got $actual, want $expected" >&2
    exit 1
  fi
}

if ! command -v ollama >/dev/null 2>&1; then
  echo "ollama must be installed before this package is deployed" >&2
  exit 1
fi

verify_file "$MODEL_PATH" "$MODEL_SHA256"
verify_file "$TEMPLATE_PATH" "$TEMPLATE_SHA256"
verify_file "$LICENSE_PATH" "$LICENSE_SHA256"

{
  printf 'FROM %s\n' "$MODEL_PATH"
  printf 'PARAMETER num_gpu 0\n'
  printf 'TEMPLATE """\n'
  cat "$TEMPLATE_PATH"
  printf '\n"""\n'
  printf 'LICENSE """\n'
  cat "$LICENSE_PATH"
  printf '\n"""\n'
} > "$MODELFILE_PATH"

ollama create "$MODEL_NAME" --file "$MODELFILE_PATH"
ollama show "$MODEL_NAME" >/dev/null

if [ "${ZARF_KEEP_MODEL_SOURCE:-0}" != "1" ]; then
  rm -f "$MODEL_PATH"
fi

printf 'Imported %s into Ollama\n' "$MODEL_NAME"
