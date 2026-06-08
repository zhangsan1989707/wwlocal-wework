package repository

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"wwlocal-wework/internal/model"
)

type behaviorField struct {
	Column string
	Label  string
	Array  bool
}

var behaviorFieldsByFeature = map[int][]behaviorField{
	90000031: {{Column: "login_user_openid", Label: "登录用户"}},
	90000032: {{Column: "login_user_openid", Label: "唤醒用户"}},
	90000033: {{Column: "user_openid", Label: "访问用户"}},
	90000034: {{Column: "to_user", Label: "接收用户", Array: true}},
	90000035: {{Column: "sender_openid", Label: "发送人"}},
	90000036: {{Column: "sender_openid", Label: "发送人"}, {Column: "receiver_openid", Label: "接收人"}},
	90000037: {{Column: "sender_openid", Label: "发送人"}, {Column: "receiver", Label: "群成员", Array: true}},
	90000038: {{Column: "creator_openid", Label: "创建人"}, {Column: "members", Label: "群成员", Array: true}},
	90000039: {{Column: "oper_openid", Label: "操作人"}, {Column: "add_members", Label: "新增成员", Array: true}, {Column: "original_members", Label: "原群成员", Array: true}},
	90000040: {{Column: "oper_openid", Label: "操作人"}, {Column: "del_members", Label: "移除成员", Array: true}, {Column: "original_members", Label: "原群成员", Array: true}},
	90000041: {{Column: "quit_user_openid", Label: "退群用户"}, {Column: "original_members", Label: "原群成员", Array: true}},
	90000042: {{Column: "oper_openid", Label: "操作人"}, {Column: "new_owner_openid", Label: "新群主"}, {Column: "members", Label: "群成员", Array: true}},
	90000043: {{Column: "oper_openid", Label: "操作人"}, {Column: "members", Label: "群成员", Array: true}},
	90000044: {{Column: "oper_openid", Label: "操作人"}},
	90000046: {{Column: "user_openid", Label: "用户"}, {Column: "agent_user_openid", Label: "管理端用户"}},
	90000047: {{Column: "user_openid", Label: "添加用户"}},
	90000048: {{Column: "user_openid", Label: "激活用户"}},
	90000054: {{Column: "openid", Label: "用户"}},
	90000055: {{Column: "openid", Label: "用户"}},
	90000058: {{Column: "openid", Label: "用户"}},
	90000059: {{Column: "openid", Label: "用户"}},
}

func (r *LogRepository) QueryBehavior(featureIDs []int, openid string, startTime, endTime int64, page, pageSize int) ([]model.BehaviorRecord, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}

	months := r.monthsBetween(startTime, endTime)
	var all []model.BehaviorRecord

	for _, featureID := range featureIDs {
		fields := behaviorFieldsByFeature[featureID]
		if len(fields) == 0 {
			continue
		}
		for _, month := range months {
			tableName := r.GetTableName(featureID, month)
			if !r.TableExists(tableName) {
				continue
			}
			rows, err := r.queryBehaviorTable(tableName, featureID, fields, openid, startTime, endTime)
			if err != nil {
				return nil, 0, err
			}
			all = append(all, rows...)
		}
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].LogTime > all[j].LogTime
	})

	total := int64(len(all))
	start := (page - 1) * pageSize
	if start >= len(all) {
		return []model.BehaviorRecord{}, total, nil
	}
	end := start + pageSize
	if end > len(all) {
		end = len(all)
	}
	return all[start:end], total, nil
}

func (r *LogRepository) queryBehaviorTable(tableName string, featureID int, fields []behaviorField, openid string, startTime, endTime int64) ([]model.BehaviorRecord, error) {
	whereParts := []string{"log_time >= ?", "log_time <= ?"}
	args := []interface{}{startTime, endTime}
	for _, field := range fields {
		if field.Array {
			whereParts = append(whereParts, fmt.Sprintf("(%s IS NOT NULL AND %s != '' AND %s LIKE ?)", field.Column, field.Column, field.Column))
			args = append(args, "%"+openid+"%")
		} else {
			whereParts = append(whereParts, fmt.Sprintf("%s = ?", field.Column))
			args = append(args, openid)
		}
	}
	timeWhere := strings.Join(whereParts[:2], " AND ")
	matchWhere := strings.Join(whereParts[2:], " OR ")
	sql := fmt.Sprintf("SELECT * FROM %s WHERE %s AND (%s) ORDER BY log_time DESC", tableName, timeWhere, matchWhere)

	var rawRows []map[string]interface{}
	if err := r.DB.Raw(sql, args...).Scan(&rawRows).Error; err != nil {
		return nil, fmt.Errorf("query behavior table %s failed: %w", tableName, err)
	}

	var records []model.BehaviorRecord
	for _, row := range rawRows {
		matches := matchedFields(row, fields, openid)
		if len(matches) == 0 {
			continue
		}
		logTime := rowInt64(row["log_time"])
		records = append(records, model.BehaviorRecord{
			FeatureID:     featureID,
			LogTime:       logTime,
			LogDate:       time.Unix(logTime, 0).Format("2006-01-02 15:04:05"),
			MatchedFields: matches,
			Data:          normalizeBehaviorRow(row),
		})
	}
	return records, nil
}

func matchedFields(row map[string]interface{}, fields []behaviorField, openid string) []model.MatchedField {
	var matches []model.MatchedField
	for _, field := range fields {
		value := rowString(row[field.Column])
		if value == "" {
			continue
		}
		if field.Array {
			if jsonTextContainsOpenID(value, openid) {
				matches = append(matches, model.MatchedField{Field: field.Column, Label: field.Label, Value: openid})
			}
			continue
		}
		if value == openid {
			matches = append(matches, model.MatchedField{Field: field.Column, Label: field.Label, Value: value})
		}
	}
	return matches
}

func jsonTextContainsOpenID(text, openid string) bool {
	var value interface{}
	if err := json.Unmarshal([]byte(text), &value); err != nil {
		return strings.Contains(text, openid)
	}
	return jsonValueContainsOpenID(value, openid)
}

func jsonValueContainsOpenID(value interface{}, openid string) bool {
	switch v := value.(type) {
	case string:
		return v == openid
	case []interface{}:
		for _, item := range v {
			if jsonValueContainsOpenID(item, openid) {
				return true
			}
		}
	case map[string]interface{}:
		if s, ok := v["openid"].(string); ok && s == openid {
			return true
		}
		if s, ok := v["userid"].(string); ok && s == openid {
			return true
		}
		for _, item := range v {
			if jsonValueContainsOpenID(item, openid) {
				return true
			}
		}
	}
	return false
}

func normalizeBehaviorRow(row map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(row))
	for key, value := range row {
		if bytes, ok := value.([]byte); ok {
			result[key] = string(bytes)
		} else {
			result[key] = value
		}
	}
	return result
}

func rowString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		if v != nil {
			return fmt.Sprintf("%v", v)
		}
	}
	return ""
}

func rowInt64(value interface{}) int64 {
	switch v := value.(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case uint64:
		return int64(v)
	case []byte:
		var parsed int64
		fmt.Sscanf(string(v), "%d", &parsed)
		return parsed
	default:
		var parsed int64
		fmt.Sscanf(fmt.Sprintf("%v", v), "%d", &parsed)
		return parsed
	}
}
