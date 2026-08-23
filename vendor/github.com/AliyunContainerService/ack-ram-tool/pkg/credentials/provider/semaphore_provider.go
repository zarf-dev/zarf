package provider

import (
	"context"
	"fmt"
	"golang.org/x/sync/semaphore"
)

type SemaphoreProvider struct {
	weighted *semaphore.Weighted

	cp CredentialsProvider
}

type SemaphoreProviderOptions struct {
	MaxWeight int64
}

func NewSemaphoreProvider(cp CredentialsProvider, opts SemaphoreProviderOptions) *SemaphoreProvider {
	opts.applyDefaults()

	w := semaphore.NewWeighted(opts.MaxWeight)
	return &SemaphoreProvider{
		weighted: w,
		cp:       cp,
	}
}

func (p *SemaphoreProvider) Credentials(ctx context.Context) (*Credentials, error) {
	if err := p.weighted.Acquire(ctx, 1); err != nil {
		return nil, fmt.Errorf("acquire semaphore: %w", err)
	}
	defer p.weighted.Release(1)

	return p.cp.Credentials(ctx)
}

func (o *SemaphoreProviderOptions) applyDefaults() {
	if o.MaxWeight <= 0 {
		o.MaxWeight = 1
	}
}

func (p *SemaphoreProvider) Stop(ctx context.Context) {
	if s, ok := p.cp.(Stopper); ok {
		s.Stop(ctx)
	}
}
