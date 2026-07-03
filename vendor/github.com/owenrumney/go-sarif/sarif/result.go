package sarif

// Result represents the results block in the sarif report
type Result struct {
	PropertyBag
	Guid            *string                         `json:"guid,omitempty"`
	CorrelationGuid *string                         `json:"correlationGuid,omitempty"`
	RuleID          *string                         `json:"ruleId,omitempty"`
	RuleIndex       *uint                           `json:"ruleIndex,omitempty"`
	Rule            *ReportingDescriptorReference   `json:"rule,omitempty"`
	Taxa            []*ReportingDescriptorReference `json:"taxa,omitempty"`
	Kind            *string                         `json:"kind,omitempty"`
	Level           *string                         `json:"level,omitempty"`
	Message         Message                         `json:"message"`
	Locations       []*Location                     `json:"locations,omitempty"`
	AnalysisTarget  *ArtifactLocation               `json:"analysisTarget,omitempty"`
	// WebRequest			*webRequest						`json:"webRequest,omitempty"`
	// WebResponse			*webResponse					`json:"webResponse,omitempty"`
	Fingerprints        map[string]interface{} `json:"fingerprints,omitempty"`
	PartialFingerprints map[string]interface{} `json:"partialFingerprints,omitempty"`
	// CodeFlows			[]*codeFlows					`json:"codeFlows,omitempty"`
	// Graphs				[]*graphs						`json:"graphs,omitempty"`
	// GraphTraversals		[]*graphTraversals				`json:"graphTraversals,omitempty"`
	// Stacks				[]*stack						`json:"stacks,omitempty"`
	RelatedLocations []*Location    `json:"relatedLocations,omitempty"`
	Suppressions     []*Suppression `json:"suppressions,omitempty"`
	BaselineState    *string        `json:"baselineState,omitempty"`
	Rank             *float32       `json:"rank,omitempty"`
	// Attachments			[]*attachment					`json:"attachments,omitempty"`
	WorkItemUris    []string `json:"workItemUris,omitempty"` // can be null
	HostedViewerUri *string  `json:"hostedViewerUri,omitempty"`
	// Provenance			*resultProvenance				`json:"provenance,omitempty"`
	Fixes           []*Fix     `json:"fixes,omitempty"`
	OccurrenceCount *uint      `json:"occurrenceCount,omitempty"`
	Properties      Properties `json:"properties,omitempty"`
}

func newRuleResult(ruleID string) *Result {
	return &Result{
		RuleID: &ruleID,
	}
}

// WithGuid ...
func (r *Result) WithGuid(guid string) *Result {
	r.Guid = &guid
	return r
}

// WithCorrelationGuid ...
func (r *Result) WithCorrelationGuid(correlationGuid string) *Result {
	r.CorrelationGuid = &correlationGuid
	return r
}

// WithRuleIndex ...
func (r *Result) WithRuleIndex(ruleIndex int) *Result {
	index := uint(ruleIndex)
	r.RuleIndex = &index
	return r
}

// WithRule ...
func (r *Result) WithRule(rdp *ReportingDescriptorReference) *Result {
	r.Rule = rdp
	return r
}

// WithTaxa ...
func (r *Result) WithTaxa(rdp *ReportingDescriptorReference) *Result {
	r.Taxa = append(r.Taxa, rdp)
	return r
}

// WithKind ...
func (r *Result) WithKind(kind string) *Result {
	r.Kind = &kind
	return r
}

// WithLevel ...
func (r *Result) WithLevel(level string) *Result {
	r.Level = &level
	return r
}

// WithMessage ...
func (r *Result) WithMessage(message *Message) *Result {
	r.Message = *message
	return r
}

// WithLocation ...
func (r *Result) WithLocation(location *Location) *Result {
	r.Locations = append(r.Locations, location)
	return r
}

// WithAnalysisTarget ...
func (r *Result) WithAnalysisTarget(target *ArtifactLocation) *Result {
	r.AnalysisTarget = target
	return r
}

// WithFingerPrints ...
func (r *Result) WithFingerPrints(fingerPrints map[string]interface{}) *Result {
	r.Fingerprints = fingerPrints
	return r
}

// WithPartialFingerPrints ...
func (r *Result) WithPartialFingerPrints(fingerPrints map[string]interface{}) *Result {
	r.PartialFingerprints = fingerPrints
	return r
}

// WithRelatedLocation ...
func (r *Result) WithRelatedLocation(location *Location) *Result {
	r.RelatedLocations = append(r.RelatedLocations, location)
	return r
}

// WithSuppression ...
func (r *Result) WithSuppression(suppression *Suppression) *Result {
	r.Suppressions = append(r.Suppressions, suppression)
	return r
}

// WithBaselineState ...
func (r *Result) WithBaselineState(state string) *Result {
	r.BaselineState = &state
	return r
}

// WithRank ...
func (r *Result) WithRank(rank float32) *Result {
	r.Rank = &rank
	return r
}

// WithWorkItemUri ...
func (r *Result) WithWorkItemUri(workItemUri string) *Result {
	r.WorkItemUris = append(r.WorkItemUris, workItemUri)
	return r
}

// WithHostedViewerUri ...
func (r *Result) WithHostedViewerUri(hostedViewerUri string) *Result {
	r.HostedViewerUri = &hostedViewerUri
	return r
}

// WithFix ...
func (r *Result) WithFix(fix *Fix) *Result {
	r.Fixes = append(r.Fixes, fix)
	return r
}

// WithOccurrenceCount ...
func (r *Result) WithOccurrenceCount(occurrenceCount int) *Result {
	count := uint(occurrenceCount)
	r.OccurrenceCount = &count
	return r
}

// WithProperties specifies properties for a rule and returns the updated rule
func (r *Result) WithProperties(properties Properties) *Result {
	r.Properties = properties
	return r
}

// AttachPropertyBag adds a property bag to a rule
func (r *Result) AttachPropertyBag(pb *PropertyBag) {
	r.Properties = pb.Properties
}
