package provider

import "context"

type NotEnableError struct {
	err error
}

type NoAvailableProviderError struct {
	err error
}

func NewNotEnableError(err error) *NotEnableError {
	return &NotEnableError{err: err}
}

func NewNoAvailableProviderError(err error) *NoAvailableProviderError {
	return &NoAvailableProviderError{err: err}
}

func (e NotEnableError) Error() string {
	return e.err.Error()
}

func (e NoAvailableProviderError) Error() string {
	return e.err.Error()
}

func IsNotEnableError(err error) bool {
	if _, ok := err.(*NotEnableError); ok {
		return true
	}
	if _, ok := err.(NotEnableError); ok {
		return true
	}
	return false
}

func IsNoAvailableProviderError(err error) bool {
	if _, ok := err.(*NoAvailableProviderError); ok {
		return true
	}
	if _, ok := err.(NoAvailableProviderError); ok {
		return true
	}
	return false
}

type errorProvider struct {
	err error
}

func (e errorProvider) Credentials(ctx context.Context) (*Credentials, error) {
	return nil, e.err
}
