package sarif

// Properties ...
type Properties map[string]interface{}

// PropertyBag ...
type PropertyBag struct {
	Properties Properties `json:"properties,omitempty"`
}

// NewPropertyBag ...
func NewPropertyBag() *PropertyBag {
	return &PropertyBag{
		Properties: Properties{},
	}
}

// Add ...
func (pb *PropertyBag) Add(key string, value interface{}) {
	pb.Properties[key] = value
}

// AddString ...
func (pb *PropertyBag) AddString(key, value string) {
	pb.Add(key, value)
}

// AddBoolean ...
func (pb *PropertyBag) AddBoolean(key string, value bool) {
	pb.Add(key, value)
}

// AddInteger ...
func (pb *PropertyBag) AddInteger(key string, value int) {
	pb.Add(key, value)
}
