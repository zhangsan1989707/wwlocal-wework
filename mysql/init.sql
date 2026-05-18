-- 创建应用专用数据库用户（避免使用 root）
CREATE USER IF NOT EXISTS 'wework_app'@'%' IDENTIFIED BY '${DB_PASSWORD}';
GRANT ALL PRIVILEGES ON wework_logs.* TO 'wework_app'@'%';
FLUSH PRIVILEGES;
