package fangs

import "reflect"

type DescriptionProvider interface {
	GetDescription(value reflect.Value, field reflect.StructField) string
}

type combinedDescriptionProvider struct {
	providers []DescriptionProvider
}

var _ DescriptionProvider = (*combinedDescriptionProvider)(nil)

func (c combinedDescriptionProvider) GetDescription(value reflect.Value, field reflect.StructField) string {
	description := ""
	for _, p := range c.providers {
		description = p.GetDescription(value, field)
		if description != "" {
			break
		}
	}
	return description
}

func DescriptionProviders(providers ...DescriptionProvider) DescriptionProvider {
	return &combinedDescriptionProvider{
		providers: providers,
	}
}
