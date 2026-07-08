package provider

import (
	"context"
	"time"
)

var defaultTimeout = time.Minute * 10

type CredentialForV2SDK struct {
	p                          CredentialsProvider
	Logger                     Logger
	credentialRetrievalTimeout time.Duration
}

type CredentialForV2SDKOptions struct {
	Logger                     Logger
	CredentialRetrievalTimeout time.Duration
}

func NewCredentialForV2SDK(p CredentialsProvider, opts CredentialForV2SDKOptions) *CredentialForV2SDK {
	opts.applyDefaults()

	return &CredentialForV2SDK{
		p:                          p,
		Logger:                     opts.Logger,
		credentialRetrievalTimeout: opts.CredentialRetrievalTimeout,
	}
}

func (c *CredentialForV2SDK) GetAccessKeyId() (*string, error) {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), c.credentialRetrievalTimeout)
	defer cancel()
	cred, err := c.p.Credentials(timeoutCtx)
	if err != nil {
		return nil, err
	}
	return stringPointer(cred.AccessKeyId), nil
}

func (c *CredentialForV2SDK) GetAccessKeySecret() (*string, error) {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), c.credentialRetrievalTimeout)
	defer cancel()
	cred, err := c.p.Credentials(timeoutCtx)
	if err != nil {
		return nil, err
	}
	return stringPointer(cred.AccessKeySecret), nil
}

func (c *CredentialForV2SDK) GetSecurityToken() (*string, error) {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), c.credentialRetrievalTimeout)
	defer cancel()
	cred, err := c.p.Credentials(timeoutCtx)
	if err != nil {
		return nil, err
	}
	return stringPointer(cred.SecurityToken), nil
}

func (c *CredentialForV2SDK) GetBearerToken() *string {
	return stringPointer("")
}

func (c *CredentialForV2SDK) GetType() *string {
	return stringPointer("CredentialForV2SDK")
}

func (c *CredentialForV2SDK) logger() Logger {
	if c.Logger != nil {
		return c.Logger
	}
	return defaultLog
}

func (o *CredentialForV2SDKOptions) applyDefaults() {
	if o.Logger == nil {
		o.Logger = defaultLog
	}
	if o.CredentialRetrievalTimeout <= 0 {
		o.CredentialRetrievalTimeout = defaultTimeout
	}
}

func stringPointer(s string) *string {
	return &s
}
