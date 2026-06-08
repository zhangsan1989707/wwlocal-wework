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
	Field string `json:"field"`
	Label string `json:"label"`
	Value string `json:"value"`
}

type BehaviorRecord struct {
	FeatureID     int                    `json:"feature_id"`
	FeatureName   string                 `json:"feature_name"`
	LogTime       int64                  `json:"log_time"`
	LogDate       string                 `json:"log_date"`
	MatchedFields []MatchedField         `json:"matched_fields"`
	Data          map[string]interface{} `json:"data"`
}

type BehaviorQueryResult struct {
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	PageSize int              `json:"page_size"`
	Data     []BehaviorRecord `json:"data"`
}
