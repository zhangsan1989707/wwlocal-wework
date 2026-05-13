package model

type WeWorkLogItem struct {
	FeatureID int    `json:"feature_id"`
	LogTime   int64  `json:"log_time"`
	IDC       string `json:"idc"`
	EncKey    string `json:"enc_key"`
	EncData   string `json:"enc_data"`
}
