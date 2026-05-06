package growthbook

// UserAttributes holds the targeting attributes sent to the GrowthBook server
// for feature flag evaluation.
type UserAttributes struct {
	ID                string `json:"id"`
	SessionID         string `json:"sessionId"`
	DeviceID          string `json:"deviceID"`
	Platform          string `json:"platform"`
	APIBaseURLHost    string `json:"apiBaseUrlHost,omitempty"`
	OrganizationUUID  string `json:"organizationUUID,omitempty"`
	AccountUUID       string `json:"accountUUID,omitempty"`
	UserType          string `json:"userType,omitempty"`
	SubscriptionType  string `json:"subscriptionType,omitempty"`
	RateLimitTier     string `json:"rateLimitTier,omitempty"`
	FirstTokenTime    int64  `json:"firstTokenTime,omitempty"`
	Email             string `json:"email,omitempty"`
	AppVersion        string `json:"appVersion,omitempty"`
}

// StoredExperimentData holds experiment metadata used for exposure logging.
type StoredExperimentData struct {
	ExperimentID string `json:"experimentId"`
	VariationID  int    `json:"variationId"`
	InExperiment bool   `json:"inExperiment,omitempty"`
	HashAttribute string `json:"hashAttribute,omitempty"`
	HashValue    string `json:"hashValue,omitempty"`
}

// FeatureResult holds the result of a feature flag evaluation.
type FeatureResult struct {
	Value      interface{}
	Source     string // "envOverride", "configOverride", "remoteEval", "diskCache", "defaultValue"
	Experiment *StoredExperimentData
}

// GitHubActionsMetadata provides CI/CD context for GrowthBook targeting.
type GitHubActionsMetadata struct {
	Actor          string `json:"actor,omitempty"`
	Ref            string `json:"ref,omitempty"`
	EventName      string `json:"eventName,omitempty"`
	Repository     string `json:"repository,omitempty"`
}

// RefreshListener is a callback invoked when GrowthBook feature values refresh.
type RefreshListener func()

// GrowthBookRefreshListener is an alias for RefreshListener.
type GrowthBookRefreshListener = RefreshListener

// FeaturePayload represents a single feature definition from the remote eval API.
type FeaturePayload struct {
	DefaultValue    interface{}            `json:"defaultValue,omitempty"`
	Value           interface{}            `json:"value,omitempty"`
	Rules           []interface{}          `json:"rules,omitempty"`
	Source          string                 `json:"source,omitempty"`
	ExperimentResult map[string]interface{} `json:"experimentResult,omitempty"`
	Experiment     map[string]interface{} `json:"experiment,omitempty"`
}
