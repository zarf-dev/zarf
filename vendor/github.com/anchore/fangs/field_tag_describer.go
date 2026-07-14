package fangs

import "reflect"

type structDescriptionTagProvider struct{}

var _ DescriptionProvider = (*structDescriptionTagProvider)(nil)

// NewStructDescriptionTagProvider returns a DescriptionProvider that returns "description" field tag values
func NewStructDescriptionTagProvider() DescriptionProvider {
	return &structDescriptionTagProvider{}
}

func (*structDescriptionTagProvider) GetDescription(_ reflect.Value, field reflect.StructField) string {
	return field.Tag.Get("description")
}
