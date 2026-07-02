package gguf_parser

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/gpustack/gguf-parser-go/util/httpx"
	"github.com/gpustack/gguf-parser-go/util/json"
	"github.com/gpustack/gguf-parser-go/util/stringx"
)

// Inspired by https://github.com/ollama/ollama/blob/380e06e5bea06ae8ded37f47c37bd5d604194d3e/types/model/name.go,
// and https://github.com/ollama/ollama/blob/380e06e5bea06ae8ded37f47c37bd5d604194d3e/server/modelpath.go.

const (
	OllamaDefaultScheme    = "https"
	OllamaDefaultRegistry  = "registry.ollama.ai"
	OllamaDefaultNamespace = "library"
	OllamaDefaultTag       = "latest"
)

type (
	// OllamaModel represents an Ollama model,
	// its manifest(including MediaType, Config and Layers) can be completed further by calling the Complete method.
	OllamaModel struct {
		Schema        string             `json:"schema"`
		Registry      string             `json:"registry"`
		Namespace     string             `json:"namespace"`
		Repository    string             `json:"repository"`
		Tag           string             `json:"tag"`
		SchemaVersion uint32             `json:"schemaVersion"`
		MediaType     string             `json:"mediaType"`
		Config        OllamaModelLayer   `json:"config"`
		Layers        []OllamaModelLayer `json:"layers"`

		// Client is the http client used to complete the OllamaModel's network operations.
		//
		// When this field is nil,
		// it will be set to the client used by OllamaModel.Complete.
		//
		// When this field is offered,
		// the network operations will be done with this client.
		Client *http.Client `json:"-"`
	}

	// OllamaModelLayer represents an Ollama model layer,
	// its digest can be used to download the artifact.
	OllamaModelLayer struct {
		MediaType string `json:"mediaType"`
		Size      uint64 `json:"size"`
		Digest    string `json:"digest"`

		// Root points to the root OllamaModel,
		// which is never serialized or deserialized.
		//
		// When called OllamaModel.Complete,
		// this field will be set to the OllamaModel itself.
		// If not, this field will be nil,
		// and must be set manually to the root OllamaModel before calling the method of OllamaModelLayer.
		Root *OllamaModel `json:"-"`
	}
)

// ParseOllamaModel parses the given Ollama model string,
// and returns the OllamaModel, or nil if the model is invalid.
func ParseOllamaModel(model string, opts ...OllamaModelOption) *OllamaModel {
	if model == "" {
		return nil
	}

	var o _OllamaModelOptions
	for _, opt := range opts {
		opt(&o)
	}

	om := OllamaModel{
		Schema:    OllamaDefaultScheme,
		Registry:  OllamaDefaultRegistry,
		Namespace: OllamaDefaultNamespace,
		Tag:       OllamaDefaultTag,
	}
	{
		if o.DefaultScheme != "" {
			om.Schema = o.DefaultScheme
		}
		if o.DefaultRegistry != "" {
			om.Registry = o.DefaultRegistry
		}
		if o.DefaultNamespace != "" {
			om.Namespace = o.DefaultNamespace
		}
		if o.DefaultTag != "" {
			om.Tag = o.DefaultTag
		}
	}

	m := model

	// Drop digest.
	m, _, _ = stringx.CutFromRight(m, "@")

	// Get tag.
	m, s, ok := stringx.CutFromRight(m, ":")
	if ok && s != "" {
		om.Tag = s
	}

	// Get repository.
	m, s, ok = stringx.CutFromRight(m, "/")
	if ok && s != "" {
		om.Repository = s
	} else if m != "" {
		om.Repository = m
		m = ""
	}

	// Get namespace.
	m, s, ok = stringx.CutFromRight(m, "/")
	if ok && s != "" {
		om.Namespace = s
	} else if m != "" {
		om.Namespace = m
		m = ""
	}

	// Get registry.
	m, s, ok = stringx.CutFromLeft(m, "://")
	if ok && s != "" {
		om.Schema = m
		om.Registry = s
	} else if m != "" {
		om.Registry = m
	}

	if om.Repository == "" {
		return nil
	}
	return &om
}

func (om *OllamaModel) String() string {
	var b strings.Builder
	if om.Registry != "" {
		b.WriteString(om.Registry)
		b.WriteByte('/')
	}
	if om.Namespace != "" {
		b.WriteString(om.Namespace)
		b.WriteByte('/')
	}
	b.WriteString(om.Repository)
	if om.Tag != "" {
		b.WriteByte(':')
		b.WriteString(om.Tag)
	}
	return b.String()
}

// GetLayer returns the OllamaModelLayer with the given media type,
// and true if found, and false otherwise.
func (om *OllamaModel) GetLayer(mediaType string) (OllamaModelLayer, bool) {
	for i := range om.Layers {
		if om.Layers[i].MediaType == mediaType {
			return om.Layers[i], true
		}
	}
	return OllamaModelLayer{}, false
}

// SearchLayers returns a list of OllamaModelLayer with the media type that matches the given regex.
func (om *OllamaModel) SearchLayers(mediaTypeRegex *regexp.Regexp) []OllamaModelLayer {
	var ls []OllamaModelLayer
	for i := range om.Layers {
		if mediaTypeRegex.MatchString(om.Layers[i].MediaType) {
			ls = append(ls, om.Layers[i])
		}
	}
	return ls
}

// WebPageURL returns the Ollama web page URL of the OllamaModel.
func (om *OllamaModel) WebPageURL() *url.URL {
	u := &url.URL{
		Scheme: om.Schema,
		Host:   om.Registry,
	}
	return u.JoinPath(om.Namespace, om.Repository+":"+om.Tag)
}

// Complete completes the OllamaModel with the given context and http client.
func (om *OllamaModel) Complete(ctx context.Context, cli *http.Client) error {
	if om.Client == nil {
		om.Client = cli
	}

	u := &url.URL{
		Scheme: om.Schema,
		Host:   om.Registry,
	}
	u = u.JoinPath("v2", om.Namespace, om.Repository, "manifests", om.Tag)

	req, err := httpx.NewGetRequestWithContext(ctx, u.String())
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")

	err = httpx.Do(om.Client, req, func(resp *http.Response) error {
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("status code %d", resp.StatusCode)
		}
		return json.NewDecoder(resp.Body).Decode(om)
	})
	if err != nil {
		return fmt.Errorf("do request %s: %w", u, err)
	}

	// Connect.
	om.Config.Root = om
	for i := range om.Layers {
		om.Layers[i].Root = om
	}

	return nil
}

// Params returns the parameters of the OllamaModel.
func (om *OllamaModel) Params(ctx context.Context, cli *http.Client) (map[string]any, error) {
	if cli == nil {
		cli = om.Client
	}
	if cli == nil {
		return nil, fmt.Errorf("no client")
	}

	mls := om.SearchLayers(regexp.MustCompile(`^application/vnd\.ollama\.image\.params$`))
	if len(mls) == 0 {
		return nil, nil
	}

	rs := make([]map[string]any, len(mls))
	eg, ctx := errgroup.WithContext(ctx)
	for i := range mls {
		x := i
		eg.Go(func() error {
			bs, err := mls[x].FetchBlob(ctx, cli)
			if err == nil {
				p := make(map[string]any)
				if err = json.Unmarshal(bs, &p); err == nil {
					rs[x] = p
				}
			}
			return err
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("fetch blob: %w", err)
	}

	r := make(map[string]any)
	for i := range rs {
		for k, v := range rs[i] {
			r[k] = v
		}
	}
	return r, nil
}

// Template returns the template of the OllamaModel.
func (om *OllamaModel) Template(ctx context.Context, cli *http.Client) (string, error) {
	if cli == nil {
		cli = om.Client
	}
	if cli == nil {
		return "", fmt.Errorf("no client")
	}

	mls := om.SearchLayers(regexp.MustCompile(`^application/vnd\.ollama\.image\.(prompt|template)$`))
	if len(mls) == 0 {
		return "", nil
	}

	ml := mls[len(mls)-1]
	bs, err := ml.FetchBlob(ctx, cli)
	if err != nil {
		return "", fmt.Errorf("fetch blob: %w", err)
	}
	return stringx.FromBytes(&bs), nil
}

// System returns the system message of the OllamaModel.
func (om *OllamaModel) System(ctx context.Context, cli *http.Client) (string, error) {
	if cli == nil {
		cli = om.Client
	}
	if cli == nil {
		return "", fmt.Errorf("no client")
	}

	mls := om.SearchLayers(regexp.MustCompile(`^application/vnd\.ollama\.image\.system$`))
	if len(mls) == 0 {
		return "", nil
	}

	ml := mls[len(mls)-1]
	bs, err := ml.FetchBlob(ctx, cli)
	if err != nil {
		return "", fmt.Errorf("fetch blob: %w", err)
	}
	return stringx.FromBytes(&bs), nil
}

// License returns the license of the OllamaModel.
func (om *OllamaModel) License(ctx context.Context, cli *http.Client) ([]string, error) {
	if cli == nil {
		cli = om.Client
	}
	if cli == nil {
		return nil, fmt.Errorf("no client")
	}

	mls := om.SearchLayers(regexp.MustCompile(`^application/vnd\.ollama\.image\.license$`))
	if len(mls) == 0 {
		return nil, nil
	}

	rs := make([]string, len(mls))
	eg, ctx := errgroup.WithContext(ctx)
	for i := range mls {
		x := i
		eg.Go(func() error {
			bs, err := mls[x].FetchBlob(ctx, cli)
			if err == nil {
				rs[x] = stringx.FromBytes(&bs)
			}
			return err
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("fetch blob: %w", err)
	}
	return rs, nil
}

// Messages returns the messages of the OllamaModel.
func (om *OllamaModel) Messages(ctx context.Context, cli *http.Client) ([]json.RawMessage, error) {
	if cli == nil {
		cli = om.Client
	}
	if cli == nil {
		return nil, fmt.Errorf("no client")
	}

	mls := om.SearchLayers(regexp.MustCompile(`^application/vnd\.ollama\.image\.messages$`))
	if len(mls) == 0 {
		return nil, nil
	}

	rs := make([]json.RawMessage, len(mls))
	eg, ctx := errgroup.WithContext(ctx)
	for i := range mls {
		x := i
		eg.Go(func() error {
			bs, err := mls[x].FetchBlob(ctx, cli)
			if err == nil {
				rs[x] = bs
			}
			return err
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, fmt.Errorf("fetch blob: %w", err)
	}
	return rs, nil
}

// BlobURL returns the blob URL of the OllamaModelLayer.
func (ol *OllamaModelLayer) BlobURL() *url.URL {
	if ol.Root == nil {
		return nil
	}

	u := &url.URL{
		Scheme: ol.Root.Schema,
		Host:   ol.Root.Registry,
	}
	return u.JoinPath("v2", ol.Root.Namespace, ol.Root.Repository, "blobs", ol.Digest)
}

// FetchBlob fetches the blob of the OllamaModelLayer with the given context and http client,
// and returns the response body as bytes.
func (ol *OllamaModelLayer) FetchBlob(ctx context.Context, cli *http.Client) ([]byte, error) {
	var b []byte
	err := ol.FetchBlobFunc(ctx, cli, func(resp *http.Response) error {
		b = httpx.BodyBytes(resp)
		return nil
	})
	return b, err
}

// FetchBlobFunc fetches the blob of the OllamaModelLayer with the given context and http client,
// and processes the response with the given function.
func (ol *OllamaModelLayer) FetchBlobFunc(ctx context.Context, cli *http.Client, process func(*http.Response) error) error {
	if cli == nil {
		cli = ol.Root.Client
	}
	if cli == nil {
		return fmt.Errorf("no client")
	}

	u := ol.BlobURL()
	if u == nil {
		return fmt.Errorf("no blob URL")
	}

	req, err := httpx.NewGetRequestWithContext(ctx, u.String())
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}

	err = httpx.Do(cli, req, process)
	if err != nil {
		return fmt.Errorf("do request %s: %w", u, err)
	}
	return nil
}
