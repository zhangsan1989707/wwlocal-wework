package model

type BehaviorQueryRequest struct {
	OpenID     string `json:"openid"`
	FeatureIDs []int  `json:"feature_ids"`
	StartTime  int64  `json:"start_time"`
	EndTime    int64  `json:"end_time"`
	Page       int    `json:"page"`
	PageSize   int    `json:"page_size"`
}

type MatchedField struct {
	Field        string `json:"field"`
	Label        string `json:"label"`
	Value        string `json:"value"`
	DisplayValue string `json:"display_value,omitempty"`
}

type BehaviorRecord struct {
	FeatureID     int                    `json:"feature_id"`
	FeatureName   string                 `json:"feature_name"`
	LogTime       int64                  `json:"log_time"`
	LogDate       string                 `json:"log_date"`
	MatchedFields []MatchedField         `json:"matched_fields"`
	Data          map[string]interface{} `json:"data"`
}

type BehaviorFeatureSummary struct {
	FeatureID     int    `json:"feature_id"`
	FeatureName   string `json:"feature_name"`
	Status        string `json:"status"`
	Reason        string `json:"reason,omitempty"`
	Tables        int    `json:"tables"`
	QueriedTables int    `json:"queried_tables"`
	MatchedRows   int64  `json:"matched_rows"`
}

type BehaviorQueryResult struct {
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
	Features []BehaviorFeatureSummary `json:"features"`
	Data     []BehaviorRecord         `json:"data"`
}
