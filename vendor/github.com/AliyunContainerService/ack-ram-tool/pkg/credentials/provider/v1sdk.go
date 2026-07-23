package provider

import (
	"context"
	"fmt"
	"time"
)

type SignerForV1SDK struct {
	p                          CredentialsProvider
	Logger                     Logger
	credentialRetrievalTimeout time.Duration
}

type SignerForV1SDKOptions struct {
	Logger                     Logger
	CredentialRetrievalTimeout time.Duration
}

func NewSignerForV1SDK(p CredentialsProvider, opts SignerForV1SDKOptions) *SignerForV1SDK {
	opts.applyDefaults()

	return &SignerForV1SDK{
		p:                          p,
		Logger:                     opts.Logger,
		credentialRetrievalTimeout: opts.CredentialRetrievalTimeout,
	}
}

func (s *SignerForV1SDK) GetName() string {
	return "HMAC-SHA1"
}

func (s *SignerForV1SDK) GetType() string {
	return ""
}

func (s *SignerForV1SDK) GetVersion() string {
	return "1.0"
}

func (s *SignerForV1SDK) GetAccessKeyId() (string, error) {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), s.credentialRetrievalTimeout)
	defer cancel()
	cred, err := s.p.Credentials(timeoutCtx)
	if err != nil {
		return "", err
	}
	return cred.AccessKeyId, nil
}

func (s *SignerForV1SDK) GetExtraParam() map[string]string {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), s.credentialRetrievalTimeout)
	defer cancel()
	cred, err := s.p.Credentials(timeoutCtx)
	if err != nil {
		s.logger().Error(err, fmt.Sprintf("get credentials failed: %s", err))
		return nil
	}
	if cred.SecurityToken != "" {
		return map[string]string{"SecurityToken": cred.SecurityToken}
	}
	return nil
}

func (s *SignerForV1SDK) Sign(stringToSign, secretSuffix string) string {
	timeoutCtx, cancel := context.WithTimeout(context.Background(), s.credentialRetrievalTimeout)
	defer cancel()
	cred, err := s.p.Credentials(timeoutCtx)
	if err != nil {
		s.logger().Error(err, fmt.Sprintf("get credentials failed: %s", err))
		return ""
	}
	secret := cred.AccessKeySecret + secretSuffix
	return shaHmac1(stringToSign, secret)
}

func (s *SignerForV1SDK) logger() Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return defaultLog
}

func (o *SignerForV1SDKOptions) applyDefaults() {
	if o.Logger == nil {
		o.Logger = defaultLog
	}
	if o.CredentialRetrievalTimeout <= 0 {
		o.CredentialRetrievalTimeout = defaultTimeout
	}
}
