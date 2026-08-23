package provider

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

var defaultRuntimeSwitchCacheDuration = time.Minute * 15

type ChainProvider struct {
	providers []CredentialsProvider

	currentProvider CredentialsProvider
	Logger          Logger
	logPrefix       string

	enableRuntimeSwitch        bool
	runtimeSwitchCacheDuration time.Duration
	lastSelectProviderTime     time.Time

	lock sync.RWMutex
}

type ChainProviderOptions struct {
	EnableRuntimeSwitch        bool
	RuntimeSwitchCacheDuration time.Duration

	logPrefix string
}

func NewChainProvider(providers ...CredentialsProvider) *ChainProvider {
	return NewChainProviderWithOptions(providers, ChainProviderOptions{})
}

func NewChainProviderWithOptions(providers []CredentialsProvider, opts ChainProviderOptions) *ChainProvider {
	opts.applyDefaults()

	if len(providers) == 0 {
		return NewDefaultChainProvider(DefaultChainProviderOptions{
			EnableRuntimeSwitch:        opts.EnableRuntimeSwitch,
			RuntimeSwitchCacheDuration: opts.RuntimeSwitchCacheDuration,
		})
	}
	return &ChainProvider{
		enableRuntimeSwitch:        opts.EnableRuntimeSwitch,
		runtimeSwitchCacheDuration: opts.RuntimeSwitchCacheDuration,
		providers:                  providers,
		logPrefix:                  opts.logPrefix,
	}
}

func (c *ChainProvider) Credentials(ctx context.Context) (*Credentials, error) {
	return c.getCredentials(ctx)
}

func (c *ChainProvider) SelectProvider(ctx context.Context) (CredentialsProvider, error) {
	return c.selectProvider(ctx)
}

func (c *ChainProvider) Stop(ctx context.Context) {
	c.logger().Debug(fmt.Sprintf("%s start to stop...", c.logPrefix))

	for _, p := range c.providers {
		if s, ok := p.(Stopper); ok {
			s.Stop(ctx)
		}
	}
}

func (c *ChainProvider) getCredentials(ctx context.Context) (*Credentials, error) {
	p := c.getCurrentProvider()
	if p != nil {
		return p.Credentials(ctx)
	}

	p, err := c.selectProvider(ctx)
	if err != nil {
		return nil, err
	}
	c.setCurrentProvider(p)

	return p.Credentials(ctx)
}

func (c *ChainProvider) selectProvider(ctx context.Context) (CredentialsProvider, error) {
	var notEnableErrors []string
	for _, p := range c.providers {
		if _, err := p.Credentials(ctx); err != nil {
			if IsNotEnableError(err) {
				c.logger().Debug(fmt.Sprintf("%s provider %T is not enabled will try to next: %s",
					c.logPrefix, p, err.Error()))
				notEnableErrors = append(notEnableErrors, fmt.Sprintf("provider %T is not enabled: %s", p, err.Error()))
				continue
			}
		}
		return p, nil
	}

	err := fmt.Errorf("no available credentials provider: [%s]", strings.Join(notEnableErrors, ", "))
	return nil, NewNoAvailableProviderError(err)
}

func (c *ChainProvider) getCurrentProvider() CredentialsProvider {
	c.lock.RLock()
	defer c.lock.RUnlock()

	if !c.enableRuntimeSwitch {
		return c.currentProvider
	}

	p := c.currentProvider
	if p == nil {
		return nil
	}
	if c.lastSelectProviderTime.IsZero() {
		return nil
	}
	if time.Since(c.lastSelectProviderTime) >= c.runtimeSwitchCacheDuration {
		c.logger().Debug(fmt.Sprintf("%s trigger select provider again", c.logPrefix))
		return nil
	}

	return p
}

func (c *ChainProvider) setCurrentProvider(p CredentialsProvider) {
	c.lock.Lock()
	defer c.lock.Unlock()

	prePT := fmt.Sprintf("%T", c.currentProvider)
	pT := fmt.Sprintf("%T", p)
	if prePT != pT {
		c.logger().Info(fmt.Sprintf("%s switch to using new provider: %s -> %s", c.logPrefix, prePT, pT))
	}

	c.lastSelectProviderTime = time.Now()
	c.currentProvider = p
}

func (c *ChainProvider) logger() Logger {
	if c.Logger != nil {
		return c.Logger
	}
	return defaultLog
}

type DefaultChainProviderOptions struct {
	EnableRuntimeSwitch        bool
	RuntimeSwitchCacheDuration time.Duration

	STSEndpoint   string
	ExpiryWindow  time.Duration
	RefreshPeriod time.Duration
	Logger        Logger

	logPrefix string
}

func NewDefaultChainProvider(opts DefaultChainProviderOptions) *ChainProvider {
	opts.applyDefaults()

	p := NewChainProviderWithOptions(
		[]CredentialsProvider{
			NewEnvProvider(EnvProviderOptions{}),
			NewOIDCProvider(OIDCProviderOptions{
				STSEndpoint:   opts.STSEndpoint,
				ExpiryWindow:  opts.ExpiryWindow,
				RefreshPeriod: opts.RefreshPeriod,
				Logger:        opts.Logger,
			}),
			NewEncryptedFileProvider(EncryptedFileProviderOptions{
				ExpiryWindow:  opts.ExpiryWindow,
				RefreshPeriod: opts.RefreshPeriod,
				Logger:        opts.Logger,
			}),
			NewECSMetadataProvider(ECSMetadataProviderOptions{
				ExpiryWindow:  opts.ExpiryWindow,
				RefreshPeriod: opts.RefreshPeriod,
				Logger:        opts.Logger,
			}),
		},
		ChainProviderOptions{
			EnableRuntimeSwitch:        opts.EnableRuntimeSwitch,
			RuntimeSwitchCacheDuration: opts.RuntimeSwitchCacheDuration,
			logPrefix:                  opts.logPrefix,
		},
	)
	p.Logger = opts.Logger
	return p
}

// Deprecated: use NewDefaultChainProvider instead
func DefaultChainProvider() *ChainProvider {
	return NewDefaultChainProvider(DefaultChainProviderOptions{})
}

// Deprecated: use NewDefaultChainProvider instead
func DefaultChainProviderWithLogger(l Logger) *ChainProvider {
	return NewDefaultChainProvider(DefaultChainProviderOptions{
		Logger: l,
	})
}

func (o *ChainProviderOptions) applyDefaults() {
	if o.RuntimeSwitchCacheDuration <= 0 {
		o.RuntimeSwitchCacheDuration = defaultRuntimeSwitchCacheDuration
	}
	if o.logPrefix == "" {
		o.logPrefix = "[ChainProvider]"
	}
}

func (o *DefaultChainProviderOptions) applyDefaults() {
	if o.logPrefix == "" {
		o.logPrefix = "[DefaultChainProvider]"
	}
}
