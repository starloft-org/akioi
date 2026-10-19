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
	Email    *service.EmailService
	Queue    *queue.Queue
	Rank     *service.RankService
	Settings *service.SettingService
	Testdata *service.TestdataService
	Judge    *service.JudgeService
}

func New(db *gorm.DB, cfg *config.Config, rdb *redis.Client, settings *service.SettingService) *Handler {
	return &Handler{
		DB:       db,
		Cfg:      cfg,
		RDB:      rdb,
		Auth:     service.NewAuth(db, rdb, cfg, settings),
		WeChat:   service.NewWeChat(db, rdb, cfg, settings),
		SMS:      service.NewSMS(rdb, cfg, settings),
		Email:    service.NewEmail(rdb, cfg, settings),
		Queue:    queue.New(rdb, cfg.Judge.QueueKey),
		Rank:     service.NewRank(db),
		Settings: settings,
		Testdata: service.NewTestdata(db, cfg, settings),
		Judge:    service.NewJudge(db, cfg, rdb, settings),
	}
}
