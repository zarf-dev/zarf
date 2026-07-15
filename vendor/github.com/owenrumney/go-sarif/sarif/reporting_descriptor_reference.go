package sarif

// ReportingDescriptorReference ...
type ReportingDescriptorReference struct {
	PropertyBag
	Id            *string                 `json:"id,omitempty"`
	Index         *uint                   `json:"index,omitempty"`
	Guid          *string                 `json:"guid,omitempty"`
	ToolComponent *ToolComponentReference `json:"toolComponent,omitempty"`
}

// NewReportingDescriptorReference ...
func NewReportingDescriptorReference() *ReportingDescriptorReference {
	return &ReportingDescriptorReference{}
}

// WithId ...
func (r *ReportingDescriptorReference) WithId(id string) *ReportingDescriptorReference {
	r.Id = &id
	return r
}

// WithIndex ...
func (r *ReportingDescriptorReference) WithIndex(index int) *ReportingDescriptorReference {
	i := uint(index)
	r.Index = &i
	return r
}

// WithGuid ...
func (r *ReportingDescriptorReference) WithGuid(guid string) *ReportingDescriptorReference {
	r.Guid = &guid
	return r
}

// WithToolComponentReference ...
func (r *ReportingDescriptorReference) WithToolComponentReference(toolComponentRef *ToolComponentReference) *ReportingDescriptorReference {
	r.ToolComponent = toolComponentRef
	return r
}
