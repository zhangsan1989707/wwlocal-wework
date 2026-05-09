package model

type LogListResponse struct {
	FeatureID  int               `json:"feature_id"`
	LogTime    int64              `json:"log_time"`
	IDC        string             `json:"idc"`
	ParsedJSON map[string]interface{} `json:"parsed_json"`
}

type WeWorkLogItem struct {
	FeatureID int    `json:"feature_id"`
	LogTime   int64  `json:"log_time"`
	IDC       string `json:"idc"`
	EncKey    string `json:"encrypt_key"`
	EncData   string `json:"encrypt_data"`
}

type WeWorkGetTokenResponse struct {
	ErrCode     int    `json:"errcode"`
	ErrMsg      string `json:"errmsg"`
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
}

type WeWorkLogListResponse struct {
	ErrCode     int              `json:"errcode"`
	ErrMsg      string           `json:"errmsg"`
	FeatureID   int              `json:"feature_id"`
	LogList     []WeWorkLogItem  `json:"log_list"`
	StartIndex  int              `json:"start_index"`
	Limit       int              `json:"limit"`
	EndTime     int64            `json:"end_time"`
	HasMore     bool             `json:"has_more"`
}