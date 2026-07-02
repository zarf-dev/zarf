package bubbly

import (
	"context"
	"errors"
	"fmt"
)

type PromptReader interface {
	Response(ctx context.Context) (string, error)
}

type PromptWriter interface {
	IsSensitive() bool
	PromptMessage() string
	Respond(string) error
	Validate(string) error
}

type Prompter struct {
	message    string
	value      *string
	responses  chan string // prompt responses come from the prompt writer
	validators []func(string) error
	sensitive  bool
}

func NewPrompter(message string, sensitive bool, validators ...func(string) error) *Prompter {
	return &Prompter{
		message:    message,
		responses:  make(chan string, 1),
		validators: validators,
		sensitive:  sensitive,
	}
}

func (p Prompter) PromptMessage() string {
	return p.message
}

func (p Prompter) IsSensitive() bool {
	return p.sensitive
}

func (p *Prompter) Validate(value string) error {
	for _, validator := range p.validators {
		if err := validator(value); err != nil {
			return err
		}
	}
	return nil
}

func (p *Prompter) Respond(value string) error {
	if p.value != nil {
		return fmt.Errorf("prompt cannot take another value")
	}

	p.value = &value
	p.responses <- value
	close(p.responses)
	return nil
}

func (p *Prompter) Response(ctx context.Context) (string, error) {
	if p.value != nil {
		return *p.value, nil
	}
	select {
	case v := <-p.responses:
		return v, nil
	case <-ctx.Done():
		return "", errors.New("timeout")
	}
}
