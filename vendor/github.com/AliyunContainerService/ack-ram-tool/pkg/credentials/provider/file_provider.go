package provider

import (
	"context"
	"fmt"
	"os"
	"time"
)

type FileProvider struct {
	u *Updater

	filepath string
	decoder  func(data []byte) (*Credentials, error)
}

type FileProviderOptions struct {
	RefreshPeriod time.Duration
	ExpiryWindow  time.Duration
	Logger        Logger
	LogPrefix     string
}

func NewFileProvider(filepath string, decoder func(data []byte) (*Credentials, error), opts FileProviderOptions) *FileProvider {
	opts.applyDefaults()

	e := &FileProvider{
		filepath: filepath,
		decoder:  decoder,
	}
	e.u = NewUpdater(e.getCredentials, UpdaterOptions{
		ExpiryWindow:  opts.ExpiryWindow,
		RefreshPeriod: opts.RefreshPeriod,
		Logger:        opts.Logger,
		LogPrefix:     opts.LogPrefix,
	})
	e.u.Start(context.TODO())

	return e
}

func (f *FileProvider) Credentials(ctx context.Context) (*Credentials, error) {
	return f.u.Credentials(ctx)
}

func (f *FileProvider) Stop(ctx context.Context) {
	f.u.Stop(ctx)
}

func (f *FileProvider) getCredentials(ctx context.Context) (*Credentials, error) {
	data, err := os.ReadFile(f.filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, NewNotEnableError(fmt.Errorf("read file %s failed: %w", f.filepath, err))
		}
		return nil, fmt.Errorf("read file %s failed: %w", f.filepath, err)
	}

	cred, err := f.decoder(data)
	if err != nil {
		return nil, fmt.Errorf("decode data from %s failed: %w", f.filepath, err)
	}
	return cred, nil
}

func (f *FileProviderOptions) applyDefaults() {
	if f.ExpiryWindow == 0 {
		f.ExpiryWindow = defaultExpiryWindow
	}
	if f.Logger == nil {
		f.Logger = defaultLog
	}
	if f.LogPrefix == "" {
		f.LogPrefix = "[FileProvider]"
	}
}
