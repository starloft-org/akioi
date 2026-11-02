// Package report 负责把评测结果回传给主站。
package report

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"akioi/internal/config"
	"akioi/internal/model"
)

// Reporter 通过与主站共享的同步密钥回传评测结果。
type Reporter struct {
	cfg    *config.Config
	client *http.Client
}

func New(cfg *config.Config) *Reporter {
	return &Reporter{
		cfg:    cfg,
		client: &http.Client{Timeout: 15 * time.Second},
	}
}

// Send 回传一次评测结果；失败时由调用方决定是否重试。
func (r *Reporter) Send(ctx context.Context, res *model.JudgeResult) error {
	body, err := json.Marshal(res)
	if err != nil {
		return err
	}
	endpoint := r.cfg.MainBaseURL + "/api/v1/internal/judgements"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Sync-Secret", r.cfg.Judge.SyncSecret)

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("主站返回 %d", resp.StatusCode)
	}
	return nil
}

// SendProgress 把当前评测进度报给主站，让页面能看到「已测 x / n 个测试点」。
// 进度只是锦上添花，调用方失败时忽略错误即可。
func (r *Reporter) SendProgress(ctx context.Context, p *model.JudgeProgress) error {
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	endpoint := r.cfg.MainBaseURL + "/api/v1/internal/judge-progress"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Sync-Secret", r.cfg.Judge.SyncSecret)
	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("主站返回 %d", resp.StatusCode)
	}
	return nil
}
