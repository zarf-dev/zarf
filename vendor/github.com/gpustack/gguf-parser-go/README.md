# GGUF Parser

> tl;dr, Review/Check [GGUF](https://github.com/ggerganov/ggml/blob/master/docs/gguf.md) files and estimate the memory
> usage.

[![Go Report Card](https://goreportcard.com/badge/github.com/gpustack/gguf-parser-go)](https://goreportcard.com/report/github.com/gpustack/gguf-parser-go)
[![CI](https://img.shields.io/github/actions/workflow/status/gpustack/gguf-parser-go/cmd.yml?label=ci)](https://github.com/gpustack/gguf-parser-go/actions)
[![License](https://img.shields.io/github/license/gpustack/gguf-parser-go?label=license)](https://github.com/gpustack/gguf-parser-go#license)
[![Download](https://img.shields.io/github/downloads/gpustack/gguf-parser-go/total)](https://github.com/gpustack/gguf-parser-go/releases)
[![Docker Pulls](https://img.shields.io/docker/pulls/gpustack/gguf-parser)](https://hub.docker.com/r/gpustack/gguf-parser)
[![Release](https://img.shields.io/github/v/release/gpustack/gguf-parser-go)](https://github.com/gpustack/gguf-parser-go/releases/latest)

[GGUF](https://github.com/ggerganov/ggml/blob/master/docs/gguf.md) is a file format for storing models for inference
with GGML and executors based on GGML. GGUF is a binary format that is designed for fast loading and saving of models,
and for ease of reading. Models are traditionally developed using PyTorch or another framework, and then converted to
GGUF for use in GGML.

GGUF Parser helps in reviewing and estimating the usage and maximum tokens per second of a GGUF format model without
download it.

## Key Features

- **No File Required**: GGUF Parser uses chunking reading to parse the metadata of remote GGUF file, which means you
  don't need to download the entire file and load it.
- **Accurate Prediction**: The evaluation results of GGUF Parser usually deviate from the actual usage by about 100MiB.
- **Quick Verification**: You can provide device metrics to calculate the maximum tokens per second (TPS) without
  running the model.
- **Type Screening**: GGUF Parser can distinguish what the GGUF file used for, such as Embedding, Reranking, LoRA, etc.
- **Fast**: GGUF Parser is written in Go, which is fast and efficient.

## Agenda

- [Notes](#notes)
- [Installation](#installation)
- [Overview](#overview)
    + [Parse](#parse)
        * [Local File](#parse-local-file)
        * [Remote File](#parse-remote-file)
        * [From HuggingFace](#parse-from-huggingface)
        * [From ModelScope](#parse-from-modelscope)
        * [From Ollama Library](#parse-from-ollama-library)
        * [Others](#others)
            * [Image Model](#parse-image-model)
            * [None Model](#parse-none-model)
    + [Estimate](#estimate)
        * [Across Multiple GPU devices](#across-multiple-gpu-devices)
        * [Maximum Tokens Per Second](#maximum-tokens-per-second)
        * [Full Layers Offload (default)](#full-layers-offload-default)
        * [Zero Layers Offload](#zero-layers-offload)
        * [Specific Layers Offload](#specific-layers-offload)
        * [Specific Context Size](#specific-context-size)
        * [Enable Flash Attention](#enable-flash-attention)
        * [Disable MMap](#disable-mmap)
        * [With Adapter](#with-adapter)
        * [Get Proper Offload Layers](#get-proper-offload-layers)

## Notes

- **Since v0.20.0**, GGUF Parser supports leveraging `--override-tensor` to indicate how to place the model tensors.
- **Since v0.19.0**, GGUF Parser supports estimating Audio projector model file, like Ultravox series, Qwen2 Audio
  series, etc.
- **Since v0.18.0**, GGUF Parser supports estimating SWA-supported(sliding window attention) model file, like LLaMA 4
  series, Gemma2/3 series, etc.
- **Since v0.17.0**, GGUF Parser align the `QUANTIZATION`(
  aka. [`general.file_type`](https://github.com/ggml-org/ggml/blob/master/docs/gguf.md#general-metadata))
  to [HuggingFace processing](https://github.com/huggingface/huggingface.js/blob/2475d6d316135c0a4fceff6b3fe2aed0dde36ac1/packages/gguf/src/types.ts#L11-L48),
  but there are still many model files whose naming does not fully follow `general.file_type`.
- **Since v0.16.0**, GGUF Parser supports estimating MLA-supported model file, like DeepSeek series.
- **Since v0.14.0 (BREAKING CHANGE)**, GGUF Parser parses `*.feed_forward_length` metadata as `[]uint64`,
  which means the architecture `feedForwardLength` is a list of integers.
- **Since v0.13.0 (BREAKING CHANGE)**, GGUF Parser can parse files
  for [StableDiffusion.Cpp](https://github.com/leejet/stable-diffusion.cpp) or StableDiffusion.Cpp like application.
    + [LLaMA Box](https://github.com/gpustack/llama-box) is able to offload different components of the all-in-one model
      to different devices, e.g. with `-ts 1,1,1`, GGUF Parser return the usage of Text Encoder Models in 1st device,
      VAE Model in 2nd device, and Diffusion Model in 3rd device.
- Experimentally, GGUF Parser can estimate the maximum tokens per second(`MAX TPS`) for a (V)LM model according to the
  `--device-metric` options.
- GGUF Parser distinguishes the remote devices from `--tensor-split` via `--rpc`.
    + For one host multiple GPU devices, you can use `--tensor-split` to get the estimated memory usage of each GPU.
    + For multiple hosts multiple GPU devices, you can use `--tensor-split` and `--rpc` to get the estimated memory
      usage of each GPU. Since v0.11.0, `--rpc` flag masks the devices specified by `--tensor-split` in front.
- Table result usage:
    + `DISTRIBUTABLE` indicates the GGUF file supports distribution inference or not, if the file doesn't support
      distribution inference, you can not offload it
      with [RPC servers](https://github.com/ggerganov/llama.cpp/tree/master/examples/rpc).
    + `RAM` indicates the system memory usage.
    + `VRAM *` indicates the local GPU memory usage.
    + `RPC * (V)RAM` indicates the remote memory usage. The kind of memory is determined by which backend the RPC server
      uses, check the running logs for more details.
    + `UMA` indicates the memory usage of Apple macOS only. `NONUMA` adapts to other cases, including non-GPU devices.
    + `LAYERS`(`I`/`T`/`O`) indicates the count for input layers, transformer layers, and output layers. Input layers
      are not offloaded at present.

## Installation

Install from [releases](https://github.com/gpustack/gguf-parser-go/releases).

## Overview

### Parse

#### Parse Local File

```shell
$ gguf-parser --path ~/.cache/lm-studio/models/unsloth/DeepSeek-R1-Distill-Qwen-7B-GGUF/DeepSeek-R1-Distill-Qwen-7B-Q4_K_M.gguf
+-----------------------------------------------------------------------------------------------------------+
| METADATA                                                                                                  |
+-------+-------------------------+-------+--------------+---------------+----------+------------+----------+
|  TYPE |           NAME          |  ARCH | QUANTIZATION | LITTLE ENDIAN |   SIZE   | PARAMETERS |    BPW   |
+-------+-------------------------+-------+--------------+---------------+----------+------------+----------+
| model | DeepSeek R1 Distill ... | qwen2 |    Q4_K_M    |      true     | 4.36 GiB |   7.62 B   | 4.91 bpw |
+-------+-------------------------+-------+--------------+---------------+----------+------------+----------+

+-----------------------------------------------------------------------------------------------------------------------------------+
| ARCHITECTURE                                                                                                                      |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
| MAX CONTEXT LEN | EMBEDDING LEN | ATTENTION CAUSAL | ATTENTION HEAD CNT | LAYERS | FEED FORWARD LEN | EXPERT CNT | VOCABULARY LEN |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
|      131072     |      3584     |       true       |         28         |   28   |       18944      |      0     |     152064     |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+

+-------------------------------------------------------------------------------------------------------------------------------------------------------+
| TOKENIZER                                                                                                                                             |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
| MODEL | TOKENS SIZE | TOKENS LEN | ADDED TOKENS LEN | BOS TOKEN | EOS TOKEN | EOT TOKEN | EOM TOKEN | UNKNOWN TOKEN | SEPARATOR TOKEN | PADDING TOKEN |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
|  gpt2 |   2.47 MiB  |   152064   |        N/A       |   151646  |   151643  |    N/A    |    N/A    |      N/A      |       N/A       |     151654    |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+

+-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| ESTIMATE                                                                                                                                                                                                                                        |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+----------------------------------------------+---------------------------------------+
|  ARCH | CONTEXT SIZE | BATCH SIZE (L / P) | FLASH ATTENTION | MMAP LOAD | EMBEDDING ONLY |  RERANKING  | DISTRIBUTABLE | OFFLOAD LAYERS | FULL OFFLOADED |                      RAM                     |                 VRAM 0                |
|       |              |                    |                 |           |                |             |               |                |                +--------------------+------------+------------+----------------+----------+-----------+
|       |              |                    |                 |           |                |             |               |                |                | LAYERS (I + T + O) |     UMA    |   NONUMA   | LAYERS (T + O) |    UMA   |   NONUMA  |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+------------+------------+----------------+----------+-----------+
| qwen2 |    131072    |     2048 / 512     |     Disabled    |  Enabled  |       No       | Unsupported |   Supported   |   29 (28 + 1)  |       Yes      |      1 + 0 + 0     | 677.44 MiB | 827.44 MiB |     28 + 1     | 7.30 GiB | 18.89 GiB |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+------------+------------+----------------+----------+-----------+

$ # Retrieve the model's metadata via split file,
$ # which needs all split files has been downloaded.
$  gguf-parser --path ~/.cache/lm-studio/models/Qwen/Qwen2.5-7B-Instruct-GGUF/qwen2.5-7b-instruct-q8_0-00001-of-00003.gguf 
+-------------------------------------------------------------------------------------------------------+
| METADATA                                                                                              |
+-------+---------------------+-------+--------------+---------------+----------+------------+----------+
|  TYPE |         NAME        |  ARCH | QUANTIZATION | LITTLE ENDIAN |   SIZE   | PARAMETERS |    BPW   |
+-------+---------------------+-------+--------------+---------------+----------+------------+----------+
| model | qwen2.5-7b-instruct | qwen2 |     Q8_0     |      true     | 7.54 GiB |   7.62 B   | 8.50 bpw |
+-------+---------------------+-------+--------------+---------------+----------+------------+----------+

+-----------------------------------------------------------------------------------------------------------------------------------+
| ARCHITECTURE                                                                                                                      |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
| MAX CONTEXT LEN | EMBEDDING LEN | ATTENTION CAUSAL | ATTENTION HEAD CNT | LAYERS | FEED FORWARD LEN | EXPERT CNT | VOCABULARY LEN |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
|      131072     |      3584     |       true       |         28         |   28   |       18944      |      0     |     152064     |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+

+-------------------------------------------------------------------------------------------------------------------------------------------------------+
| TOKENIZER                                                                                                                                             |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
| MODEL | TOKENS SIZE | TOKENS LEN | ADDED TOKENS LEN | BOS TOKEN | EOS TOKEN | EOT TOKEN | EOM TOKEN | UNKNOWN TOKEN | SEPARATOR TOKEN | PADDING TOKEN |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
|  gpt2 |   2.47 MiB  |   152064   |        N/A       |   151643  |   151645  |    N/A    |    N/A    |      N/A      |       N/A       |     151643    |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+

+-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| ESTIMATE                                                                                                                                                                                                                                        |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+----------------------------------------------+---------------------------------------+
|  ARCH | CONTEXT SIZE | BATCH SIZE (L / P) | FLASH ATTENTION | MMAP LOAD | EMBEDDING ONLY |  RERANKING  | DISTRIBUTABLE | OFFLOAD LAYERS | FULL OFFLOADED |                      RAM                     |                 VRAM 0                |
|       |              |                    |                 |           |                |             |               |                |                +--------------------+------------+------------+----------------+----------+-----------+
|       |              |                    |                 |           |                |             |               |                |                | LAYERS (I + T + O) |     UMA    |   NONUMA   | LAYERS (T + O) |    UMA   |   NONUMA  |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+------------+------------+----------------+----------+-----------+
| qwen2 |    131072    |     2048 / 512     |     Disabled    |  Enabled  |       No       | Unsupported |   Supported   |   29 (28 + 1)  |       Yes      |      1 + 0 + 0     | 677.44 MiB | 827.44 MiB |     28 + 1     | 7.30 GiB | 21.82 GiB |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+------------+------------+----------------+----------+-----------+
```

#### Parse Remote File

```shell
$ gguf-parser --url="https://huggingface.co/bartowski/Qwen2.5-72B-Instruct-GGUF/resolve/main/Qwen2.5-72B-Instruct-Q4_K_M.gguf"
+---------------------------------------------------------------------------------------------------------+
| METADATA                                                                                                |
+-------+----------------------+-------+--------------+---------------+-----------+------------+----------+
|  TYPE |         NAME         |  ARCH | QUANTIZATION | LITTLE ENDIAN |    SIZE   | PARAMETERS |    BPW   |
+-------+----------------------+-------+--------------+---------------+-----------+------------+----------+
| model | Qwen2.5 72B Instruct | qwen2 |    Q4_K_M    |      true     | 44.15 GiB |   72.71 B  | 5.22 bpw |
+-------+----------------------+-------+--------------+---------------+-----------+------------+----------+

+-----------------------------------------------------------------------------------------------------------------------------------+
| ARCHITECTURE                                                                                                                      |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
| MAX CONTEXT LEN | EMBEDDING LEN | ATTENTION CAUSAL | ATTENTION HEAD CNT | LAYERS | FEED FORWARD LEN | EXPERT CNT | VOCABULARY LEN |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
|      32768      |      8192     |       true       |         64         |   80   |       29568      |      0     |     152064     |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+

+-------------------------------------------------------------------------------------------------------------------------------------------------------+
| TOKENIZER                                                                                                                                             |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
| MODEL | TOKENS SIZE | TOKENS LEN | ADDED TOKENS LEN | BOS TOKEN | EOS TOKEN | EOT TOKEN | EOM TOKEN | UNKNOWN TOKEN | SEPARATOR TOKEN | PADDING TOKEN |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
|  gpt2 |   2.47 MiB  |   152064   |        N/A       |   151643  |   151645  |    N/A    |    N/A    |      N/A      |       N/A       |     151643    |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+

+--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| ESTIMATE                                                                                                                                                                                                                                         |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+----------------------------------------------+----------------------------------------+
|  ARCH | CONTEXT SIZE | BATCH SIZE (L / P) | FLASH ATTENTION | MMAP LOAD | EMBEDDING ONLY |  RERANKING  | DISTRIBUTABLE | OFFLOAD LAYERS | FULL OFFLOADED |                      RAM                     |                 VRAM 0                 |
|       |              |                    |                 |           |                |             |               |                |                +--------------------+------------+------------+----------------+-----------+-----------+
|       |              |                    |                 |           |                |             |               |                |                | LAYERS (I + T + O) |     UMA    |   NONUMA   | LAYERS (T + O) |    UMA    |   NONUMA  |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+------------+------------+----------------+-----------+-----------+
| qwen2 |     32768    |     2048 / 512     |     Disabled    |  Enabled  |       No       | Unsupported |   Supported   |   81 (80 + 1)  |       Yes      |      1 + 0 + 0     | 426.57 MiB | 576.57 MiB |     80 + 1     | 10.31 GiB | 58.18 GiB |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+------------+------------+----------------+-----------+-----------+

$ # Retrieve the model's metadata via split file

$ gguf-parser --url="https://huggingface.co/unsloth/DeepSeek-R1-GGUF/resolve/main/DeepSeek-R1-UD-IQ1_S/DeepSeek-R1-UD-IQ1_S-00001-of-00003.gguf"
+----------------------------------------------------------------------------------------------------------+
| METADATA                                                                                                 |
+-------+------------------+-----------+--------------+---------------+------------+------------+----------+
|  TYPE |       NAME       |    ARCH   | QUANTIZATION | LITTLE ENDIAN |    SIZE    | PARAMETERS |    BPW   |
+-------+------------------+-----------+--------------+---------------+------------+------------+----------+
| model | DeepSeek R1 BF16 | deepseek2 |     IQ1_S    |      true     | 130.60 GiB |  671.03 B  | 1.67 bpw |
+-------+------------------+-----------+--------------+---------------+------------+------------+----------+

+-----------------------------------------------------------------------------------------------------------------------------------+
| ARCHITECTURE                                                                                                                      |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
| MAX CONTEXT LEN | EMBEDDING LEN | ATTENTION CAUSAL | ATTENTION HEAD CNT | LAYERS | FEED FORWARD LEN | EXPERT CNT | VOCABULARY LEN |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
|      163840     |      7168     |       true       |         N/A        |   61   |       18432      |     256    |     129280     |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+

+-------------------------------------------------------------------------------------------------------------------------------------------------------+
| TOKENIZER                                                                                                                                             |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
| MODEL | TOKENS SIZE | TOKENS LEN | ADDED TOKENS LEN | BOS TOKEN | EOS TOKEN | EOT TOKEN | EOM TOKEN | UNKNOWN TOKEN | SEPARATOR TOKEN | PADDING TOKEN |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
|  gpt2 |   2.21 MiB  |   129280   |        N/A       |     0     |     1     |    N/A    |    N/A    |      N/A      |       N/A       |     128815    |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+

+--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| ESTIMATE                                                                                                                                                                                                                                         |
+-----------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------------------------------+--------------------------------------+
|    ARCH   | CONTEXT SIZE | BATCH SIZE (L / P) | FLASH ATTENTION | MMAP LOAD | EMBEDDING ONLY |  RERANKING  | DISTRIBUTABLE | OFFLOAD LAYERS | FULL OFFLOADED |                     RAM                    |                VRAM 0                |
|           |              |                    |                 |           |                |             |               |                |                +--------------------+-----------+-----------+----------------+------------+--------+
|           |              |                    |                 |           |                |             |               |                |                | LAYERS (I + T + O) |    UMA    |   NONUMA  | LAYERS (T + O) |     UMA    | NONUMA |
+-----------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+-----------+-----------+----------------+------------+--------+
| deepseek2 |    163840    |     2048 / 512     |     Disabled    |  Enabled  |       No       | Unsupported |   Supported   |   62 (61 + 1)  |       Yes      |      1 + 0 + 0     | 13.03 GiB | 13.18 GiB |     61 + 1     | 762.76 GiB |  1 TB  |
+-----------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+-----------+-----------+----------------+------------+--------+
```

#### Parse From HuggingFace

> [!NOTE]
>
> Allow using `HF_ENDPOINT` to override the default HuggingFace endpoint: `https://huggingface.co`.

```shell
$ gguf-parser --hf-repo="bartowski/Qwen2-VL-2B-Instruct-GGUF" --hf-file="Qwen2-VL-2B-Instruct-f16.gguf" --hf-mmproj-file="mmproj-Qwen2-VL-2B-Instruct-f32.gguf" --visual-max-image-size 1344
+-----------------------------------------------------------------------------------------------------------+
| METADATA                                                                                                  |
+-------+----------------------+---------+--------------+---------------+----------+------------+-----------+
|  TYPE |         NAME         |   ARCH  | QUANTIZATION | LITTLE ENDIAN |   SIZE   | PARAMETERS |    BPW    |
+-------+----------------------+---------+--------------+---------------+----------+------------+-----------+
| model | Qwen2 VL 2B Instruct | qwen2vl |      F16     |      true     | 2.88 GiB |   1.54 B   | 16.00 bpw |
+-------+----------------------+---------+--------------+---------------+----------+------------+-----------+

+-----------------------------------------------------------------------------------------------------------------------------------+
| ARCHITECTURE                                                                                                                      |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
| MAX CONTEXT LEN | EMBEDDING LEN | ATTENTION CAUSAL | ATTENTION HEAD CNT | LAYERS | FEED FORWARD LEN | EXPERT CNT | VOCABULARY LEN |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
|      32768      |      1536     |       true       |         12         |   28   |       8960       |      0     |     151936     |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+

+-------------------------------------------------------------------------------------------------------------------------------------------------------+
| TOKENIZER                                                                                                                                             |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
| MODEL | TOKENS SIZE | TOKENS LEN | ADDED TOKENS LEN | BOS TOKEN | EOS TOKEN | EOT TOKEN | EOM TOKEN | UNKNOWN TOKEN | SEPARATOR TOKEN | PADDING TOKEN |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
|  gpt2 |   2.47 MiB  |   151936   |        N/A       |   151643  |   151645  |    N/A    |    N/A    |      N/A      |       N/A       |     151643    |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+

+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| ESTIMATE                                                                                                                                                                                                                                          |
+---------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+----------------------------------------------+---------------------------------------+
|   ARCH  | CONTEXT SIZE | BATCH SIZE (L / P) | FLASH ATTENTION | MMAP LOAD | EMBEDDING ONLY |  RERANKING  | DISTRIBUTABLE | OFFLOAD LAYERS | FULL OFFLOADED |                      RAM                     |                 VRAM 0                |
|         |              |                    |                 |           |                |             |               |                |                +--------------------+------------+------------+----------------+----------+-----------+
|         |              |                    |                 |           |                |             |               |                |                | LAYERS (I + T + O) |     UMA    |   NONUMA   | LAYERS (T + O) |    UMA   |   NONUMA  |
+---------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+------------+------------+----------------+----------+-----------+
| qwen2vl |     32768    |     2048 / 512     |     Disabled    |  Enabled  |       No       | Unsupported |   Supported   |   29 (28 + 1)  |       Yes      |      1 + 0 + 0     | 236.87 MiB | 386.87 MiB |     28 + 1     | 3.65 GiB | 12.86 GiB |
+---------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+------------+------------+----------------+----------+-----------+

$ # Retrieve the model's metadata via split file

$ gguf-parser --hf-repo="bartowski/openbuddy-llama3.3-70b-v24.1-131k-GGUF" --hf-file="openbuddy-llama3.3-70b-v24.1-131k-Q4_0.gguf"
+------------------------------------------------------------------------------------------------------------+
| METADATA                                                                                                   |
+-------+-------------------------+-------+--------------+---------------+-----------+------------+----------+
|  TYPE |           NAME          |  ARCH | QUANTIZATION | LITTLE ENDIAN |    SIZE   | PARAMETERS |    BPW   |
+-------+-------------------------+-------+--------------+---------------+-----------+------------+----------+
| model | Openbuddy Llama3.3 7... | llama |     Q4_0     |      true     | 37.35 GiB |   70.55 B  | 4.55 bpw |
+-------+-------------------------+-------+--------------+---------------+-----------+------------+----------+

+-----------------------------------------------------------------------------------------------------------------------------------+
| ARCHITECTURE                                                                                                                      |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
| MAX CONTEXT LEN | EMBEDDING LEN | ATTENTION CAUSAL | ATTENTION HEAD CNT | LAYERS | FEED FORWARD LEN | EXPERT CNT | VOCABULARY LEN |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
|      131072     |      8192     |       true       |         64         |   80   |       28672      |      0     |     128256     |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+

+-------------------------------------------------------------------------------------------------------------------------------------------------------+
| TOKENIZER                                                                                                                                             |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
| MODEL | TOKENS SIZE | TOKENS LEN | ADDED TOKENS LEN | BOS TOKEN | EOS TOKEN | EOT TOKEN | EOM TOKEN | UNKNOWN TOKEN | SEPARATOR TOKEN | PADDING TOKEN |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
|  gpt2 |    2 MiB    |   128256   |        N/A       |   128000  |   128048  |    N/A    |    N/A    |      N/A      |       N/A       |     128044    |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+

+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| ESTIMATE                                                                                                                                                                                                                                    |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+-----------------------------------------+----------------------------------------+
|  ARCH | CONTEXT SIZE | BATCH SIZE (L / P) | FLASH ATTENTION | MMAP LOAD | EMBEDDING ONLY |  RERANKING  | DISTRIBUTABLE | OFFLOAD LAYERS | FULL OFFLOADED |                   RAM                   |                 VRAM 0                 |
|       |              |                    |                 |           |                |             |               |                |                +--------------------+---------+----------+----------------+-----------+-----------+
|       |              |                    |                 |           |                |             |               |                |                | LAYERS (I + T + O) |   UMA   |  NONUMA  | LAYERS (T + O) |    UMA    |   NONUMA  |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+---------+----------+----------------+-----------+-----------+
| llama |    131072    |     2048 / 512     |     Disabled    |  Enabled  |       No       | Unsupported |   Supported   |   81 (80 + 1)  |       Yes      |      1 + 0 + 0     | 1.06 GB | 1.13 GiB |     80 + 1     | 40.26 GiB | 93.62 GiB |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+---------+----------+----------------+-----------+-----------+
```

#### Parse From ModelScope

> [!NOTE]
>
> Allow using `MS_ENDPOINT` to override the default ModelScope endpoint: `https://modelscope.cn`.

```shell
$ gguf-parser --ms-repo="unsloth/DeepSeek-R1-Distill-Qwen-7B-GGUF" --ms-file="DeepSeek-R1-Distill-Qwen-7B-F16.gguf"
+-------------------------------------------------------------------------------------------------------------+
| METADATA                                                                                                    |
+-------+-------------------------+-------+--------------+---------------+-----------+------------+-----------+
|  TYPE |           NAME          |  ARCH | QUANTIZATION | LITTLE ENDIAN |    SIZE   | PARAMETERS |    BPW    |
+-------+-------------------------+-------+--------------+---------------+-----------+------------+-----------+
| model | DeepSeek R1 Distill ... | qwen2 |      F16     |      true     | 14.19 GiB |   7.62 B   | 16.00 bpw |
+-------+-------------------------+-------+--------------+---------------+-----------+------------+-----------+

+-----------------------------------------------------------------------------------------------------------------------------------+
| ARCHITECTURE                                                                                                                      |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
| MAX CONTEXT LEN | EMBEDDING LEN | ATTENTION CAUSAL | ATTENTION HEAD CNT | LAYERS | FEED FORWARD LEN | EXPERT CNT | VOCABULARY LEN |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
|      131072     |      3584     |       true       |         28         |   28   |       18944      |      0     |     152064     |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+

+-------------------------------------------------------------------------------------------------------------------------------------------------------+
| TOKENIZER                                                                                                                                             |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
| MODEL | TOKENS SIZE | TOKENS LEN | ADDED TOKENS LEN | BOS TOKEN | EOS TOKEN | EOT TOKEN | EOM TOKEN | UNKNOWN TOKEN | SEPARATOR TOKEN | PADDING TOKEN |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
|  gpt2 |   2.47 MiB  |   152064   |        N/A       |   151646  |   151643  |    N/A    |    N/A    |      N/A      |       N/A       |     151654    |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+

+-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| ESTIMATE                                                                                                                                                                                                                                        |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+----------------------------------------------+---------------------------------------+
|  ARCH | CONTEXT SIZE | BATCH SIZE (L / P) | FLASH ATTENTION | MMAP LOAD | EMBEDDING ONLY |  RERANKING  | DISTRIBUTABLE | OFFLOAD LAYERS | FULL OFFLOADED |                      RAM                     |                 VRAM 0                |
|       |              |                    |                 |           |                |             |               |                |                +--------------------+------------+------------+----------------+----------+-----------+
|       |              |                    |                 |           |                |             |               |                |                | LAYERS (I + T + O) |     UMA    |   NONUMA   | LAYERS (T + O) |    UMA   |   NONUMA  |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+------------+------------+----------------+----------+-----------+
| qwen2 |    131072    |     2048 / 512     |     Disabled    |  Enabled  |       No       | Unsupported |   Supported   |   29 (28 + 1)  |       Yes      |      1 + 0 + 0     | 677.44 MiB | 827.44 MiB |     28 + 1     | 7.30 GiB | 27.99 GiB |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+------------+------------+----------------+----------+-----------+
```

#### Parse From Ollama Library

> [!NOTE]
>
> Allow using `--ol-base-url` to override the default Ollama registry endpoint: `https://registry.ollama.ai`.

```shell
$ gguf-parser --ol-model="llama3.3"
+------------------------------------------------------------------------------------------------------------+
| METADATA                                                                                                   |
+-------+-------------------------+-------+--------------+---------------+-----------+------------+----------+
|  TYPE |           NAME          |  ARCH | QUANTIZATION | LITTLE ENDIAN |    SIZE   | PARAMETERS |    BPW   |
+-------+-------------------------+-------+--------------+---------------+-----------+------------+----------+
| model | Llama 3.1 70B Instru... | llama |    Q4_K_M    |      true     | 39.59 GiB |   70.55 B  | 4.82 bpw |
+-------+-------------------------+-------+--------------+---------------+-----------+------------+----------+

+-----------------------------------------------------------------------------------------------------------------------------------+
| ARCHITECTURE                                                                                                                      |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
| MAX CONTEXT LEN | EMBEDDING LEN | ATTENTION CAUSAL | ATTENTION HEAD CNT | LAYERS | FEED FORWARD LEN | EXPERT CNT | VOCABULARY LEN |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
|      131072     |      8192     |       true       |         64         |   80   |       28672      |      0     |     128256     |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+

+-------------------------------------------------------------------------------------------------------------------------------------------------------+
| TOKENIZER                                                                                                                                             |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
| MODEL | TOKENS SIZE | TOKENS LEN | ADDED TOKENS LEN | BOS TOKEN | EOS TOKEN | EOT TOKEN | EOM TOKEN | UNKNOWN TOKEN | SEPARATOR TOKEN | PADDING TOKEN |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
|  gpt2 |    2 MiB    |   128256   |        N/A       |   128000  |   128009  |    N/A    |    N/A    |      N/A      |       N/A       |      N/A      |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+

+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| ESTIMATE                                                                                                                                                                                                                                    |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+-----------------------------------------+----------------------------------------+
|  ARCH | CONTEXT SIZE | BATCH SIZE (L / P) | FLASH ATTENTION | MMAP LOAD | EMBEDDING ONLY |  RERANKING  | DISTRIBUTABLE | OFFLOAD LAYERS | FULL OFFLOADED |                   RAM                   |                 VRAM 0                 |
|       |              |                    |                 |           |                |             |               |                |                +--------------------+---------+----------+----------------+-----------+-----------+
|       |              |                    |                 |           |                |             |               |                |                | LAYERS (I + T + O) |   UMA   |  NONUMA  | LAYERS (T + O) |    UMA    |   NONUMA  |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+---------+----------+----------------+-----------+-----------+
| llama |    131072    |     2048 / 512     |     Disabled    |  Enabled  |       No       | Unsupported |   Supported   |   81 (80 + 1)  |       Yes      |      1 + 0 + 0     | 1.06 GB | 1.13 GiB |     80 + 1     | 40.26 GiB | 95.86 GiB |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+---------+----------+----------------+-----------+-----------+

$ # Ollama Model includes the preset params and other artifacts, like multimodal projectors or LoRA adapters, 
$ # you can get the usage of Ollama running by using `--ol-usage` option.

+------------------------------------------------------------------------------------------------------------+
| METADATA                                                                                                   |
+-------+-------------------------+-------+--------------+---------------+-----------+------------+----------+
|  TYPE |           NAME          |  ARCH | QUANTIZATION | LITTLE ENDIAN |    SIZE   | PARAMETERS |    BPW   |
+-------+-------------------------+-------+--------------+---------------+-----------+------------+----------+
| model | Llama 3.1 70B Instru... | llama |    Q4_K_M    |      true     | 39.59 GiB |   70.55 B  | 4.82 bpw |
+-------+-------------------------+-------+--------------+---------------+-----------+------------+----------+

+-----------------------------------------------------------------------------------------------------------------------------------+
| ARCHITECTURE                                                                                                                      |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
| MAX CONTEXT LEN | EMBEDDING LEN | ATTENTION CAUSAL | ATTENTION HEAD CNT | LAYERS | FEED FORWARD LEN | EXPERT CNT | VOCABULARY LEN |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+
|      131072     |      8192     |       true       |         64         |   80   |       28672      |      0     |     128256     |
+-----------------+---------------+------------------+--------------------+--------+------------------+------------+----------------+

+-------------------------------------------------------------------------------------------------------------------------------------------------------+
| TOKENIZER                                                                                                                                             |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
| MODEL | TOKENS SIZE | TOKENS LEN | ADDED TOKENS LEN | BOS TOKEN | EOS TOKEN | EOT TOKEN | EOM TOKEN | UNKNOWN TOKEN | SEPARATOR TOKEN | PADDING TOKEN |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+
|  gpt2 |    2 MiB    |   128256   |        N/A       |   128000  |   128009  |    N/A    |    N/A    |      N/A      |       N/A       |      N/A      |
+-------+-------------+------------+------------------+-----------+-----------+-----------+-----------+---------------+-----------------+---------------+

+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| ESTIMATE                                                                                                                                                                                                                                          |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+----------------------------------------------+-----------------------------------------+
|  ARCH | CONTEXT SIZE | BATCH SIZE (L / P) | FLASH ATTENTION | MMAP LOAD | EMBEDDING ONLY |  RERANKING  | DISTRIBUTABLE | OFFLOAD LAYERS | FULL OFFLOADED |                      RAM                     |                  VRAM 0                 |
|       |              |                    |                 |           |                |             |               |                |                +--------------------+------------+------------+----------------+------------+-----------+
|       |              |                    |                 |           |                |             |               |                |                | LAYERS (I + T + O) |     UMA    |   NONUMA   | LAYERS (T + O) |     UMA    |   NONUMA  |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+------------+------------+----------------+------------+-----------+
| llama |     2048     |     2048 / 512     |     Disabled    |  Enabled  |       No       | Unsupported |   Supported   |   81 (80 + 1)  |       Yes      |      1 + 0 + 0     | 255.27 MiB | 405.27 MiB |     80 + 1     | 906.50 MiB | 40.49 GiB |
+-------+--------------+--------------------+-----------------+-----------+----------------+-------------+---------------+----------------+----------------+--------------------+------------+------------+----------------+------------+-----------+
```

#### Others

##### Parse Image Model

```shell
$ # Parse FLUX.1-dev Model
$ gguf-parser --hf-repo="gpustack/FLUX.1-dev-GGUF" --hf-file="FLUX.1-dev-FP16.gguf"
+----------------------------------------------------------------------------------------------+
| METADATA                                                                                     |
+-------+------+-----------+--------------+---------------+-----------+------------+-----------+
|  TYPE | NAME |    ARCH   | QUANTIZATION | LITTLE ENDIAN |    SIZE   | PARAMETERS |    BPW    |
+-------+------+-----------+--------------+---------------+-----------+------------+-----------+
| model |  N/A | diffusion |      F16     |      true     | 31.79 GiB |    17 B    | 16.06 bpw |
+-------+------+-----------+--------------+---------------+-----------+------------+-----------+

+----------------------------------------------------------------------------------------------------------+
| ARCHITECTURE                                                                                             |
+----------------+---------------------------------------------------------------+-------------------------+
| DIFFUSION ARCH |                          CONDITIONERS                         |       AUTOENCODER       |
+----------------+---------------------------------------------------------------+-------------------------+
|     FLUX.1     | OpenAI CLIP ViT-L/14 (MOSTLY_F16), Google T5-xxl (MOSTLY_F16) | FLUX.1 VAE (MOSTLY_F16) |
+----------------+---------------------------------------------------------------+-------------------------+

+---------------------------------------------------------------------------------------------------------------------------+
| ESTIMATE                                                                                                                  |
+--------+-----------------+-------------+---------------+----------------+-------------------------+-----------------------+
|  ARCH  | FLASH ATTENTION |  MMAP LOAD  | DISTRIBUTABLE | FULL OFFLOADED |           RAM           |         VRAM 0        |
|        |                 |             |               |                +------------+------------+-----------+-----------+
|        |                 |             |               |                |     UMA    |   NONUMA   |    UMA    |   NONUMA  |
+--------+-----------------+-------------+---------------+----------------+------------+------------+-----------+-----------+
| flux_1 |     Disabled    | Unsupported |   Supported   |       Yes      | 343.89 MiB | 493.89 MiB | 31.89 GiB | 41.15 GiB |
+--------+-----------------+-------------+---------------+----------------+------------+------------+-----------+-----------+

$ # Parse FLUX.1-dev Model without offload Conditioner and Autoencoder
$ gguf-parser --hf-repo="gpustack/FLUX.1-dev-GGUF" --hf-file="FLUX.1-dev-FP16.gguf" --clip-on-cpu --vae-on-cpu
+----------------------------------------------------------------------------------------------+
| METADATA                                                                                     |
+-------+------+-----------+--------------+---------------+-----------+------------+-----------+
|  TYPE | NAME |    ARCH   | QUANTIZATION | LITTLE ENDIAN |    SIZE   | PARAMETERS |    BPW    |
+-------+------+-----------+--------------+---------------+-----------+------------+-----------+
| model |  N/A | diffusion |      F16     |      true     | 31.79 GiB |    17 B    | 16.06 bpw |
+-------+------+-----------+--------------+---------------+-----------+------------+-----------+

+----------------------------------------------------------------------------------------------------------+
| ARCHITECTURE                                                                                             |
+----------------+---------------------------------------------------------------+-------------------------+
| DIFFUSION ARCH |                          CONDITIONERS                         |       AUTOENCODER       |
+----------------+---------------------------------------------------------------+-------------------------+
|     FLUX.1     | OpenAI CLIP ViT-L/14 (MOSTLY_F16), Google T5-xxl (MOSTLY_F16) | FLUX.1 VAE (MOSTLY_F16) |
+----------------+---------------------------------------------------------------+-------------------------+

+-------------------------------------------------------------------------------------------------------------------------+
| ESTIMATE                                                                                                                |
+--------+-----------------+-------------+---------------+----------------+-----------------------+-----------------------+
|  ARCH  | FLASH ATTENTION |  MMAP LOAD  | DISTRIBUTABLE | FULL OFFLOADED |          RAM          |         VRAM 0        |
|        |                 |             |               |                +-----------+-----------+-----------+-----------+
|        |                 |             |               |                |    UMA    |   NONUMA  |    UMA    |   NONUMA  |
+--------+-----------------+-------------+---------------+----------------+-----------+-----------+-----------+-----------+
| flux_1 |     Disabled    | Unsupported |   Supported   |       Yes      | 16.44 GiB | 16.59 GiB | 22.29 GiB | 25.05 GiB |
+--------+-----------------+-------------+---------------+----------------+-----------+-----------+-----------+-----------+

$ # Parse FLUX.1-dev Model with Autoencoder tiling
$ gguf-parser --hf-repo="gpustack/FLUX.1-dev-GGUF" --hf-file="FLUX.1-dev-FP16.gguf" --vae-tiling
+----------------------------------------------------------------------------------------------+
| METADATA                                                                                     |
+-------+------+-----------+--------------+---------------+-----------+------------+-----------+
|  TYPE | NAME |    ARCH   | QUANTIZATION | LITTLE ENDIAN |    SIZE   | PARAMETERS |    BPW    |
+-------+------+-----------+--------------+---------------+-----------+------------+-----------+
| model |  N/A | diffusion |      F16     |      true     | 31.79 GiB |    17 B    | 16.06 bpw |
+-------+------+-----------+--------------+---------------+-----------+------------+-----------+

+----------------------------------------------------------------------------------------------------------+
| ARCHITECTURE                                                                                             |
+----------------+---------------------------------------------------------------+-------------------------+
| DIFFUSION ARCH |                          CONDITIONERS                         |       AUTOENCODER       |
+----------------+---------------------------------------------------------------+-------------------------+
|     FLUX.1     | OpenAI CLIP ViT-L/14 (MOSTLY_F16), Google T5-xxl (MOSTLY_F16) | FLUX.1 VAE (MOSTLY_F16) |
+----------------+---------------------------------------------------------------+-------------------------+

+---------------------------------------------------------------------------------------------------------------------------+
| ESTIMATE                                                                                                                  |
+--------+-----------------+-------------+---------------+----------------+-------------------------+-----------------------+
|  ARCH  | FLASH ATTENTION |  MMAP LOAD  | DISTRIBUTABLE | FULL OFFLOADED |           RAM           |         VRAM 0        |
|        |                 |             |               |                +------------+------------+-----------+-----------+
|        |                 |             |               |                |     UMA    |   NONUMA   |    UMA    |   NONUMA  |
+--------+-----------------+-------------+---------------+----------------+------------+------------+-----------+-----------+
| flux_1 |     Disabled    | Unsupported |   Supported   |       Yes      | 343.89 MiB | 493.89 MiB | 31.89 GiB | 36.28 GiB |
+--------+-----------------+-------------+---------------+----------------+------------+------------+-----------+-----------+

$ # Parse FLUX.1-dev Model with multiple devices offloading
$ # Support by LLaMA Box v0.0.106+, https://github.com/gpustack/llama-box.
$ gguf-parser --hf-repo="gpustack/FLUX.1-dev-GGUF" --hf-file="FLUX.1-dev-FP16.gguf" --tensor-split="1,1,1"
+----------------------------------------------------------------------------------------------+
| METADATA                                                                                     |
+-------+------+-----------+--------------+---------------+-----------+------------+-----------+
|  TYPE | NAME |    ARCH   | QUANTIZATION | LITTLE ENDIAN |    SIZE   | PARAMETERS |    BPW    |
+-------+------+-----------+--------------+---------------+-----------+------------+-----------+
| model |  N/A | diffusion |      F16     |      true     | 31.79 GiB |    17 B    | 16.06 bpw |
+-------+------+-----------+--------------+---------------+-----------+------------+-----------+

+----------------------------------------------------------------------------------------------------------+
| ARCHITECTURE                                                                                             |
+----------------+---------------------------------------------------------------+-------------------------+
| DIFFUSION ARCH |                          CONDITIONERS                         |       AUTOENCODER       |
+----------------+---------------------------------------------------------------+-------------------------+
|     FLUX.1     | OpenAI CLIP ViT-L/14 (MOSTLY_F16), Google T5-xxl (MOSTLY_F16) | FLUX.1 VAE (MOSTLY_F16) |
+----------------+---------------------------------------------------------------+-------------------------+

+-----------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| ESTIMATE                                                                                                                                                              |
+--------+-----------------+-------------+---------------+----------------+-------------------------+---------------------+---------------------+-----------------------+
|  ARCH  | FLASH ATTENTION |  MMAP LOAD  | DISTRIBUTABLE | FULL OFFLOADED |           RAM           |        VRAM 0       |        VRAM 1       |         VRAM 2        |
|        |                 |             |               |                +------------+------------+----------+----------+------------+--------+-----------+-----------+
|        |                 |             |               |                |     UMA    |   NONUMA   |    UMA   |  NONUMA  |     UMA    | NONUMA |    UMA    |   NONUMA  |
+--------+-----------------+-------------+---------------+----------------+------------+------------+----------+----------+------------+--------+-----------+-----------+
| flux_1 |     Disabled    | Unsupported |   Supported   |       Yes      | 343.89 MiB | 493.89 MiB | 9.34 GiB | 9.60 GiB | 259.96 MiB |  7 GiB | 22.29 GiB | 25.05 GiB |
+--------+-----------------+-------------+---------------+----------------+------------+------------+----------+----------+------------+--------+-----------+-----------+
```

##### Parse None Model

```shell
$ # Parse Multi-Modal Projector
$ gguf-parser --hf-repo="unsloth/Qwen2.5-Omni-3B-GGUF" --hf-file="mmproj-F32.gguf"                                                                        
+-------------------------------------------------------------------------------------------------------+
| METADATA                                                                                              |
+-----------+-----------------+------+--------------+---------------+----------+------------+-----------+
|    TYPE   |       NAME      | ARCH | QUANTIZATION | LITTLE ENDIAN |   SIZE   | PARAMETERS |    BPW    |
+-----------+-----------------+------+--------------+---------------+----------+------------+-----------+
| projector | Qwen2.5-Omni-3B | clip |      F32     |      true     | 4.86 GiB |   1.31 B   | 31.93 bpw |
+-----------+-----------------+------+--------------+---------------+----------+------------+-----------+

+-------------------------------------------------------------------------------------------------------------------------+
| ARCHITECTURE                                                                                                            |
+----------------+-------------------------------+-----------------+-------------------------------------+----------------+
| PROJECTOR TYPE |         EMBEDDING LEN         |      LAYERS     |           FEED FORWARD LEN          |     ENCODER    |
|                +---------------+---------------+--------+--------+------------------+------------------+                |
|                |     VISION    |     AUDIO     | VISION |  AUDIO |      VISION      |       AUDIO      |                |
+----------------+---------------+---------------+--------+--------+------------------+------------------+----------------+
|    qwen2.5o    |      1280     |      1280     |   32   |   32   |       1280       |       5120       | Vision & Audio |
+----------------+---------------+---------------+--------+--------+------------------+------------------+----------------+

$ # Parse LoRA Adapter
$ gguf-parser --hf-repo="ngxson/test_gguf_lora_adapter" --hf-file="lora-Llama-3-Instruct-abliteration-LoRA-8B-f16.gguf"
+---------------------------------------------------------------------------------------------+
| METADATA                                                                                    |
+---------+------+-------+--------------+---------------+------------+------------+-----------+
|   TYPE  | NAME |  ARCH | QUANTIZATION | LITTLE ENDIAN |    SIZE    | PARAMETERS |    BPW    |
+---------+------+-------+--------------+---------------+------------+------------+-----------+
| adapter |  N/A | llama |      F16     |      true     | 168.08 MiB |   88.12 M  | 16.00 bpw |
+---------+------+-------+--------------+---------------+------------+------------+-----------+

+---------------------------+
| ARCHITECTURE              |
+--------------+------------+
| ADAPTER TYPE | LORA ALPHA |
+--------------+------------+
|     lora     |     32     |
+--------------+------------+
```

### Estimate

#### Across Multiple GPU Devices

Imaging you're preparing to run
the [hierholzer/Llama-3.1-70B-Instruct-GGUF](https://huggingface.co/hierholzer/Llama-3.1-70B-Instruct-GGUF) model file
across several hosts in your local network. Some of these hosts are equipped with GPU devices, while others do not have
any GPU capabilities.

```mermaid
flowchart TD
    subgraph host4["Windows 11 (host4)"]
        ram40(["11GiB RAM remaining"])
    end
    subgraph host3["Apple macOS (host3)"]
        gpu10["Apple M1 Max (6GiB VRAM remaining)"]
    end
    subgraph host2["Windows 11 (host2)"]
        gpu20["NVIDIA 4090 (12GiB VRAM remaining)"]
    end
    subgraph host1["Ubuntu (host1)"]
        gpu30["NVIDIA 4080 0 (8GiB VRAM remaining)"]
        gpu31["NVIDIA 4080 1 (10GiB VRAM remaining)"]
    end
```

##### Single Host Multiple GPU Devices

Let's assume you plan to run the model on `host1` only.

```mermaid
flowchart TD
    subgraph host1["Ubuntu (host1)"]
        gpu30["NVIDIA 4080 0 (8GiB VRAM remaining)"]
        gpu31["NVIDIA 4080 1 (10GiB VRAM remaining)"]
    end
```

```shell
$ gguf-parser --hf-repo="hierholzer/Llama-3.1-70B-Instruct-GGUF" --hf-file="Llama-3.1-70B-Instruct-Q4_K_M.gguf" --ctx-size=1024 --tensor-split="8,10" --estimate --in-short
+------------------------------------------------------------------------------------------------------------------------------+
| ESTIMATE                                                                                                                     |
+----------------------------------------------+--------------------------------------+----------------------------------------+
|                      RAM                     |                VRAM 0                |                 VRAM 1                 |
+--------------------+------------+------------+----------------+---------+-----------+----------------+-----------+-----------+
| LAYERS (I + T + O) |     UMA    |   NONUMA   | LAYERS (T + O) |   UMA   |   NONUMA  | LAYERS (T + O) |    UMA    |   NONUMA  |
+--------------------+------------+------------+----------------+---------+-----------+----------------+-----------+-----------+
|      1 + 0 + 0     | 249.27 MiB | 399.27 MiB |     36 + 0     | 144 MiB | 17.83 GiB |     44 + 1     | 22.27 GiB | 22.83 GiB |
+--------------------+------------+------------+----------------+---------+-----------+----------------+-----------+-----------+
```

Based on the output provided, serving the `hierholzer/Llama-3.1-70B-Instruct-GGUF` model on `host1` has the following
resource consumption:

| Host                  | Available RAM | Request RAM | Available VRAM | Request VRAM | Result     |
|-----------------------|---------------|-------------|----------------|--------------|------------|
| host1                 | ENOUGH        | 399.27 MiB  |                |              | :thumbsup: |
| host1 (NVIDIA 4080 0) |               |             | 8 GiB          | 17.83 GiB    |            |
| host1 (NVIDIA 4080 1) |               |             | 10 GiB         | 22.83 GiB    |            |

It appears that running the model on `host1` alone is not feasible.

##### Multiple Hosts Multiple GPU Devices

Next, let's consider the scenario where you plan to run the model on `host4`, while offloading all layers to `host1`,
`host2`,
and `host3`.

```mermaid
flowchart TD
    host4 -->|TCP| gpu10
    host4 -->|TCP| gpu20
    host4 -->|TCP| gpu30
    host4 -->|TCP| gpu31

    subgraph host4["Windows 11 (host4)"]
        ram40(["11GiB RAM remaining"])
    end
    subgraph host3["Apple macOS (host3)"]
        gpu10["Apple M1 Max (6GiB VRAM remaining)"]
    end
    subgraph host2["Windows 11 (host2)"]
        gpu20["NVIDIA 4090 (12GiB VRAM remaining)"]
    end
    subgraph host1["Ubuntu (host1)"]
        gpu30["NVIDIA 4080 0 (8GiB VRAM remaining)"]
        gpu31["NVIDIA 4080 1 (10GiB VRAM remaining)"]
    end
```

```shell
$ gguf-parser --hf-repo="hierholzer/Llama-3.1-70B-Instruct-GGUF" --hf-file="Llama-3.1-70B-Instruct-Q4_K_M.gguf" --ctx-size=1024 --tensor-split="8,10,12,6" --rpc="host1:50052,host1:50053,host2:50052,host3:50052" --estimate --in-short
+------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| ESTIMATE                                                                                                                                                                                                                                 |
+----------------------------------------------+----------------------------------------------+----------------------------------------------+----------------------------------------------+----------------------------------------------+
|                      RAM                     |                 RPC 0 (V)RAM                 |                 RPC 1 (V)RAM                 |                 RPC 2 (V)RAM                 |                 RPC 3 (V)RAM                 |
+--------------------+------------+------------+----------------+--------------+--------------+----------------+--------------+--------------+----------------+--------------+--------------+----------------+--------------+--------------+
| LAYERS (I + T + O) |     UMA    |   NONUMA   | LAYERS (T + O) |      UMA     |    NONUMA    | LAYERS (T + O) |      UMA     |    NONUMA    | LAYERS (T + O) |      UMA     |    NONUMA    | LAYERS (T + O) |      UMA     |    NONUMA    |
+--------------------+------------+------------+----------------+--------------+--------------+----------------+--------------+--------------+----------------+--------------+--------------+----------------+--------------+--------------+
|      1 + 0 + 0     | 249.27 MiB | 399.27 MiB |     18 + 0     |   8.85 GiB   |   9.28 GiB   |     23 + 0     |   10.88 GiB  |   11.32 GiB  |     27 + 0     |   12.75 GiB  |   13.19 GiB  |     12 + 1     |   7.13 GiB   |   7.64 GiB   |
+--------------------+------------+------------+----------------+--------------+--------------+----------------+--------------+--------------+----------------+--------------+--------------+----------------+--------------+--------------+
```

According to the output provided, serving the `hierholzer/Llama-3.1-70B-Instruct-GGUF` model on `host4` results in the
following resource consumption:

| Host                  | Available RAM | Request RAM | Available VRAM | Request VRAM | Result     |
|-----------------------|---------------|-------------|----------------|--------------|------------|
| host4                 | 11 GiB        | 399.27 MiB  |                |              | :thumbsup: |
| host1 (NVIDIA 4080 0) |               |             | 8 GiB          | 9.28 GiB     |            |
| host1 (NVIDIA 4080 1) |               |             | 10 GiB         | 11.32 GiB    |            |
| host2 (NVIDIA 4090)   |               |             | 12 GiB         | 13.19 GiB    |            |
| host3 (Apple M1 Max)  | ENOUGH        |             | 6 GiB          | 7.13 GiB     |            |

It seems that the model cannot be served on `host4`, even with all layers offloaded to `host1`, `host2`, and `host3`.

We should consider a different approach: running the model on `host3` while offloading all layers to `host1`, `host2`,
and `host4`.

```mermaid
flowchart TD
    host3 -->|TCP| ram40
    host3 -->|TCP| gpu20
    host3 -->|TCP| gpu30
    host3 -->|TCP| gpu31

    subgraph host4["Windows 11 (host4)"]
        ram40(["11GiB RAM remaining"])
    end
    subgraph host3["Apple macOS (host3)"]
        gpu10["Apple M1 Max (6GiB VRAM remaining)"]
    end
    subgraph host2["Windows 11 (host2)"]
        gpu20["NVIDIA 4090 (12GiB VRAM remaining)"]
    end
    subgraph host1["Ubuntu (host1)"]
        gpu30["NVIDIA 4080 0 (8GiB VRAM remaining)"]
        gpu31["NVIDIA 4080 1 (10GiB VRAM remaining)"]
    end
```

```shell
$ gguf-parser --hf-repo="hierholzer/Llama-3.1-70B-Instruct-GGUF" --hf-file="Llama-3.1-70B-Instruct-Q4_K_M.gguf" --ctx-size=1024 --tensor-split="11,12,8,10,6" --rpc="host4:50052,host2:50052,host1:50052,host1:50053" --estimate --in-short
+-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| ESTIMATE                                                                                                                                                                                                                                                                          |
+----------------------------------------------+----------------------------------------------+----------------------------------------------+----------------------------------------------+----------------------------------------------+----------------------------------------+
|                      RAM                     |                 RPC 0 (V)RAM                 |                 RPC 1 (V)RAM                 |                 RPC 2 (V)RAM                 |                 RPC 3 (V)RAM                 |                 VRAM 0                 |
+--------------------+------------+------------+----------------+--------------+--------------+----------------+--------------+--------------+----------------+--------------+--------------+----------------+--------------+--------------+----------------+------------+----------+
| LAYERS (I + T + O) |     UMA    |   NONUMA   | LAYERS (T + O) |      UMA     |    NONUMA    | LAYERS (T + O) |      UMA     |    NONUMA    | LAYERS (T + O) |      UMA     |    NONUMA    | LAYERS (T + O) |      UMA     |    NONUMA    | LAYERS (T + O) |     UMA    |  NONUMA  |
+--------------------+------------+------------+----------------+--------------+--------------+----------------+--------------+--------------+----------------+--------------+--------------+----------------+--------------+--------------+----------------+------------+----------+
|      1 + 0 + 0     | 249.27 MiB | 399.27 MiB |     19 + 0     |   9.36 GiB   |   9.79 GiB   |     21 + 0     |   9.92 GiB   |   10.35 GiB  |     14 + 0     |   6.57 GiB   |   7.01 GiB   |     17 + 0     |   8.11 GiB   |   8.54 GiB   |      9 + 1     | 302.50 MiB | 6.16 GiB |
+--------------------+------------+------------+----------------+--------------+--------------+----------------+--------------+--------------+----------------+--------------+--------------+----------------+--------------+--------------+----------------+------------+----------+
```

According to the output provided, serving the `hierholzer/Llama-3.1-70B-Instruct-GGUF` model on `host3` results in the
following resource consumption:

| Host                  | Available RAM | Request RAM | Available VRAM | Request VRAM | Result     |
|-----------------------|---------------|-------------|----------------|--------------|------------|
| host3 (Apple M1 Max)  | ENOUGH        | 249.27 MiB  |                |              | :thumbsup: |
| host4                 | 11 GiB        | 9.79 GiB    |                |              | :thumbsup: |
| host2 (NVIDIA 4090)   |               |             | 12 GiB         | 10.35 GiB    | :thumbsup: |
| host1 (NVIDIA 4080 0) |               |             | 8 GiB          | 7.01 GiB     | :thumbsup: |
| host1 (NVIDIA 4080 1) |               |             | 10 GiB         | 8.54 GiB     | :thumbsup: |
| host3 (Apple M1 Max)  |               |             | 6 GiB          | 302.50 MiB   | :thumbsup: |

Now, the model can be successfully served on `host3`, with all layers offloaded to `host1`, `host2`, and `host4`.

#### Maximum Tokens Per Second

The maximum TPS estimation for the GGUF Parser is determined by the model's parameter size, context size, model
offloaded layers, and devices on which the model runs. Among these factors, the device's specifications are particularly
important.

Inspired
by [LLM inference speed of light](https://zeux.io/2024/03/15/llm-inference-sol/), GGUF Parser use the **FLOPS** and
**bandwidth** of the device as evaluation metrics:

- When the device is a CPU, FLOPS refers to the performance of that CPU, while bandwidth corresponds to the DRAM
  bandwidth.
- When the device is a (i)GPU, FLOPS indicates the performance of that (i)GPU, and bandwidth corresponds to the VRAM
  bandwidth.
- When the device is a specific host, FLOPS depends on whether the CPU or (i)GPU of that host is being used, while
  bandwidth corresponds to the bandwidth connecting the main node to that host. **After all, a chain is only as strong
  as
  its weakest link.** If the connection bandwidth between the
  main node and the host is equal to or greater than the *RAM bandwidth, then the bandwidth should be taken as the *RAM
  bandwidth value.

##### CPU FLOPS Calculation

The performance of a single CPU cache can be calculated using the following formula:

$$ CPU\ FLOPS = Number\ of \ Cores \times Core\ Frequency \times Floating\ Point\ Operations\ per\ Cycle $$

The Apple M1 Max CPU features a total of 10 cores, consisting of 8 performance cores and 2 efficiency cores. The
performance cores operate at a clock speed of 3.2 GHz, while the efficiency cores run at 2.2 GHz. All cores support
the [ARM NEON instruction set](https://en.wikipedia.org/wiki/ARM_architecture_family#Advanced_SIMD_(Neon)), which
enables 128-bit SIMD operations, allowing multiple floating-point numbers to be processed simultaneously within a
single CPU cycle. Specifically, using single-precision (32-bit) floating-point numbers, each cycle can handle 4
floating-point operations.

The peak floating-point performance for a single performance core is calculated as follows:

$$ Peak\ Performance = 3.2\ GHz \times 4\ FLOPS = 12.8\ GFLOPS $$

For a single efficiency core, the calculation is:

$$ Peak\ Performance = 2.2\ GHz \times 4\ FLOPS = 8.8\ GFLOPS $$

Thus, the overall peak floating-point performance of the entire CPU can be determined by combining the contributions
from both types of cores:

$$ Peak\ Performance = 8\ Cores \times 12.8\ GFLOPS + 2\ Cores \times 8.8\ GFLOPS = 120\ GFLOPS $$

> This results in an average performance of 12 GFLOPS per core. It is evident that the average performance achieved by
> utilizing both performance and efficiency cores is lower than that obtained by exclusively using performance cores.

##### Run LLaMA2-7B-Chat with Apple Silicon M-series

Taking [TheBloke/Llama-2-7B-Chat-GGUF](https://huggingface.co/TheBloke/Llama-2-7B-Chat-GGUF) as an
example and estimate the maximum tokens per second for Apple Silicon M-series using the GGUF Parser.

```shell
$ # Estimate full offloaded Q8_0 model
$ gguf-parser --hf-repo TheBloke/LLaMA-7b-GGUF --hf-file llama-7b.Q8_0.gguf --estimate --in-short \
  -c 512 \
  --device-metric "<CPU FLOPS>;<RAM BW>,<iGPU FLOPS>;<VRAM BW>"

$ # Estimate full offloaded Q4_0 model
$ gguf-parser --hf-repo TheBloke/LLaMA-7b-GGUF --hf-file llama-7b.Q4_0.gguf --estimate --in-short \
  -c 512 \
  --device-metric "<CPU FLOPS>;<RAM BW>,<iGPU FLOPS>;<VRAM BW>"
```

| Variant  | CPU FLOPS (Performance Core) | iGPU FLOPS             | (V)RAM Bandwidth | Q8_0 Max TPS | Q4_0 Max TPS |
|----------|------------------------------|------------------------|------------------|--------------|--------------|
| M1       | 51.2 GFLOPS  (4 cores)       | 2.6 TFLOPS (8 cores)   | 68.3 GBps        | 8.68         | 14.56        |
| M1 Pro   | 102.4 GFLOPS  (8 cores)      | 5.2 TFLOPS (16 cores)  | 204.8 GBps       | 26.04        | 43.66        |
| M1 Max   | 102.4 GFLOPS  (8 cores)      | 10.4 TFLOPS (32 cores) | 409.6 GBps       | 52.08        | 87.31        |
| M1 Ultra | 204.8 GFLOPS (16 cores)      | 21 TFLOPS (64 cores)   | 819.2 GBps       | 104.16       | 174.62       |
| M2       | 56 GFLOPS (4 cores)          | 3.6 TFLOPS (10 cores)  | 102.4 GBps       | 13.02        | 21.83        |
| M2 Pro   | 112 GFLOPS (8 cores)         | 6.8 TFLOPS (19 cores)  | 204.8 GBps       | 26.04        | 43.66        |
| M2 Max   | 112 GFLOPS (8 cores)         | 13.6 TFLOPS (38 cores) | 409.6 GBps       | 52.08        | 87.31        |
| M2 Ultra | 224 GFLOPS (16 cores)        | 27.2 TFLOPS (76 cores) | 819.2 GBps       | 104.16       | 174.62       |
| M3       | 64.96 GFLOPS (4 cores)       | 4.1 TFLOPS (10 cores)  | 102.4 GBps       | 13.02        | 21.83        |
| M3 Pro   | 97.44 GFLOPS (6 cores)       | 7.4 TFLOPS (18 cores)  | 153.6 GBps       | 19.53        | 32.74        |
| M3 Max   | 194.88 GFLOPS (12 cores)     | 16.4 TFLOPS (40 cores) | 409.6 GBps       | 52.08        | 87.31        |
| M4       | 70.56 GFLOPS (4 cores)       | 4.1 TFLOPS             | 120 GBps         | 15.26        | 25.58        |

> References:
> - https://www.cpu-monkey.com/en/cpu_family-apple_m_series
> - https://nanoreview.net/
> - https://en.wikipedia.org/wiki/Apple_M1#Variants
> - https://en.wikipedia.org/wiki/Apple_M2#Variants
> - https://en.wikipedia.org/wiki/Apple_M3#Variants
> - https://en.wikipedia.org/wiki/Apple_M4#Variants

You can further verify the above results in [Performance of llama.cpp on Apple Silicon M-series
](https://github.com/ggerganov/llama.cpp/discussions/4167#user-content-fn-1-e9a4caf2848534167e450e18fc4ede7f).

##### Run LLaMA3.1-405B-Instruct with Apple Mac Studio devices combined with Thunderbolt

Example
by [leafspark/Meta-Llama-3.1-405B-Instruct-GGUF](https://huggingface.co/leafspark/Meta-Llama-3.1-405B-Instruct-GGUF)
and estimate the maximum tokens per second for three Apple Mac Studio devices combined with Thunderbolt.

| Device                        | CPU FLOPS (Performance Core) | iGPU FLOPS             | (V)RAM Bandwidth | Thunderbolt Bandwidth | Role       |
|-------------------------------|------------------------------|------------------------|------------------|-----------------------|------------|
| Apple Mac Studio (M2 Ultra) 0 | 224 GFLOPS (16 cores)        | 27.2 TFLOPS (76 cores) | 819.2 GBps       | 40 Gbps               | Main       |
| Apple Mac Studio (M2 Ultra) 1 | 224 GFLOPS (16 cores)        | 27.2 TFLOPS (76 cores) | 819.2 GBps       | 40 Gbps               | RPC Server |
| Apple Mac Studio (M2 Ultra) 2 | 224 GFLOPS (16 cores)        | 27.2 TFLOPS (76 cores) | 819.2 GBps       | 40 Gbps               | RPC Server |

Get the maximum tokens per second with the following command:

```shell
$ # Explain the command:
$ # --device-metric "224GFLOPS;819.2GBps"         <-- Apple Mac Studio 0 CPU FLOPS and RAM Bandwidth
$ # --device-metric "27.2TFLOPS;819.2GBps;40Gbps" <-- Apple Mac Studio 1 (RPC 0) iGPU FLOPS, VRAM Bandwidth, and Thunderbolt Bandwidth
$ # --device-metric "27.2TFLOPS;819.2GBps;40Gbps" <-- Apple Mac Studio 2 (RPC 1) iGPU FLOPS, VRAM Bandwidth, and Thunderbolt Bandwidth
$ # --device-metric "27.2TFLOPS;819.2GBps"        <-- Apple Mac Studio 0 iGPU FLOPS and VRAM Bandwidth
$ gguf-parser --hf-repo leafspark/Meta-Llama-3.1-405B-Instruct-GGUF --hf-file Llama-3.1-405B-Instruct.Q4_0.gguf/Llama-3.1-405B-Instruct.Q4_0-00001-of-00012.gguf --estimate --in-short \
  --no-mmap \
  -c 512 \
  --rpc host1:port,host2:port \
  --tensor-split "<Proportions>" \
  --device-metric "224GFLOPS;819.2GBps" \
  --device-metric "27.2TFLOPS;819.2GBps;40Gbps" \
  --device-metric "27.2TFLOPS;819.2GBps;40Gbps" \
  --device-metric "27.2TFLOPS;819.2GBps"
```

| Tensor Split | Apple Mac Studio 0 RAM | Apple Mac Studio 1 VRAM (RPC 0) | Apple Mac Studio 2 VRAM  (RPC 1) | Apple Mac Studio 0 VRAM | Q4_0 Max TPS |
|--------------|------------------------|---------------------------------|----------------------------------|-------------------------|--------------|
| 1,1,1        | 1.99 GiB               | 72.74 GiB                       | 71.04 GiB                        | 70.96 GiB               | 10.71        |
| 2,1,1        | 1.99 GiB               | 108.26 GiB                      | 54.13 GiB                        | 52.35 GiB               | 11.96        |
| 3,1,1        | 1.99 GiB               | 130.25 GiB                      | 42.29 GiB                        | 42.20 GiB               | 9.10         |
| 4,1,1        | 1.99 GiB               | 143.78 GiB                      | 35.52 GiB                        | 35.44 GiB               | 7.60         |

##### Run Qwen2.5-72B-Instruct with NVIDIA RTX 4080 and remote RPC by Apple Mac Studio (M2)

Example by [Qwen/Qwen2.5-72B-Instruct-GGUF](https://huggingface.co/Qwen/Qwen2.5-72B-Instruct-GGUF) and estimate the
maximum tokens per second for NVIDIA RTX 4080.

| Hardware                                    | FLOPS        | Bandwidth  |
|---------------------------------------------|--------------|------------|
| Intel i5-14600k                             | 510.4 GFLOPS |            |
| 2 x Corsair Vengeance RGB DDR5-6000 (32GiB) |              | 96 GBps    |
| 2 x NVIDIA GeForce RTX 4080                 | 48.74 TFLOPS | 736.3 GBps |
| Apple Mac Studio (M2)                       | 27.2 TFLOPS  | 819.2 GBps |

```shell
$ # Explain the command:
$ # --tensor-split 20369,12935,13325               <-- Available Memory in MiB for each device
$ # --device-metric "510.4GFLOPS;96GBps"           <-- Intel i5-14600k CPU FLOPS and RAM Bandwidth
$ # --device-metric "27.2TFLOPS;819.2GBps;40Gbps"  <-- Apple Mac Studio (M2) (RPC 0) iGPU FLOPS, VRAM Bandwidth, and Thunderbolt Bandwidth
$ # --device-metric "48.74TFLOPS;736.3GBps;64GBps" <-- NVIDIA GeForce RTX 0 4080 GPU FLOPS, VRAM Bandwidth, and PCIe 5.0 x16 Bandwidth
$ # --device-metric "48.74TFLOPS;736.3GBps;8GBps"  <-- NVIDIA GeForce RTX 1 4080 GPU FLOPS, VRAM Bandwidth, and PCIe 4.0 x4 Bandwidth
$ gguf-parser --hf-repo Qwen/Qwen2.5-72B-Instruct-GGUF --hf-file qwen2.5-72b-instruct-q4_k_m-00001-of-00012.gguf --estimate --in-short \
  --no-mmap \
  -c 8192 \
  --rpc host:port \
  --tensor-split 20369,12935,13325 \
  --device-metric "510.4GFLOPS;96GBps" \
  --device-metric "27.2TFLOPS;819.2GBps;40Gbps" \
  --device-metric "48.74TFLOPS;736.3GBps;64GBps" \
  --device-metric "48.74TFLOPS;736.3GBps;8GBps"
+---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------+
| ESTIMATE                                                                                                                                                                              |
+-----------+------------------------------------------+----------------------------------------------+----------------------------------------+----------------------------------------+
|  MAX TPS  |                    RAM                   |                 RPC 0 (V)RAM                 |                 VRAM 0                 |                 VRAM 1                 |
|           +--------------------+----------+----------+----------------+--------------+--------------+----------------+-----------+-----------+----------------+-----------+-----------+
|           | LAYERS (I + T + O) |    UMA   |  NONUMA  | LAYERS (T + O) |      UMA     |    NONUMA    | LAYERS (T + O) |    UMA    |   NONUMA  | LAYERS (T + O) |    UMA    |   NONUMA  |
+-----------+--------------------+----------+----------+----------------+--------------+--------------+----------------+-----------+-----------+----------------+-----------+-----------+
| 51.82 tps |      1 + 0 + 0     | 1.19 GiB | 1.34 GiB |     36 + 0     |   18.85 GiB  |   20.17 GiB  |     22 + 0     | 11.34 GiB | 12.66 GiB |     22 + 1     | 12.65 GiB | 13.97 GiB |
+-----------+--------------------+----------+----------+----------------+--------------+--------------+----------------+-----------+-----------+----------------+-----------+-----------+
```

#### Full Layers Offload (default)

```shell
$ gguf-parser --hf-repo="etemiz/Llama-3.1-405B-Inst-GGUF" --hf-file="llama-3.1-405b-IQ1_M-00019-of-00019.gguf" --estimate --in-short
+-------------------------------------------------------------------------------------+
| ESTIMATE                                                                            |
+------------------------------------------+------------------------------------------+
|                    RAM                   |                  VRAM 0                  |
+--------------------+----------+----------+----------------+------------+------------+
| LAYERS (I + T + O) |    UMA   |  NONUMA  | LAYERS (T + O) |     UMA    |   NONUMA   |
+--------------------+----------+----------+----------------+------------+------------+
|      1 + 0 + 0     | 1.63 GiB | 1.78 GiB |     126 + 1    | 126.28 GiB | 246.86 GiB |
+--------------------+----------+----------+----------------+------------+------------+
```

#### Zero Layers Offload

```shell
$ gguf-parser --hf-repo="etemiz/Llama-3.1-405B-Inst-GGUF" --hf-file="llama-3.1-405b-IQ1_M-00019-of-00019.gguf" --gpu-layers=0 --estimate --in-short
+------------------------------------------------------------------------------------+
| ESTIMATE                                                                           |
+----------------------------------------------+-------------------------------------+
|                      RAM                     |                VRAM 0               |
+--------------------+------------+------------+----------------+--------+-----------+
| LAYERS (I + T + O) |     UMA    |   NONUMA   | LAYERS (T + O) |   UMA  |   NONUMA  |
+--------------------+------------+------------+----------------+--------+-----------+
|     1 + 126 + 1    | 127.64 GiB | 127.79 GiB |      0 + 0     |   0 B  | 33.62 GiB |
+--------------------+------------+------------+----------------+--------+-----------+
```

#### Specific Layers Offload

```shell
$ gguf-parser --hf-repo="etemiz/Llama-3.1-405B-Inst-GGUF" --hf-file="llama-3.1-405b-IQ1_M-00019-of-00019.gguf" --gpu-layers=10 --estimate --in-short
+----------------------------------------------------------------------------------+
| ESTIMATE                                                                         |
+----------------------------------------------+-----------------------------------+
|                      RAM                     |               VRAM 0              |
+--------------------+------------+------------+----------------+--------+---------+
| LAYERS (I + T + O) |     UMA    |   NONUMA   | LAYERS (T + O) |   UMA  |  NONUMA |
+--------------------+------------+------------+----------------+--------+---------+
|     1 + 126 + 1    | 127.64 GiB | 127.79 GiB |      0 + 0     |   0 B  | 250 MiB |
+--------------------+------------+------------+----------------+--------+---------+
```

#### Specific Context Size

By default, the context size retrieved from the model's metadata.

Use `--ctx-size` to specify the context size.

```shell
$ gguf-parser --hf-repo="etemiz/Llama-3.1-405B-Inst-GGUF" --hf-file="llama-3.1-405b-IQ1_M-00019-of-00019.gguf" --ctx-size=4096 --estimate --in-short
+--------------------------------------------------------------------------------------+
| ESTIMATE                                                                             |
+----------------------------------------------+---------------------------------------+
|                      RAM                     |                 VRAM 0                |
+--------------------+------------+------------+----------------+----------+-----------+
| LAYERS (I + T + O) |     UMA    |   NONUMA   | LAYERS (T + O) |    UMA   |   NONUMA  |
+--------------------+------------+------------+----------------+----------+-----------+
|      1 + 0 + 0     | 404.53 MiB | 554.53 MiB |     126 + 1    | 3.94 GiB | 93.28 GiB |
+--------------------+------------+------------+----------------+----------+-----------+
```

#### Enable Flash Attention

By default, LLaMA.cpp disables the Flash Attention.

Enable Flash Attention will reduce the VRAM usage, but it also increases the GPU/CPU usage.

Use `--flash-attention` to enable the Flash Attention.

Please note that not all models support Flash Attention, if the model does not support, the "FLASH ATTENTION" shows "
Disabled" even if you enable it.

```shell
$ gguf-parser --hf-repo="etemiz/Llama-3.1-405B-Inst-GGUF" --hf-file="llama-3.1-405b-IQ1_M-00019-of-00019.gguf" --flash-attention --estimate --in-short
+-------------------------------------------------------------------------------------+
| ESTIMATE                                                                            |
+------------------------------------------+------------------------------------------+
|                    RAM                   |                  VRAM 0                  |
+--------------------+----------+----------+----------------+------------+------------+
| LAYERS (I + T + O) |    UMA   |  NONUMA  | LAYERS (T + O) |     UMA    |   NONUMA   |
+--------------------+----------+----------+----------------+------------+------------+
|      1 + 0 + 0     | 1.63 GiB | 1.78 GiB |     126 + 1    | 126.28 GiB | 215.98 GiB |
+--------------------+----------+----------+----------------+------------+------------+
```

#### Disable MMap

By default, LLaMA.cpp loads the model via Memory-Mapped.

For Apple MacOS, Memory-Mapped is an efficient way to load the model, and results in a lower VRAM usage.
For other platforms, Memory-Mapped affects the first-time model loading speed only.

Use `--no-mmap` to disable loading the model via Memory-Mapped.

Please note that some models require loading the whole weight into memory, if the model does not support MMap, the "MMAP
LOAD" shows "Not Supported".

```shell
$ gguf-parser --hf-repo="etemiz/Llama-3.1-405B-Inst-GGUF" --hf-file="llama-3.1-405b-IQ1_M-00019-of-00019.gguf" --no-mmap --estimate --in-short
+-------------------------------------------------------------------------------------+
| ESTIMATE                                                                            |
+------------------------------------------+------------------------------------------+
|                    RAM                   |                  VRAM 0                  |
+--------------------+----------+----------+----------------+------------+------------+
| LAYERS (I + T + O) |    UMA   |  NONUMA  | LAYERS (T + O) |     UMA    |   NONUMA   |
+--------------------+----------+----------+----------------+------------+------------+
|      1 + 0 + 0     | 2.97 GiB | 3.12 GiB |     126 + 1    | 214.24 GiB | 246.86 GiB |
+--------------------+----------+----------+----------------+------------+------------+
```

#### With Adapter

Use `--lora`/`--control-vector` to estimate the usage when loading a model with adapters.

```shell
$ gguf-parser --hf-repo="QuantFactory/Meta-Llama-3-8B-Instruct-GGUF" --hf-file="Meta-Llama-3-8B-Instruct.Q5_K_M.gguf" --estimate --in-short
+-------------------------------------------------------------------------------------+
| ESTIMATE                                                                            |
+----------------------------------------------+--------------------------------------+
|                      RAM                     |                VRAM 0                |
+--------------------+------------+------------+----------------+----------+----------+
| LAYERS (I + T + O) |     UMA    |   NONUMA   | LAYERS (T + O) |    UMA   |  NONUMA  |
+--------------------+------------+------------+----------------+----------+----------+
|      1 + 0 + 0     | 210.80 MiB | 360.80 MiB |     32 + 1     | 1.25 GiB | 7.04 GiB |
+--------------------+------------+------------+----------------+----------+----------+

$ # With a LoRA adapter.
$ gguf-parser --hf-repo="QuantFactory/Meta-Llama-3-8B-Instruct-GGUF" --hf-file="Meta-Llama-3-8B-Instruct.Q5_K_M.gguf" --lora-url="https://huggingface.co/ngxson/test_gguf_lora_adapter/resolve/main/lora-Llama-3-Instruct-abliteration-LoRA-8B-f16.gguf" --estimate --in-short
+-------------------------------------------------------------------------------------+
| ESTIMATE                                                                            |
+----------------------------------------------+--------------------------------------+
|                      RAM                     |                VRAM 0                |
+--------------------+------------+------------+----------------+----------+----------+
| LAYERS (I + T + O) |     UMA    |   NONUMA   | LAYERS (T + O) |    UMA   |  NONUMA  |
+--------------------+------------+------------+----------------+----------+----------+
|      1 + 0 + 0     | 223.91 MiB | 373.91 MiB |     32 + 1     | 1.42 GiB | 7.20 GiB |
+--------------------+------------+------------+----------------+----------+----------+
```

#### Get Proper Offload Layers

Use `--gpu-layers-step` to get the proper offload layers number when the model is too large to fit into the GPUs memory.

```shell
$ gguf-parser --hf-repo="etemiz/Llama-3.1-405B-Inst-GGUF" --hf-file="llama-3.1-405b-IQ1_M-00019-of-00019.gguf" --gpu-layers-step=6 --estimate --in-short
+-----------------------------------------------------------------------------------------+
| ESTIMATE                                                                                |
+----------------------------------------------+------------------------------------------+
|                      RAM                     |                  VRAM 0                  |
+--------------------+------------+------------+----------------+------------+------------+
| LAYERS (I + T + O) |     UMA    |   NONUMA   | LAYERS (T + O) |     UMA    |   NONUMA   |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 126 + 1    | 127.64 GiB | 127.79 GiB |      0 + 0     |     0 B    |   250 MiB  |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 120 + 1    | 121.90 GiB | 122.05 GiB |      6 + 0     |    6 GiB   |  44.68 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 114 + 1    | 115.90 GiB | 116.05 GiB |     12 + 0     |   12 GiB   |  54.74 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 108 + 1    | 109.90 GiB | 110.05 GiB |     18 + 0     |   18 GiB   |  64.80 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 102 + 1    | 103.90 GiB | 104.05 GiB |     24 + 0     |   24 GiB   |  74.86 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 96 + 1     |  97.90 GiB |  98.05 GiB |     30 + 0     |   30 GiB   |  84.93 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 90 + 1     |  91.90 GiB |  92.05 GiB |     36 + 0     |   36 GiB   |  94.99 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 84 + 1     |  85.90 GiB |  86.05 GiB |     42 + 0     |   42 GiB   | 105.05 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 78 + 1     |  79.90 GiB |  80.05 GiB |     48 + 0     |   48 GiB   | 115.11 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 72 + 1     |  73.90 GiB |  74.05 GiB |     54 + 0     |   54 GiB   | 125.17 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 66 + 1     |  67.90 GiB |  68.05 GiB |     60 + 0     |   60 GiB   | 135.23 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 60 + 1     |  61.90 GiB |  62.05 GiB |     66 + 0     |   66 GiB   | 145.29 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 54 + 1     |  55.90 GiB |  56.05 GiB |     72 + 0     |   72 GiB   | 155.35 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 48 + 1     |  49.90 GiB |  50.05 GiB |     78 + 0     |   78 GiB   | 165.42 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 42 + 1     |  43.90 GiB |  44.05 GiB |     84 + 0     |   84 GiB   | 175.48 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 36 + 1     |  37.90 GiB |  38.05 GiB |     90 + 0     |   90 GiB   | 185.54 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 30 + 1     |  31.90 GiB |  32.05 GiB |     96 + 0     |   96 GiB   | 195.60 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 24 + 1     |  25.90 GiB |  26.05 GiB |     102 + 0    |   102 GiB  | 205.66 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 18 + 1     |  19.90 GiB |  20.05 GiB |     108 + 0    |   108 GiB  | 215.72 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|     1 + 12 + 1     |  13.90 GiB |  14.05 GiB |     114 + 0    |   114 GiB  | 226.05 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|      1 + 6 + 1     |  7.90 GiB  |  8.05 GiB  |     120 + 0    |   120 GiB  | 236.64 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|      1 + 0 + 1     |  1.90 GiB  |  2.05 GiB  |     126 + 0    |   126 GiB  | 246.24 GiB |
+--------------------+------------+------------+----------------+------------+------------+
|      1 + 0 + 0     |  1.63 GiB  |  1.78 GiB  |     126 + 1    | 126.28 GiB | 246.86 GiB |
+--------------------+------------+------------+----------------+------------+------------+
```

## License

MIT
