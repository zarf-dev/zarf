package provider

import (
	"context"
	"errors"
)

type FunctionProvider struct {
	getCredentials func(ctx context.Context) (*Credentials, error)
}

func NewFunctionProvider(getCredentials func(ctx context.Context) (*Credentials, error)) *FunctionProvider {
	return &FunctionProvider{
		getCredentials: getCredentials,
	}
}

func (f *FunctionProvider) Credentials(ctx context.Context) (*Credentials, error) {
	if f.getCredentials == nil {
		return nil, NewNotEnableError(errors.New("getCredentials function is nil"))
	}

	cred, err := f.getCredentials(ctx)
	if err != nil {
		return nil, err
	}
	return cred.DeepCopy(), nil
}
