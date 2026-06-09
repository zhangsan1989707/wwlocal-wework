package repository

import (
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"
)

var dailyStatsOpenIDColumns = map[int]string{
	90000031: "login_user_openid",
	90000032: "login_user_openid",
	90000033: "user_openid",
	90000035: "sender_openid",
	90000036: "sender_openid",
	90000037: "sender_openid",
	90000038: "creator_openid",
	90000039: "oper_openid",
	90000040: "oper_openid",
	90000041: "quit_user_openid",
	90000042: "oper_openid",
	90000043: "oper_openid",
	90000044: "oper_openid",
	90000046: "user_openid",
	90000047: "user_openid",
	90000048: "user_openid",
	90000054: "openid",
	90000055: "openid",
	90000058: "openid",
	90000059: "openid",
}

func (r *LogRepository) CreateUserDailyStatsTable() error {
	if err := r.DB.Exec(`
		CREATE TABLE IF NOT EXISTS user_daily_stats (
			mobile VARCHAR(32) NOT NULL,
			feature_id INT NOT NULL,
			stat_date DATE NOT NULL,
			PRIMARY KEY (mobile, feature_id, stat_date)
		) ENGINE=InnoDB
	`).Error; err != nil {
		return err
	}
	// 覆盖索引：加速按 feature_id + stat_date 过滤并分组 mobile 的查询
	var count int64
	r.DB.Raw("SELECT COUNT(*) FROM information_schema.statistics WHERE table_schema = DATABASE() AND table_name = 'user_daily_stats' AND index_name = 'idx_feature_date_mobile'").Scan(&count)
	if count == 0 {
		return r.DB.Exec("CREATE INDEX idx_feature_date_mobile ON user_daily_stats (feature_id, stat_date, mobile)").Error
	}
	return nil
}

func (r *LogRepository) BatchUpsertDailyStats(featureID int, mobiles map[string]bool, logTime int64) {
	if len(mobiles) == 0 {
		return
	}
	statDate := time.Unix(logTime, 0).Format("2006-01-02")

	const batchSize = 2000
	mobileList := make([]string, 0, len(mobiles))
	for m := range mobiles {
		mobileList = append(mobileList, m)
	}

	for i := 0; i < len(mobileList); i += batchSize {
		end := i + batchSize
		if end > len(mobileList) {
			end = len(mobileList)
		}
		batch := mobileList[i:end]

		sql := "INSERT IGNORE INTO user_daily_stats (mobile, feature_id, stat_date) VALUES "
		var args []interface{}
		for j, mobile := range batch {
			if j > 0 {
				sql += ","
			}
			sql += "(?,?,?)"
			args = append(args, mobile, featureID, statDate)
		}
		if err := r.DB.Exec(sql, args...).Error; err != nil {
			slog.Info(fmt.Sprintf("upsert daily stats failed (batch %d-%d): %v", i, end, err))
		}
	}
}

func (r *LogRepository) BatchUpsertDailyStatsWithTx(tx *gorm.DB, featureID int, mobiles map[string]bool, logTime int64) error {
	if len(mobiles) == 0 {
		return nil
	}
	statDate := time.Unix(logTime, 0).Format("2006-01-02")

	const batchSize = 2000
	mobileList := make([]string, 0, len(mobiles))
	for m := range mobiles {
		mobileList = append(mobileList, m)
	}

	for i := 0; i < len(mobileList); i += batchSize {
		end := i + batchSize
		if end > len(mobileList) {
			end = len(mobileList)
		}
		batch := mobileList[i:end]

		sql := "INSERT IGNORE INTO user_daily_stats (mobile, feature_id, stat_date) VALUES "
		var args []interface{}
		for j, mobile := range batch {
			if j > 0 {
				sql += ","
			}
			sql += "(?,?,?)"
			args = append(args, mobile, featureID, statDate)
		}
		if err := tx.Exec(sql, args...).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *LogRepository) BackfillDailyStatsFromLogs(featureIDs []int, statDate string) error {
	t, err := time.Parse("2006-01-02", statDate)
	if err != nil {
		return err
	}
	for _, featureID := range featureIDs {
		openIDColumn := dailyStatsOpenIDColumns[featureID]
		if openIDColumn == "" {
			continue
		}
		tableName := r.GetTableName(featureID, t)
		if !r.TableExists(tableName) {
			continue
		}
		if err := r.backfillDailyStatsForFeature(tableName, openIDColumn, featureID, statDate); err != nil {
			return err
		}
	}
	return nil
}

func (r *LogRepository) backfillDailyStatsForFeature(tableName, openIDColumn string, featureID int, statDate string) error {
	if !safeSQLName(tableName) || !safeSQLName(openIDColumn) {
		return fmt.Errorf("unsafe SQL identifier: %s.%s", tableName, openIDColumn)
	}
	sql := fmt.Sprintf(`
		INSERT IGNORE INTO user_daily_stats (mobile, feature_id, stat_date)
		SELECT DISTINCT %s, ?, ?
		FROM %s
		WHERE DATE(FROM_UNIXTIME(log_time)) = ?
		  AND %s IS NOT NULL
		  AND %s != ''
	`, openIDColumn, tableName, openIDColumn, openIDColumn)
	return r.DB.Exec(sql, featureID, statDate, statDate).Error
}

func safeSQLName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' {
			continue
		}
		return false
	}
	return !strings.HasPrefix(name, "_")
}
