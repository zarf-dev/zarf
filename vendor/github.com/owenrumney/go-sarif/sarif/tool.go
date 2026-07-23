package sarif

// Tool ...
type Tool struct {
	PropertyBag
	Driver *ToolComponent `json:"driver"`
}
