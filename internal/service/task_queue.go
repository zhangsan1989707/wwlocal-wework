package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
)

type TaskQueueService struct {
	redisClient *redis.Client
	cfg         *config.Config
	syncService *SyncService
	contactService *ContactSyncService
	adminLogService *AdminOperLogService
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	workers     int
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
		log.Printf("warning: redis connection failed, task queue disabled: %v", err)
		rdb = nil
	}

	ctx, cancel = context.WithCancel(context.Background())
	return &TaskQueueService{
		redisClient: rdb,
		cfg:         cfg,
		syncService: syncService,
		contactService: contactService,
		adminLogService: adminLogService,
		ctx:         ctx,
		cancel:      cancel,
		workers:     3,
	}, nil
}

func (t *TaskQueueService) IsEnabled() bool {
	return t.redisClient != nil
}

func (t *TaskQueueService) Start() {
	if !t.IsEnabled() {
		log.Println("task queue disabled, starting without worker")
		return
	}
	
	log.Println("starting task queue workers...")
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
		log.Printf("failed to add task to stream: %v", err)
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

	keys, err := t.redisClient.Keys(ctx, "task:*").Result()
	if err != nil {
		return nil, err
	}

	var tasks []*model.SyncTask
	for _, key := range keys {
		if len(tasks) >= limit {
			break
		}
		taskJSON, err := t.redisClient.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		var task model.SyncTask
		if err := json.Unmarshal([]byte(taskJSON), &task); err == nil {
			tasks = append(tasks, &task)
		}
	}

	// 按创建时间倒序
	for i := range tasks {
		for j := i + 1; j < len(tasks); j++ {
			if tasks[i].CreatedAt.Before(tasks[j].CreatedAt) {
				tasks[i], tasks[j] = tasks[j], tasks[i]
			}
		}
	}

	return tasks, nil
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
	taskJSON, _ := json.Marshal(task)
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

func (t *TaskQueueService) updateTaskStatus(taskID string, status model.TaskStatus, progress int, total int, errMsg string, result map[string]interface{}) {
	task, err := t.GetTask(taskID)
	if err != nil {
		log.Printf("failed to get task %s: %v", taskID, err)
		return
	}

	task.Status = status
	task.Progress = progress
	task.Total = total
	task.Error = errMsg
	task.Result = result
	task.UpdatedAt = time.Now()

	if err := t.saveTask(task); err != nil {
		log.Printf("failed to update task %s: %v", taskID, err)
	}
}

func (t *TaskQueueService) worker(id int) {
	defer t.wg.Done()
	log.Printf("worker %d started", id)

	for {
		select {
		case <-t.ctx.Done():
			log.Printf("worker %d stopping", id)
			return
		default:
			// 从 Stream 获取任务
			ctx, cancel := context.WithTimeout(t.ctx, 10*time.Second)
			streams, err := t.redisClient.XRead(ctx, &redis.XReadArgs{
				Streams: []string{t.cfg.Redis.Stream, "0"},
				Block:   2 * time.Second,
				Count:   1,
			}).Result()
			cancel()

			if err != nil && err != redis.Nil {
				log.Printf("worker %d error reading from stream: %v", id, err)
				time.Sleep(1 * time.Second)
				continue
			}

			if len(streams) == 0 {
				continue
			}

			// 处理任务
			for _, stream := range streams {
				for _, msg := range stream.Messages {
					taskJSON := msg.Values["task"].(string)
					var task model.SyncTask
					if err := json.Unmarshal([]byte(taskJSON), &task); err != nil {
						log.Printf("failed to parse task: %v", err)
						continue
					}

					t.processTask(&task, id)

					// 确认处理（删除消息）
					ctx, cancel := context.WithTimeout(t.ctx, 5*time.Second)
					t.redisClient.XDel(ctx, t.cfg.Redis.Stream, msg.ID)
					cancel()
				}
			}
		}
	}
}

func (t *TaskQueueService) processTask(task *model.SyncTask, workerID int) {
	log.Printf("worker %d processing task %s", workerID, task.ID)

	// 更新状态为 running
	t.updateTaskStatus(task.ID, model.TaskStatusRunning, 0, 0, "", nil)

	var result map[string]interface{}
	var err error

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
		log.Printf("task %s failed: %v", task.ID, err)
		t.updateTaskStatus(task.ID, model.TaskStatusFailed, 0, 0, err.Error(), nil)
	} else {
		t.updateTaskStatus(task.ID, model.TaskStatusCompleted, 100, 100, "", result)
		log.Printf("task %s completed", task.ID)
	}
}
