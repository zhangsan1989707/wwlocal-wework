package repository

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"wwlocal-wework/internal/model"
)

func (r *LogRepository) QueryAcrossMonths(featureID int, startTime, endTime int64, page, pageSize int) ([]model.LogEntry, int64, error) {
	return r.QueryAcrossMonthsContext(context.Background(), featureID, startTime, endTime, page, pageSize)
}

func (r *LogRepository) QueryAcrossMonthsContext(ctx context.Context, featureID int, startTime, endTime int64, page, pageSize int) ([]model.LogEntry, int64, error) {
	return r.queryAcrossMonthsUnion(ctx, featureID, startTime, endTime, "", nil, page, pageSize)
}

func (r *LogRepository) QueryAcrossMonthsWithConditions(featureID int, startTime, endTime int64, conditions map[string]interface{}, mobile string, page, pageSize int) ([]model.LogEntry, int64, error) {
	return r.QueryAcrossMonthsWithConditionsContext(context.Background(), featureID, startTime, endTime, conditions, mobile, page, pageSize)
}

func (r *LogRepository) QueryAcrossMonthsWithConditionsContext(ctx context.Context, featureID int, startTime, endTime int64, conditions map[string]interface{}, mobile string, page, pageSize int) ([]model.LogEntry, int64, error) {
	jsonWhere, jsonArgs := r.buildJSONConditions(conditions, mobile)
	return r.queryAcrossMonthsUnion(ctx, featureID, startTime, endTime, jsonWhere, jsonArgs, page, pageSize)
}

func (r *LogRepository) queryAcrossMonthsUnion(ctx context.Context, featureID int, startTime, endTime int64, extraWhere string, extraArgs []interface{}, page, pageSize int) ([]model.LogEntry, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 100
	}

	months := r.monthsBetween(startTime, endTime)
	var total int64
	var selects []string
	var queryArgs []interface{}
	for _, month := range months {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		tableName := r.GetTableName(featureID, month)
		if !r.TableExists(tableName) {
			continue
		}
		where := "log_time >= ? AND log_time <= ?"
		args := []interface{}{startTime, endTime}
		if extraWhere != "" {
			where += " AND " + extraWhere
			args = append(args, extraArgs...)
		}

		var count int64
		if err := r.DB.WithContext(ctx).Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", tableName, where), args...).Scan(&count).Error; err != nil {
			return nil, 0, fmt.Errorf("count table %s failed: %w", tableName, err)
		}
		total += count

		selects = append(selects, fmt.Sprintf(`
			SELECT id, feature_id, log_time, idc, enc_data, enc_key, raw_json, parsed_json, created_at
			FROM %s WHERE %s
		`, tableName, where))
		queryArgs = append(queryArgs, args...)
	}

	if total == 0 {
		return []model.LogEntry{}, 0, nil
	}

	if len(selects) == 0 {
		return []model.LogEntry{}, total, nil
	}

	offset := (page - 1) * pageSize
	queryArgs = append(queryArgs, pageSize, offset)
	querySQL := fmt.Sprintf(`
		SELECT id, feature_id, log_time, idc, enc_data, enc_key, raw_json, parsed_json, created_at
		FROM (%s) AS merged
		ORDER BY log_time DESC, id DESC
		LIMIT ? OFFSET ?
	`, strings.Join(selects, " UNION ALL "))

	var entries []model.LogEntry
	if err := r.DB.WithContext(ctx).Raw(querySQL, queryArgs...).Scan(&entries).Error; err != nil {
		return nil, 0, fmt.Errorf("query feature %d across months failed: %w", featureID, err)
	}
	return entries, total, nil
}

func (r *LogRepository) QueryByCursor(featureID int, startTime, endTime int64, cursor int64, pageSize int, conditions map[string]interface{}, mobile string) ([]model.LogEntry, int64, int64, error) {
	return r.QueryByCursorContext(context.Background(), featureID, startTime, endTime, cursor, pageSize, conditions, mobile)
}

func (r *LogRepository) QueryByCursorContext(ctx context.Context, featureID int, startTime, endTime int64, cursor int64, pageSize int, conditions map[string]interface{}, mobile string) ([]model.LogEntry, int64, int64, error) {
	months := r.monthsBetween(startTime, endTime)
	jsonWhere, jsonArgs := r.buildJSONConditions(conditions, mobile)

	var total int64
	for _, month := range months {
		if err := ctx.Err(); err != nil {
			return nil, 0, 0, err
		}
		tableName := r.GetTableName(featureID, month)
		if !r.TableExists(tableName) {
			continue
		}
		where := "log_time >= ? AND log_time <= ?"
		args := []interface{}{startTime, endTime}
		if jsonWhere != "" {
			where += " AND " + jsonWhere
			args = append(args, jsonArgs...)
		}
		var count int64
		if err := r.DB.WithContext(ctx).Raw(fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE %s", tableName, where), args...).Scan(&count).Error; err != nil {
			slog.Info(fmt.Sprintf("count table %s failed: %v", tableName, err))
		}
		total += count
	}

	queryEndTime := endTime
	if cursor > 0 {
		queryEndTime = cursor
	}

	var allEntries []model.LogEntry
	remaining := pageSize
	for i := len(months) - 1; i >= 0 && remaining > 0; i-- {
		if err := ctx.Err(); err != nil {
			return nil, 0, 0, err
		}
		month := months[i]
		tableName := r.GetTableName(featureID, month)
		if !r.TableExists(tableName) {
			continue
		}

		where := "log_time >= ? AND log_time <= ?"
		args := []interface{}{startTime, queryEndTime}
		if jsonWhere != "" {
			where += " AND " + jsonWhere
			args = append(args, jsonArgs...)
		}
		if cursor > 0 {
			where += " AND log_time < ?"
			args = append(args, cursor)
		}

		querySQL := fmt.Sprintf(`
			SELECT id, feature_id, log_time, idc, enc_data, enc_key, raw_json, parsed_json, created_at
			FROM %s WHERE %s ORDER BY log_time DESC LIMIT ?
		`, tableName, where)

		var entries []model.LogEntry
		queryArgs := append(args, remaining)
		if err := r.DB.WithContext(ctx).Raw(querySQL, queryArgs...).Scan(&entries).Error; err != nil {
			slog.Info(fmt.Sprintf("query table %s failed: %v", tableName, err))
			continue
		}
		allEntries = append(allEntries, entries...)
		remaining -= len(entries)
	}

	var nextCursor int64 = 0
	if len(allEntries) >= pageSize {
		nextCursor = allEntries[len(allEntries)-1].LogTime
		if len(allEntries) > pageSize {
			allEntries = allEntries[:pageSize]
		}
	}

	return allEntries, total, nextCursor, nil
}

func validJSONPathKey(key string) bool {
	if key == "" {
		return false
	}
	for _, c := range key {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' || c == '.' {
			continue
		}
		return false
	}
	return true
}

func (r *LogRepository) buildJSONConditions(conditions map[string]interface{}, mobile string) (string, []interface{}) {
	var parts []string
	var args []interface{}

	for key, expected := range conditions {
		if !validJSONPathKey(key) {
			continue
		}
		if key == "login_user.openid" {
			matchValue, operator := r.parseCondition(expected)
			if operator == "LIKE" {
				parts = append(parts, "LOWER(login_openid) LIKE ?")
				args = append(args, "%"+strings.ToLower(fmt.Sprintf("%v", matchValue))+"%")
			} else {
				parts = append(parts, "LOWER(login_openid) = ?")
				args = append(args, strings.ToLower(fmt.Sprintf("%v", matchValue)))
			}
			continue
		}
		if key == "sender.openid" {
			matchValue, operator := r.parseCondition(expected)
			if operator == "LIKE" {
				parts = append(parts, "LOWER(sender_openid) LIKE ?")
				args = append(args, "%"+strings.ToLower(fmt.Sprintf("%v", matchValue))+"%")
			} else {
				parts = append(parts, "LOWER(sender_openid) = ?")
				args = append(args, strings.ToLower(fmt.Sprintf("%v", matchValue)))
			}
			continue
		}
		if key == "openid" {
			matchValue, operator := r.parseCondition(expected)
			if operator == "LIKE" {
				parts = append(parts, "LOWER(root_openid) LIKE ?")
				args = append(args, "%"+strings.ToLower(fmt.Sprintf("%v", matchValue))+"%")
			} else {
				parts = append(parts, "LOWER(root_openid) = ?")
				args = append(args, strings.ToLower(fmt.Sprintf("%v", matchValue)))
			}
			continue
		}

		jsonPath := "$." + key
		jsonExtract := fmt.Sprintf("JSON_UNQUOTE(JSON_EXTRACT(parsed_json, '%s'))", jsonPath)

		matchValue, operator := r.parseCondition(expected)

		switch v := matchValue.(type) {
		case string:
			if operator == "LIKE" {
				parts = append(parts, fmt.Sprintf("LOWER(%s) LIKE ?", jsonExtract))
				args = append(args, "%"+strings.ToLower(v)+"%")
			} else {
				parts = append(parts, fmt.Sprintf("LOWER(%s) = ?", jsonExtract))
				args = append(args, strings.ToLower(v))
			}
		case float64:
			parts = append(parts, fmt.Sprintf("%s = ?", jsonExtract))
			args = append(args, v)
		default:
			parts = append(parts, fmt.Sprintf("%s = ?", jsonExtract))
			args = append(args, fmt.Sprintf("%v", v))
		}
	}

	if mobile != "" {
		parts = append(parts, "(login_openid = ? OR root_openid = ? OR sender_openid = ?)")
		args = append(args, mobile, mobile, mobile)
	}

	if len(parts) == 0 {
		return "", nil
	}
	return strings.Join(parts, " AND "), args
}

func (r *LogRepository) parseCondition(expected interface{}) (value interface{}, operator string) {
	operator = "LIKE"
	matchValue := expected
	if condMap, isMap := expected.(map[string]interface{}); isMap {
		if v, exists := condMap["value"]; exists {
			matchValue = v
		}
		if op, exists := condMap["operator"]; exists {
			if opStr, ok := op.(string); ok && opStr == "=" {
				operator = "="
			}
		}
	}
	return matchValue, operator
}

func (r *LogRepository) monthsBetween(startTime, endTime int64) []time.Time {
	start := time.Unix(startTime, 0)
	end := time.Unix(endTime, 0)

	cur := time.Date(start.Year(), start.Month(), 1, 0, 0, 0, 0, start.Location())
	endMonth := time.Date(end.Year(), end.Month(), 1, 0, 0, 0, 0, end.Location())

	var months []time.Time
	for !cur.After(endMonth) {
		months = append(months, cur)
		cur = cur.AddDate(0, 1, 0)
	}
	return months
}
