package model

// QueryRequest 统一的查询请求类型
type QueryRequest struct {
	FeatureIDs []int                  `json:"feature_ids"`
	StartTime  int64                  `json:"start_time"`
	EndTime    int64                  `json:"end_time"`
	Conditions map[string]interface{} `json:"conditions"`
	Mobile     string                 `json:"mobile"`
	Page       int                    `json:"page"`
	PageSize   int                    `json:"page_size"`
	Realtime   bool                   `json:"realtime"`
	Cursor     int64                  `json:"cursor"`
}

type QueryResult struct {
	Total    int64                    `json:"total"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
	Data     []map[string]interface{} `json:"data"`
}

type CursorQueryResult struct {
	Total  int64                    `json:"total"`
	Cursor int64                    `json:"cursor"`
	Data   []map[string]interface{} `json:"data"`
}
