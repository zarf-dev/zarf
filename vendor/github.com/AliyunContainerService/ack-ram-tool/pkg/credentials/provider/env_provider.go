package provider

import (
	"context"
	"errors"
	"fmt"
	"os"
)

const (
	envAccessKeyId     = "ALIBABA_CLOUD_ACCESS_KEY_ID"
	envAccessKeySecret = "ALIBABA_CLOUD_ACCESS_KEY_SECRET"
	envSecurityToken   = "ALIBABA_CLOUD_SECURITY_TOKEN"
)

type EnvProvider struct {
	cp CredentialsProvider
}

type EnvProviderOptions struct {
	EnvAccessKeyId     string
	EnvAccessKeySecret string
	EnvSecurityToken   string

	EnvRoleArn         string
	EnvOIDCProviderArn string
	EnvOIDCTokenFile   string

	EnvCredentialsURI string

	stsEndpoint string
}

func NewEnvProvider(opts EnvProviderOptions) *EnvProvider {
	opts.applyDefaults()

	e := &EnvProvider{}
	e.cp = e.getProvider(opts)

	return e
}

func (e *EnvProvider) Credentials(ctx context.Context) (*Credentials, error) {
	cred, err := e.cp.Credentials(ctx)

	if err != nil {
		if IsNoAvailableProviderError(err) {
			return nil, NewNotEnableError(fmt.Errorf("not found credentials from env: %w", err))
		}
		return nil, err
	}

	return cred.DeepCopy(), nil
}

func (e *EnvProvider) Stop(ctx context.Context) {
	if s, ok := e.cp.(Stopper); ok {
		s.Stop(ctx)
	}
}

func (e *EnvProvider) getProvider(opts EnvProviderOptions) CredentialsProvider {
	accessKeyId := os.Getenv(opts.EnvAccessKeyId)
	accessKeySecret := os.Getenv(opts.EnvAccessKeySecret)
	securityToken := os.Getenv(opts.EnvSecurityToken)
	roleArn := os.Getenv(opts.EnvRoleArn)
	oidcProviderArn := os.Getenv(opts.EnvOIDCProviderArn)
	oidcTokenFile := os.Getenv(opts.EnvOIDCTokenFile)
	credentialsURI := os.Getenv(opts.EnvCredentialsURI)

	switch {
	case accessKeyId != "" && accessKeySecret != "" && securityToken != "":
		return NewSTSTokenProvider(
			os.Getenv(opts.EnvAccessKeyId),
			os.Getenv(opts.EnvAccessKeySecret),
			os.Getenv(opts.EnvSecurityToken),
		)

	case roleArn != "" && oidcProviderArn != "" && oidcTokenFile != "":
		return NewOIDCProvider(OIDCProviderOptions{
			RoleArn:         os.Getenv(opts.EnvRoleArn),
			OIDCProviderArn: os.Getenv(opts.EnvOIDCProviderArn),
			OIDCTokenFile:   os.Getenv(opts.EnvOIDCTokenFile),
			STSEndpoint:     opts.stsEndpoint,
		})

	case credentialsURI != "":
		return NewURIProvider(credentialsURI, URIProviderOptions{})

	case accessKeyId != "" && accessKeySecret != "":
		return NewAccessKeyProvider(
			os.Getenv(opts.EnvAccessKeyId),
			os.Getenv(opts.EnvAccessKeySecret),
		)

	default:
		return &errorProvider{
			err: NewNoAvailableProviderError(
				errors.New("no validated credentials were found in environment variables")),
		}
	}
}

func (o *EnvProviderOptions) applyDefaults() {
	if o.EnvAccessKeyId == "" {
		o.EnvAccessKeyId = envAccessKeyId
	}
	if o.EnvAccessKeySecret == "" {
		o.EnvAccessKeySecret = envAccessKeySecret
	}
	if o.EnvSecurityToken == "" {
		o.EnvSecurityToken = envSecurityToken
	}

	if o.EnvRoleArn == "" {
		o.EnvRoleArn = defaultEnvRoleArn
	}
	if o.EnvOIDCProviderArn == "" {
		o.EnvOIDCProviderArn = defaultEnvOIDCProviderArn
	}
	if o.EnvOIDCTokenFile == "" {
		o.EnvOIDCTokenFile = defaultEnvOIDCTokenFile
	}

	if o.EnvCredentialsURI == "" {
		o.EnvCredentialsURI = envCredentialsURI
	}
}
