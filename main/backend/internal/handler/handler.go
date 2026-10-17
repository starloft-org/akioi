package handler

import (
	"akioi/internal/config"
	"akioi/internal/queue"
	"akioi/internal/service"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type Handler struct {
	DB       *gorm.DB
	Cfg      *config.Config
	RDB      *redis.Client
	Auth     *service.AuthService
	WeChat   *service.WeChatService
	SMS      *service.SMSService
	Queue    *queue.Queue
	Rank     *service.RankService
	Testdata *service.TestdataService
	Judge    *service.JudgeService
}

func New(db *gorm.DB, cfg *config.Config, rdb *redis.Client) *Handler {
	return &Handler{
		DB:       db,
		Cfg:      cfg,
		RDB:      rdb,
		Auth:     service.NewAuth(db, rdb, cfg),
		WeChat:   service.NewWeChat(db, rdb, cfg),
		SMS:      service.NewSMS(rdb, cfg),
		Queue:    queue.New(rdb, cfg.Judge.QueueKey),
		Rank:     service.NewRank(db),
		Testdata: service.NewTestdata(db, cfg),
		Judge:    service.NewJudge(db, cfg, rdb),
	}
}
