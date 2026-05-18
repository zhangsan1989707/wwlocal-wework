package repository

import (
	"fmt"
	"log/slog"
	"time"

	"gorm.io/gorm"
)

func (r *LogRepository) CreateUserDailyStatsTable() error {
	return r.DB.Exec(`
		CREATE TABLE IF NOT EXISTS user_daily_stats (
			mobile VARCHAR(32) NOT NULL,
			feature_id INT NOT NULL,
			stat_date DATE NOT NULL,
			PRIMARY KEY (mobile, feature_id, stat_date)
		) ENGINE=InnoDB
	`).Error
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
