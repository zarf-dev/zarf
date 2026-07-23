package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type URIProvider struct {
	u *Updater

	url string

	client *commonHttpClient
	Logger Logger
}

type URIProviderOptions struct {
	Timeout   time.Duration
	Transport http.RoundTripper

	ExpiryWindow  time.Duration
	RefreshPeriod time.Duration
	Logger        Logger
}

func NewURIProvider(url string, opts URIProviderOptions) *URIProvider {
	opts.applyDefaults()

	client := newCommonHttpClient(opts.Transport, opts.Timeout)
	client.logger = opts.Logger

	e := &URIProvider{
		url:    url,
		client: client,
		Logger: opts.Logger,
	}
	e.u = NewUpdater(e.getCredentials, UpdaterOptions{
		ExpiryWindow:  opts.ExpiryWindow,
		RefreshPeriod: opts.RefreshPeriod,
		Logger:        opts.Logger,
		LogPrefix:     "[URIProvider]",
	})
	e.u.Start(context.TODO())

	return e
}

func (e *URIProvider) Credentials(ctx context.Context) (*Credentials, error) {
	return e.u.Credentials(ctx)
}

func (e *URIProvider) Stop(ctx context.Context) {
	e.u.Stop(ctx)
}

func (e *URIProvider) getCredentials(ctx context.Context) (*Credentials, error) {
	if e.url == "" {
		return nil, NewNotEnableError(errors.New("URL is empty"))
	}

	data, err := e.client.send(ctx, http.MethodGet, e.url, http.Header{}, nil)
	if err != nil {
		return nil, err
	}

	var obj ecsMetadataStsResponse
	if err := json.Unmarshal([]byte(data), &obj); err != nil {
		return nil, fmt.Errorf("parse credentials failed: %w", err)
	}
	if obj.AccessKeyId == "" || obj.AccessKeySecret == "" {
		return nil, fmt.Errorf("parse credentials got unexpected data: %s",
			strings.ReplaceAll(data, "\n", " "))
	}

	var exp time.Time
	if obj.Expiration != "" {
		exp, err = time.Parse("2006-01-02T15:04:05Z", obj.Expiration)
		if err != nil {
			return nil, fmt.Errorf("parse Expiration failed: %w", err)
		}
	}

	return &Credentials{
		AccessKeyId:     obj.AccessKeyId,
		AccessKeySecret: obj.AccessKeySecret,
		SecurityToken:   obj.SecurityToken,
		Expiration:      exp,
	}, nil
}

func (e *URIProvider) logger() Logger {
	if e.Logger != nil {
		return e.Logger
	}
	return defaultLog
}

func (o *URIProviderOptions) applyDefaults() {
	if o.Timeout <= 0 {
		o.Timeout = defaultClientTimeout
	}
	if o.Transport == nil {
		ts := http.DefaultTransport.(*http.Transport).Clone()
		o.Transport = ts
	}
	if o.ExpiryWindow == 0 {
		o.ExpiryWindow = defaultExpiryWindow
	}
	if o.Logger == nil {
		o.Logger = defaultLog
	}
}
