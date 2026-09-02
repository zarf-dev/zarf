package gguf_parser

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gpustack/gguf-parser-go/util/json"
	"github.com/gpustack/gguf-parser-go/util/osx"
	"github.com/gpustack/gguf-parser-go/util/stringx"
)

var (
	ErrGGUFFileCacheDisabled  = errors.New("GGUF file cache disabled")
	ErrGGUFFileCacheMissed    = errors.New("GGUF file cache missed")
	ErrGGUFFileCacheCorrupted = errors.New("GGUF file cache corrupted")
)

type GGUFFileCache string

func (c GGUFFileCache) getKeyPath(key string) string {
	k := stringx.SumByFNV64a(key)
	p := filepath.Join(string(c), k[:1], k)
	return p
}

func (c GGUFFileCache) Get(key string, exp time.Duration) (*GGUFFile, error) {
	if c == "" {
		return nil, ErrGGUFFileCacheDisabled
	}

	if key == "" {
		return nil, ErrGGUFFileCacheMissed
	}

	p := c.getKeyPath(key)
	if !osx.Exists(p, func(stat os.FileInfo) bool {
		if !stat.Mode().IsRegular() {
			return false
		}
		return exp == 0 || time.Since(stat.ModTime()) < exp
	}) {
		return nil, ErrGGUFFileCacheMissed
	}

	var gf GGUFFile
	{
		bs, err := os.ReadFile(p)
		if err != nil {
			return nil, fmt.Errorf("GGUF file cache get: %w", err)
		}
		if err = json.Unmarshal(bs, &gf); err != nil {
			return nil, fmt.Errorf("GGUF file cache get: %w", err)
		}
	}

	if len(gf.TensorInfos) == 0 {
		_ = os.Remove(p)
		return nil, ErrGGUFFileCacheCorrupted
	}

	return &gf, nil
}

func (c GGUFFileCache) Put(key string, gf *GGUFFile) error {
	if c == "" {
		return ErrGGUFFileCacheDisabled
	}

	if key == "" || gf == nil {
		return nil
	}

	bs, err := json.Marshal(gf)
	if err != nil {
		return fmt.Errorf("GGUF file cache put: %w", err)
	}

	p := c.getKeyPath(key)
	if err = osx.WriteFile(p, bs, 0o600); err != nil {
		return fmt.Errorf("GGUF file cache put: %w", err)
	}
	return nil
}

func (c GGUFFileCache) Delete(key string) error {
	if c == "" {
		return ErrGGUFFileCacheDisabled
	}

	if key == "" {
		return ErrGGUFFileCacheMissed
	}

	p := c.getKeyPath(key)
	if !osx.ExistsFile(p) {
		return ErrGGUFFileCacheMissed
	}

	if err := os.Remove(p); err != nil {
		return fmt.Errorf("GGUF file cache delete: %w", err)
	}
	return nil
}
