package provider

import (
	"context"
	"errors"
	"time"
)

type STSTokenProvider struct {
	cred *Credentials
}

func NewSTSTokenProvider(accessKeyId, accessKeySecret, securityToken string) *STSTokenProvider {
	return &STSTokenProvider{
		cred: &Credentials{
			AccessKeyId:     accessKeyId,
			AccessKeySecret: accessKeySecret,
			SecurityToken:   securityToken,
		},
	}
}

func (a *STSTokenProvider) Credentials(ctx context.Context) (*Credentials, error) {
	if a.cred.AccessKeyId == "" || a.cred.AccessKeySecret == "" || a.cred.SecurityToken == "" {
		return nil, NewNotEnableError(
			errors.New("AccessKeyId, AccessKeySecret or SecurityToken is empty"))
	}

	return a.cred.DeepCopy(), nil
}

func (a *STSTokenProvider) SetExpiration(exp time.Time) *STSTokenProvider {
	a.cred.Expiration = exp
	return a
}
