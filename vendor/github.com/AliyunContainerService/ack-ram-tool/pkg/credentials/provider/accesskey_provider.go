package provider

import (
	"context"
	"errors"
)

type AccessKeyProvider struct {
	cred *Credentials
}

func NewAccessKeyProvider(accessKeyId, accessKeySecret string) *AccessKeyProvider {
	return &AccessKeyProvider{
		cred: &Credentials{
			AccessKeyId:     accessKeyId,
			AccessKeySecret: accessKeySecret,
		},
	}
}

func (a *AccessKeyProvider) Credentials(ctx context.Context) (*Credentials, error) {
	if a.cred.AccessKeyId == "" || a.cred.AccessKeySecret == "" {
		return nil, NewNotEnableError(errors.New("AccessKeyId or AccessKeySecret is empty"))
	}

	return a.cred.DeepCopy(), nil
}
