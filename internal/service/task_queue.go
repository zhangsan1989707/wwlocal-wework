package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
)

type TaskQueueService struct {
	redisClient     *redis.Client
	cfg             *config.Config
	syncService     *SyncService
	contactService  *ContactSyncService
	adminLogService *AdminOperLogService
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	workers         int
}

func NewTaskQueueService(cfg *config.Config, syncService *SyncService, contactService *ContactSyncService, adminLogService *AdminOperLogService) (*TaskQueueService, error) {
	// 创建 Redis 客户端
	rdb := redis.NewClient(&redis.Options{
		Addr:     fmt.Sprintf("%s:%d", cfg.Redis.Host, cfg.Redis.Port),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		slog.Info(fmt.Sprintf("warning: redis connection failed, task queue disabled: %v", err))
		rdb = nil
	}

	ctx, cancel = context.WithCancel(context.Background())
	return &TaskQueueService{
		redisClient:     rdb,
		cfg:             cfg,
		syncService:     syncService,
		contactService:  contactService,
		adminLogService: adminLogService,
		ctx:             ctx,
		cancel:          cancel,
		workers:         3,
	}, nil
}

func (t *TaskQueueService) IsEnabled() bool {
	return t.redisClient != nil
}

func (t *TaskQueueService) Start() {
	if !t.IsEnabled() {
		slog.Info("task queue disabled, starting without worker")
		return
	}

	// 创建 Consumer Group（幂等）
	ctx, cancel := context.WithTimeout(t.ctx, 5*time.Second)
	defer cancel()
	t.redisClient.XGroupCreateMkStream(ctx, t.cfg.Redis.Stream, "workers", "0")
	// 忽略 BUSYGROUP 错误（已存在）

	slog.Info("starting task queue workers...")
	for i := 0; i < t.workers; i++ {
		t.wg.Add(1)
		go t.worker(i)
	}
}

func (t *TaskQueueService) Stop() {
	t.cancel()
	t.wg.Wait()
	if t.redisClient != nil {
		t.redisClient.Close()
	}
}

func (t *TaskQueueService) SubmitTask(task *model.SyncTask) (string, error) {
	if !t.IsEnabled() {
		return "", fmt.Errorf("task queue disabled")
	}

	task.ID = fmt.Sprintf("task_%d", time.Now().UnixNano())
	task.Status = model.TaskStatusPending
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()

	// 保存任务状态
	if err := t.saveTask(task); err != nil {
		return "", err
	}

	// 添加到 Stream
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(t.ctx, 5*time.Second)
	defer cancel()

	_, err = t.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: t.cfg.Redis.Stream,
		Values: map[string]interface{}{
			"task": string(taskJSON),
		},
	}).Result()
	if err != nil {
		_ = t.deleteTask(task.ID)
		return "", fmt.Errorf("add task to stream: %w", err)
	}

	return task.ID, nil
}

func (t *TaskQueueService) GetTask(taskID string) (*model.SyncTask, error) {
	if !t.IsEnabled() {
		return nil, fmt.Errorf("task queue disabled")
	}

	ctx, cancel := context.WithTimeout(t.ctx, 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("task:%s", taskID)
	taskJSON, err := t.redisClient.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var task model.SyncTask
	if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (t *TaskQueueService) ListTasks(limit int) ([]*model.SyncTask, error) {
	if !t.IsEnabled() {
		return nil, fmt.Errorf("task queue disabled")
	}

	ctx, cancel := context.WithTimeout(t.ctx, 5*time.Second)
	defer cancel()

	if limit <= 0 {
		limit = 50
	}

	// SCAN 替代 KEYS，避免阻塞 Redis
	var keys []string
	iter := t.redisClient.Scan(ctx, 0, "task:*", 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}

	// Pipeline 批量获取，替代逐个 GET
	pipe := t.redisClient.Pipeline()
	cmds := make(map[string]*redis.StringCmd, len(keys))
	for _, key := range keys {
		cmds[key] = pipe.Get(ctx, key)
	}
	_, _ = pipe.Exec(ctx)

	tasks := make([]*model.SyncTask, 0, len(cmds))
	for _, cmd := range cmds {
		taskJSON, err := cmd.Result()
		if err != nil {
			continue
		}
		var task model.SyncTask
		if err := json.Unmarshal([]byte(taskJSON), &task); err == nil {
			tasks = append(tasks, &task)
		}
	}

	return sortAndLimitTasks(tasks, limit), nil
}

func (t *TaskQueueService) CancelTask(taskID string) error {
	if !t.IsEnabled() {
		return fmt.Errorf("task queue disabled")
	}

	task, err := t.GetTask(taskID)
	if err != nil {
		return err
	}

	if task.Status != model.TaskStatusPending {
		return fmt.Errorf("task can only be cancelled when pending")
	}

	task.Status = model.TaskStatusCancelled
	task.UpdatedAt = time.Now()
	return t.saveTask(task)
}

func (t *TaskQueueService) RetryTask(taskID string) error {
	if !t.IsEnabled() {
		return fmt.Errorf("task queue disabled")
	}

	task, err := t.GetTask(taskID)
	if err != nil {
		return err
	}

	if task.Status != model.TaskStatusFailed {
		return fmt.Errorf("task can only be retried when failed")
	}

	// 重新提交
	task.Status = model.TaskStatusPending
	task.Error = ""
	task.UpdatedAt = time.Now()
	if err := t.saveTask(task); err != nil {
		return err
	}

	// 添加回 Stream
	taskJSON, err := json.Marshal(task)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(t.ctx, 5*time.Second)
	defer cancel()

	_, err = t.redisClient.XAdd(ctx, &redis.XAddArgs{
		Stream: t.cfg.Redis.Stream,
		Values: map[string]interface{}{
			"task": string(taskJSON),
		},
	}).Result()
	return err
}

func sortAndLimitTasks(tasks []*model.SyncTask, limit int) []*model.SyncTask {
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})
	if limit > 0 && len(tasks) > limit {
		return tasks[:limit]
	}
	return tasks
}

func (t *TaskQueueService) saveTask(task *model.SyncTask) error {
	ctx, cancel := context.WithTimeout(t.ctx, 5*time.Second)
	defer cancel()

	taskJSON, err := json.Marshal(task)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("task:%s", task.ID)
	return t.redisClient.Set(ctx, key, taskJSON, 24*time.Hour).Err()
}

func (t *TaskQueueService) deleteTask(taskID string) error {
	ctx, cancel := context.WithTimeout(t.ctx, 5*time.Second)
	defer cancel()

	key := fmt.Sprintf("task:%s", taskID)
	return t.redisClient.Del(ctx, key).Err()
}

func (t *TaskQueueService) updateTaskStatus(taskID string, status model.TaskStatus, progress int, total int, errMsg string, result map[string]interface{}) {
	task, err := t.GetTask(taskID)
	if err != nil {
		slog.Info(fmt.Sprintf("failed to get task %s: %v", taskID, err))
		return
	}

	task.Status = status
	task.Progress = progress
	task.Total = total
	task.Error = errMsg
	task.Result = result
	task.UpdatedAt = time.Now()

	if err := t.saveTask(task); err != nil {
		slog.Info(fmt.Sprintf("failed to update task %s: %v", taskID, err))
	}
}

func (t *TaskQueueService) worker(id int) {
	defer t.wg.Done()
	consumerName := fmt.Sprintf("worker-%d", id)
	slog.Info(fmt.Sprintf("worker %d started", id))

	for {
		select {
		case <-t.ctx.Done():
			slog.Info(fmt.Sprintf("worker %d stopping", id))
			return
		default:
			// 使用 Consumer Group 竞争消费
			ctx, cancel := context.WithTimeout(t.ctx, 10*time.Second)
			streams, err := t.redisClient.XReadGroup(ctx, &redis.XReadGroupArgs{
				Group:    "workers",
				Consumer: consumerName,
				Streams:  []string{t.cfg.Redis.Stream, ">"},
				Block:    2 * time.Second,
				Count:    1,
			}).Result()
			cancel()

			if err != nil && err != redis.Nil {
				slog.Info(fmt.Sprintf("worker %d error reading from stream: %v", id, err))
				time.Sleep(1 * time.Second)
				continue
			}

			if len(streams) == 0 {
				continue
			}

			for _, stream := range streams {
				for _, msg := range stream.Messages {
					taskJSON, ok := taskPayload(msg.Values)
					if !ok {
						slog.Info("task message missing valid task payload")
						ctx, cancel := context.WithTimeout(t.ctx, 5*time.Second)
						t.redisClient.XAck(ctx, t.cfg.Redis.Stream, "workers", msg.ID)
						cancel()
						continue
					}
					var task model.SyncTask
					if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
						slog.Info(fmt.Sprintf("failed to parse task: %v", err))
						// 确认无效消息，避免反复投递
						ctx, cancel := context.WithTimeout(t.ctx, 5*time.Second)
						t.redisClient.XAck(ctx, t.cfg.Redis.Stream, "workers", msg.ID)
						cancel()
						continue
					}

					t.processTask(&task, id)

					// 确认消息处理完成
					ctx, cancel := context.WithTimeout(t.ctx, 5*time.Second)
					t.redisClient.XAck(ctx, t.cfg.Redis.Stream, "workers", msg.ID)
					cancel()
				}
			}
		}
	}
}

func taskPayload(values map[string]interface{}) (string, bool) {
	value, ok := values["task"]
	if !ok {
		return "", false
	}
	switch v := value.(type) {
	case string:
		return v, v != ""
	case []byte:
		return string(v), len(v) > 0
	default:
		return "", false
	}
}

func (t *TaskQueueService) processTask(task *model.SyncTask, workerID int) {
	slog.Info(fmt.Sprintf("worker %d processing task %s", workerID, task.ID))

	current, err := t.GetTask(task.ID)
	if err != nil {
		slog.Info(fmt.Sprintf("task %s skipped because state is missing: %v", task.ID, err))
		return
	}
	if current.Status == model.TaskStatusCancelled {
		slog.Info(fmt.Sprintf("task %s skipped because it was cancelled", task.ID))
		return
	}
	if current.Status != model.TaskStatusPending {
		slog.Info(fmt.Sprintf("task %s skipped because status is %s", task.ID, current.Status))
		return
	}

	// 更新状态为 running
	t.updateTaskStatus(task.ID, model.TaskStatusRunning, 0, 0, "", nil)

	var result map[string]interface{}

	switch task.Type {
	case model.TaskTypeLogSync:
		result, err = t.syncService.SyncFeaturesTask(task)
	case model.TaskTypeContactSync:
		result, err = t.contactService.SyncContactsTask(task)
	case model.TaskTypeAdminLogSync:
		result, err = t.adminLogService.SyncAdminLogsTask(task)
	default:
		err = fmt.Errorf("unknown task type: %s", task.Type)
	}

	if err != nil {
		slog.Info(fmt.Sprintf("task %s failed: %v", task.ID, err))
		t.updateTaskStatus(task.ID, model.TaskStatusFailed, 0, 0, err.Error(), nil)
	} else {
		t.updateTaskStatus(task.ID, model.TaskStatusCompleted, 100, 100, "", result)
		slog.Info(fmt.Sprintf("task %s completed", task.ID))
	}
}
