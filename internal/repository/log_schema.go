package repository

import (
	"encoding/json"
	"fmt"
)

type logColumn struct {
	Name  string
	SQL   string
	Index bool
}

type logFeatureSchema struct {
	Columns []logColumn
	Mapper  func(map[string]interface{}) map[string]interface{}
}

var logSchemas = map[int]logFeatureSchema{
	90000031: {Columns: []logColumn{
		varcharIndex("login_user_openid"), intCol("login_user_type"), varcharCol("deviceid"), intCol("devtype"),
		bigintCol("login_time"), intCol("login_type"), varcharCol("cli_ver"), varcharCol("access_ip"), varcharCol("cli_ip"),
	}, Mapper: mapLoginLog},
	90000032: {Columns: []logColumn{
		varcharIndex("login_user_openid"), intCol("login_user_type"), varcharCol("deviceid"), bigintCol("login_time"),
		varcharCol("cli_ver"), intCol("devtype"), varcharCol("cli_ip"),
	}, Mapper: mapWakeupLog},
	90000033: {Columns: []logColumn{
		varcharIndex("user_openid"), intCol("user_type"), bigintCol("access_time"), bigintCol("agentid"),
		varcharCol("name"), varcharCol("deviceid"), intCol("devtype"), intCol("landing_type"),
	}, Mapper: mapAccessAppLog},
	90000034: {Columns: []logColumn{
		bigintCol("agentid"), varcharCol("name"), bigintCol("push_time"), intCol("push_type"),
		textCol("content"), textCol("to_user"), textCol("to_party"), intCol("source"),
	}, Mapper: mapPushMessageLog},
	90000035: {Columns: []logColumn{
		intCol("chat_type"), intCol("msg_type"), intCol("devtype"), varcharIndex("sender_openid"),
		intCol("sender_type"), bigintCol("send_time"), varcharCol("deviceid"),
	}, Mapper: mapChatSendLog},
	90000036: {Columns: []logColumn{
		varcharIndex("sender_openid"), intCol("sender_type"), bigintCol("send_time"), varcharIndex("receiver_openid"),
		intCol("receiver_type"), intCol("msg_type"), varcharCol("msgid"), varcharCol("appinfo"),
	}, Mapper: mapSingleChatLog},
	90000037: {Columns: []logColumn{
		varcharIndex("sender_openid"), intCol("sender_type"), bigintCol("send_time"), textCol("receiver"),
		varcharCol("chatid"), intCol("msg_type"), varcharCol("msgid"), varcharCol("appinfo"), varcharCol("name"),
	}, Mapper: mapGroupChatLog},
	90000038: {Columns: []logColumn{
		varcharIndex("creator_openid"), intCol("creator_type"), bigintCol("create_time"),
		varcharCol("chatid"), varcharCol("name"), textCol("members"),
	}, Mapper: mapCreateGroupLog},
	90000039: {Columns: []logColumn{
		varcharIndex("oper_openid"), intCol("oper_type"), bigintCol("operate_time"), textCol("add_members"),
		varcharCol("chatid"), varcharCol("name"), textCol("original_members"),
	}, Mapper: mapAddGroupMemberLog},
	90000040: {Columns: []logColumn{
		varcharIndex("oper_openid"), intCol("oper_type"), bigintCol("operate_time"), textCol("del_members"),
		varcharCol("chatid"), varcharCol("name"), textCol("original_members"),
	}, Mapper: mapRemoveGroupMemberLog},
	90000041: {Columns: []logColumn{
		varcharIndex("quit_user_openid"), intCol("quit_user_type"), bigintCol("quit_time"),
		varcharCol("chatid"), varcharCol("name"), textCol("original_members"),
	}, Mapper: mapQuitGroupLog},
	90000042: {Columns: []logColumn{
		varcharIndex("oper_openid"), intCol("oper_type"), bigintCol("operate_time"),
		varcharIndex("new_owner_openid"), intCol("new_owner_type"), varcharCol("chatid"), varcharCol("name"), textCol("members"),
	}, Mapper: mapTransferGroupOwnerLog},
	90000043: {Columns: []logColumn{
		varcharIndex("oper_openid"), intCol("oper_type"), bigintCol("operate_time"),
		varcharCol("chatid"), varcharCol("name"), textCol("members"),
	}, Mapper: mapDisbandGroupLog},
	90000044: {Columns: []logColumn{
		varcharIndex("oper_openid"), intCol("oper_type"), bigintCol("operate_time"),
		varcharCol("chatid"), varcharCol("old_name"), varcharCol("new_name"),
	}, Mapper: mapRenameGroupLog},
	90000046: {Columns: []logColumn{
		intCol("send_target"), varcharIndex("user_openid"), intCol("user_type"), bigintCol("agentid"),
		varcharCol("name"), varcharIndex("agent_user_openid"), intCol("agent_user_type"), bigintCol("send_time"),
		intCol("msg_type"), textCol("content"),
	}, Mapper: mapAppChatLog},
	90000047: {Columns: []logColumn{
		varcharIndex("user_openid"), intCol("user_type"), bigintCol("operate_time"),
	}, Mapper: mapAddLog},
	90000048: {Columns: []logColumn{
		varcharIndex("user_openid"), intCol("user_type"), bigintCol("operate_time"),
	}, Mapper: mapActivateLog},
	90000054: {Columns: []logColumn{
		varcharIndex("openid"), varcharCol("deviceid"), intCol("devtype"), varcharCol("cli_ver"), bigintCol("timestamp"),
	}, Mapper: mapClientInstallLog},
	90000055: {Columns: []logColumn{
		varcharIndex("openid"), varcharCol("deviceid"), intCol("devtype"), bigintCol("timestamp"), varcharCol("old_ver"), varcharCol("new_ver"),
	}, Mapper: mapClientUpdateLog},
	90000058: {Columns: []logColumn{
		varcharIndex("openid"), varcharCol("deviceid"), intCol("devtype"), varcharCol("cli_ver"), varcharCol("contact_old_ver"),
		varcharCol("contact_new_ver"), intCol("update_sence"), intCol("update_type"), bigintCol("update_start_time"),
		bigintCol("update_end_time"), bigintCol("update_data_flow"), intCol("update_result"), varcharCol("update_failed_reason"),
		bigintCol("update_cost"), textCol("net_status"), textCol("device_status"),
	}, Mapper: mapContactUpdateLog},
	90000059: {Columns: []logColumn{
		varcharIndex("openid"), varcharCol("deviceid"), intCol("devtype"), varcharCol("cli_ver"), varcharCol("time_date"),
		bigintCol("req_count"), bigintCol("req_failed_count"), bigintCol("req_avg_cost"), bigintCol("req_cost_cleaning"),
		varcharCol("abnormal_cost_percentage"), bigintCol("req_cost_max"),
	}, Mapper: mapNetworkStatsLog},
	90000061: {Columns: weDriveOperColumns(), Mapper: mapWeDriveOperLog},
	90000062: {Columns: weDriveOperColumns(), Mapper: mapWeDriveOperLog},
	90000063: {Columns: weDriveOperColumns(), Mapper: mapWeDriveOperLog},
}

func varcharCol(name string) logColumn {
	return logColumn{Name: name, SQL: fmt.Sprintf("%s VARCHAR(255)", name)}
}

func varcharIndex(name string) logColumn {
	return logColumn{Name: name, SQL: fmt.Sprintf("%s VARCHAR(255)", name), Index: true}
}

func intCol(name string) logColumn {
	return logColumn{Name: name, SQL: fmt.Sprintf("%s INT", name)}
}

func bigintCol(name string) logColumn {
	return logColumn{Name: name, SQL: fmt.Sprintf("%s BIGINT", name)}
}

func textCol(name string) logColumn {
	return logColumn{Name: name, SQL: fmt.Sprintf("%s TEXT", name)}
}

func weDriveOperColumns() []logColumn {
	return []logColumn{
		bigintCol("operate_time"), intCol("oper_type"), intCol("oper_sub_type"), varcharIndex("oper_name"), textCol("ext"),
	}
}

func mapStructuredFields(featureID int, parsedJSON string) map[string]interface{} {
	schema, ok := logSchemas[featureID]
	if !ok || schema.Mapper == nil || parsedJSON == "" {
		return nil
	}
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(parsedJSON), &data); err != nil {
		return nil
	}
	return schema.Mapper(data)
}

func mapLoginLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"login_user_openid": nestedString(data, "login_user", "openid"),
		"login_user_type":   nestedInt(data, "login_user", "type"),
		"deviceid":          stringValue(data, "deviceid"),
		"devtype":           intValue(data, "devtype"),
		"login_time":        int64Value(data, "login_time"),
		"login_type":        intValue(data, "login_type"),
		"cli_ver":           stringValue(data, "cli_ver"),
		"access_ip":         stringValue(data, "access_ip"),
		"cli_ip":            stringValue(data, "cli_ip"),
	})
}

func mapWakeupLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"login_user_openid": nestedString(data, "login_user", "openid"),
		"login_user_type":   nestedInt(data, "login_user", "type"),
		"deviceid":          stringValue(data, "deviceid"),
		"login_time":        int64Value(data, "login_time"),
		"cli_ver":           stringValue(data, "cli_ver"),
		"devtype":           intValue(data, "devtype"),
		"cli_ip":            stringValue(data, "cli_ip"),
	})
}

func mapAccessAppLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"user_openid":  nestedString(data, "user", "openid"),
		"user_type":    nestedInt(data, "user", "type"),
		"access_time":  int64Value(data, "access_time"),
		"agentid":      int64Value(data, "agentid"),
		"name":         stringValue(data, "name"),
		"deviceid":     stringValue(data, "deviceid"),
		"devtype":      intValue(data, "devtype"),
		"landing_type": intValue(data, "landing_type"),
	})
}

func mapPushMessageLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"agentid":   int64Value(data, "agentid"),
		"name":      stringValue(data, "name"),
		"push_time": int64Value(data, "push_time"),
		"push_type": intValue(data, "push_type"),
		"content":   jsonString(data, "content"),
		"to_user":   jsonString(data, "to_user"),
		"to_party":  jsonString(data, "to_party"),
		"source":    intValue(data, "source"),
	})
}

func mapChatSendLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"chat_type":     intValue(data, "chat_type"),
		"msg_type":      intValue(data, "msg_type"),
		"devtype":       intValue(data, "devtype"),
		"sender_openid": nestedString(data, "sender", "openid"),
		"sender_type":   nestedInt(data, "sender", "type"),
		"send_time":     int64Value(data, "send_time"),
		"deviceid":      stringValue(data, "deviceid"),
	})
}

func mapSingleChatLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"sender_openid":   nestedString(data, "sender", "openid"),
		"sender_type":     nestedInt(data, "sender", "type"),
		"send_time":       int64Value(data, "send_time"),
		"receiver_openid": nestedString(data, "receiver", "openid"),
		"receiver_type":   nestedInt(data, "receiver", "type"),
		"msg_type":        intValue(data, "msg_type"),
		"msgid":           firstString(data, "msgid", "msg_id"),
		"appinfo":         stringValue(data, "appinfo"),
	})
}

func mapGroupChatLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"sender_openid": nestedString(data, "sender", "openid"),
		"sender_type":   nestedInt(data, "sender", "type"),
		"send_time":     int64Value(data, "send_time"),
		"receiver":      jsonString(data, "receiver"),
		"chatid":        stringValue(data, "chatid"),
		"msg_type":      intValue(data, "msg_type"),
		"msgid":         firstString(data, "msgid", "msg_id"),
		"appinfo":       stringValue(data, "appinfo"),
		"name":          stringValue(data, "name"),
	})
}

func mapCreateGroupLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"creator_openid": nestedString(data, "creator", "openid"),
		"creator_type":   nestedInt(data, "creator", "type"),
		"create_time":    int64Value(data, "create_time"),
		"chatid":         stringValue(data, "chatid"),
		"name":           stringValue(data, "name"),
		"members":        jsonString(data, "members"),
	})
}

func mapAddGroupMemberLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"oper_openid":      nestedString(data, "oper", "openid"),
		"oper_type":        nestedInt(data, "oper", "type"),
		"operate_time":     int64Value(data, "operate_time"),
		"add_members":      jsonString(data, "add_members"),
		"chatid":           stringValue(data, "chatid"),
		"name":             stringValue(data, "name"),
		"original_members": jsonString(data, "original_members"),
	})
}

func mapRemoveGroupMemberLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"oper_openid":      nestedString(data, "oper", "openid"),
		"oper_type":        nestedInt(data, "oper", "type"),
		"operate_time":     int64Value(data, "operate_time"),
		"del_members":      jsonString(data, "del_members"),
		"chatid":           stringValue(data, "chatid"),
		"name":             stringValue(data, "name"),
		"original_members": jsonString(data, "original_members"),
	})
}

func mapQuitGroupLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"quit_user_openid": nestedString(data, "quit_user", "openid"),
		"quit_user_type":   nestedInt(data, "quit_user", "type"),
		"quit_time":        int64Value(data, "quit_time"),
		"chatid":           stringValue(data, "chatid"),
		"name":             stringValue(data, "name"),
		"original_members": jsonString(data, "original_members"),
	})
}

func mapTransferGroupOwnerLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"oper_openid":      nestedString(data, "oper", "openid"),
		"oper_type":        nestedInt(data, "oper", "type"),
		"operate_time":     int64Value(data, "operate_time"),
		"new_owner_openid": nestedString(data, "new_owner", "openid"),
		"new_owner_type":   nestedInt(data, "new_owner", "type"),
		"chatid":           stringValue(data, "chatid"),
		"name":             stringValue(data, "name"),
		"members":          jsonString(data, "members"),
	})
}

func mapDisbandGroupLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"oper_openid":  nestedString(data, "oper", "openid"),
		"oper_type":    nestedInt(data, "oper", "type"),
		"operate_time": int64Value(data, "operate_time"),
		"chatid":       stringValue(data, "chatid"),
		"name":         stringValue(data, "name"),
		"members":      jsonString(data, "members"),
	})
}

func mapRenameGroupLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"oper_openid":  nestedString(data, "oper", "openid"),
		"oper_type":    nestedInt(data, "oper", "type"),
		"operate_time": int64Value(data, "operate_time"),
		"chatid":       stringValue(data, "chatid"),
		"old_name":     stringValue(data, "old_name"),
		"new_name":     stringValue(data, "new_name"),
	})
}

func mapAppChatLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"send_target":       intValue(data, "send_target"),
		"user_openid":       nestedString(data, "user", "openid"),
		"user_type":         nestedInt(data, "user", "type"),
		"agentid":           int64Value(data, "agentid"),
		"name":              stringValue(data, "name"),
		"agent_user_openid": nestedString(data, "agent_user", "openid"),
		"agent_user_type":   nestedInt(data, "agent_user", "type"),
		"send_time":         int64Value(data, "send_time"),
		"msg_type":          intValue(data, "msg_type"),
		"content":           jsonString(data, "content"),
	})
}

func mapAddLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"user_openid":  nestedString(data, "user", "openid"),
		"user_type":    nestedInt(data, "user", "type"),
		"operate_time": int64Value(data, "operate_time"),
	})
}

func mapActivateLog(data map[string]interface{}) map[string]interface{} {
	return mapAddLog(data)
}

func mapClientInstallLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"openid":    stringValue(data, "openid"),
		"deviceid":  stringValue(data, "deviceid"),
		"devtype":   intValue(data, "devtype"),
		"cli_ver":   stringValue(data, "cli_ver"),
		"timestamp": int64Value(data, "timestamp"),
	})
}

func mapClientUpdateLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"openid":    stringValue(data, "openid"),
		"deviceid":  stringValue(data, "deviceid"),
		"devtype":   intValue(data, "devtype"),
		"timestamp": int64Value(data, "timestamp"),
		"old_ver":   stringValue(data, "old_ver"),
		"new_ver":   stringValue(data, "new_ver"),
	})
}

func mapContactUpdateLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"openid":               stringValue(data, "openid"),
		"deviceid":             stringValue(data, "deviceid"),
		"devtype":              intValue(data, "devtype"),
		"cli_ver":              stringValue(data, "cli_ver"),
		"contact_old_ver":      stringValue(data, "contact_old_ver"),
		"contact_new_ver":      stringValue(data, "contact_new_ver"),
		"update_sence":         intValue(data, "update_sence"),
		"update_type":          intValue(data, "update_type"),
		"update_start_time":    int64Value(data, "update_start_time"),
		"update_end_time":      int64Value(data, "update_end_time"),
		"update_data_flow":     int64Value(data, "update_data_flow"),
		"update_result":        intValue(data, "update_result"),
		"update_failed_reason": stringValue(data, "update_failed_reason"),
		"update_cost":          int64Value(data, "update_cost"),
		"net_status":           jsonString(data, "net_status"),
		"device_status":        jsonString(data, "device_status"),
	})
}

func mapNetworkStatsLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"openid":                   stringValue(data, "openid"),
		"deviceid":                 stringValue(data, "deviceid"),
		"devtype":                  intValue(data, "devtype"),
		"cli_ver":                  stringValue(data, "cli_ver"),
		"time_date":                stringValue(data, "time_date"),
		"req_count":                int64Value(data, "req_count"),
		"req_failed_count":         int64Value(data, "req_failed_count"),
		"req_avg_cost":             int64Value(data, "req_avg_cost"),
		"req_cost_cleaning":        int64Value(data, "req_cost_cleaning"),
		"abnormal_cost_percentage": stringValue(data, "abnormal_cost_percentage"),
		"req_cost_max":             int64Value(data, "req_cost_max"),
	})
}

func mapWeDriveOperLog(data map[string]interface{}) map[string]interface{} {
	return compactMap(map[string]interface{}{
		"operate_time":  int64Value(data, "time"),
		"oper_type":     intValue(data, "oper_type"),
		"oper_sub_type": intValue(data, "oper_sub_type"),
		"oper_name":     stringValue(data, "oper_name"),
		"ext":           jsonString(data, "ext"),
	})
}

func compactMap(values map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(values))
	for key, value := range values {
		if value == nil {
			continue
		}
		if text, ok := value.(string); ok && text == "" {
			continue
		}
		result[key] = value
	}
	return result
}

func stringValue(data map[string]interface{}, key string) string {
	if v, ok := data[key].(string); ok {
		return v
	}
	return ""
}

func firstString(data map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v := stringValue(data, key); v != "" {
			return v
		}
	}
	return ""
}

func intValue(data map[string]interface{}, key string) interface{} {
	switch v := data[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case json.Number:
		i, _ := v.Int64()
		return int(i)
	}
	return nil
}

func int64Value(data map[string]interface{}, key string) interface{} {
	switch v := data[key].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case json.Number:
		i, _ := v.Int64()
		return i
	}
	return nil
}

func nestedMap(data map[string]interface{}, key string) map[string]interface{} {
	if v, ok := data[key].(map[string]interface{}); ok {
		return v
	}
	return nil
}

func nestedString(data map[string]interface{}, parent, key string) string {
	if nested := nestedMap(data, parent); nested != nil {
		return stringValue(nested, key)
	}
	return ""
}

func nestedInt(data map[string]interface{}, parent, key string) interface{} {
	if nested := nestedMap(data, parent); nested != nil {
		return intValue(nested, key)
	}
	return nil
}

func jsonString(data map[string]interface{}, key string) string {
	value, ok := data[key]
	if !ok || value == nil {
		return ""
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	return string(bytes)
}
