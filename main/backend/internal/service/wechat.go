package service

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"akioi/internal/config"
	"akioi/internal/model"
	"akioi/internal/redis"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const (
	wechatAuthorizeURL = "https://open.weixin.qq.com/connect/qrconnect"
	wechatTokenURL     = "https://api.weixin.qq.com/sns/oauth2/access_token"
	wechatUserInfoURL  = "https://api.weixin.qq.com/sns/userinfo"
	wechatProvider     = "wechat"
	wechatStateTTL     = 5 * time.Minute
)

// WeChatService 实现微信扫码登录：授权跳转、换取 openid、绑定或创建站内用户。
type WeChatService struct {
	cfg      *config.Config
	db       *gorm.DB
	rdb      *goredis.Client
	settings *SettingService
}

func NewWeChat(db *gorm.DB, rdb *goredis.Client, cfg *config.Config, settings *SettingService) *WeChatService {
	return &WeChatService{cfg: cfg, db: db, rdb: rdb, settings: settings}
}

// Enabled 表示登录方式是否可用，需开关打开且凭据齐全。
func (s *WeChatService) Enabled() bool {
	return s.settings.Current().WeChatEnabled && s.cfg.WeChat.AppID != "" && s.cfg.WeChat.AppSecret != ""
}

// SaveState 记录一次性 state，回调时校验。
func (s *WeChatService) SaveState(ctx context.Context, state, redirect string) error {
	return s.rdb.Set(ctx, redis.KeyWeChatStatePrefix+state, redirect, wechatStateTTL).Err()
}

// ConsumeState 校验并删除 state，返回发起登录时携带的跳转地址。
func (s *WeChatService) ConsumeState(ctx context.Context, state string) (string, bool) {
	if state == "" {
		return "", false
	}
	key := redis.KeyWeChatStatePrefix + state
	redirect, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		return "", false
	}
	s.rdb.Del(ctx, key)
	return redirect, true
}

func (s *WeChatService) AuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("appid", s.cfg.WeChat.AppID)
	q.Set("redirect_uri", s.settings.Current().WeChatRedirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "snsapi_login")
	q.Set("state", state)
	return wechatAuthorizeURL + "?" + q.Encode() + "#wechat_redirect"
}

type wechatProfile struct {
	OpenID   string
	UnionID  string
	Nickname string
	Avatar   string
}

// Exchange 用授权码换取用户资料。
func (s *WeChatService) Exchange(ctx context.Context, code string) (*wechatProfile, error) {
	tokenQuery := url.Values{}
	tokenQuery.Set("appid", s.cfg.WeChat.AppID)
	tokenQuery.Set("secret", s.cfg.WeChat.AppSecret)
	tokenQuery.Set("code", code)
	tokenQuery.Set("grant_type", "authorization_code")

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		OpenID      string `json:"openid"`
		UnionID     string `json:"unionid"`
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := s.getJSON(ctx, wechatTokenURL+"?"+tokenQuery.Encode(), &tokenResp); err != nil {
		return nil, err
	}
	if tokenResp.ErrCode != 0 || tokenResp.OpenID == "" {
		return nil, fmt.Errorf("微信授权失败: %s", tokenResp.ErrMsg)
	}

	infoQuery := url.Values{}
	infoQuery.Set("access_token", tokenResp.AccessToken)
	infoQuery.Set("openid", tokenResp.OpenID)
	infoQuery.Set("lang", "zh_CN")

	var infoResp struct {
		Nickname string `json:"nickname"`
		HeadImg  string `json:"headimgurl"`
		UnionID  string `json:"unionid"`
		ErrCode  int    `json:"errcode"`
		ErrMsg   string `json:"errmsg"`
	}
	if err := s.getJSON(ctx, wechatUserInfoURL+"?"+infoQuery.Encode(), &infoResp); err != nil {
		return nil, err
	}

	profile := &wechatProfile{
		OpenID:   tokenResp.OpenID,
		UnionID:  tokenResp.UnionID,
		Nickname: infoResp.Nickname,
		Avatar:   infoResp.HeadImg,
	}
	if profile.UnionID == "" {
		profile.UnionID = infoResp.UnionID
	}
	if profile.Nickname == "" {
		profile.Nickname = "微信用户"
	}
	return profile, nil
}

// BindOrCreate 按 openid 查找绑定关系，不存在时创建站内账号并绑定。
func (s *WeChatService) BindOrCreate(p *wechatProfile) (*model.User, error) {
	var binding model.OAuthBinding
	err := s.db.Where("provider = ? AND open_id = ?", wechatProvider, p.OpenID).First(&binding).Error
	if err == nil {
		var u model.User
		if err := s.db.First(&u, binding.UserID).Error; err != nil {
			return nil, fmt.Errorf("绑定用户不存在")
		}
		return &u, nil
	}
	if err != nil && err != gorm.ErrRecordNotFound {
		return nil, err
	}

	suffix, err := randomHex(4)
	if err != nil {
		return nil, err
	}
	password, err := randomHex(16)
	if err != nil {
		return nil, err
	}
	hash, err := NewAuth(s.db, s.rdb, s.cfg, s.settings).HashPassword(password)
	if err != nil {
		return nil, err
	}
	u := model.User{
		Username:     "wx_" + suffix,
		PasswordHash: hash,
		Nickname:     p.Nickname,
		Avatar:       p.Avatar,
		Role:         model.RoleUser,
		Status:       model.UserStatusActive,
	}
	var created *model.User
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&u).Error; err != nil {
			return err
		}
		b := model.OAuthBinding{
			Provider: wechatProvider,
			OpenID:   p.OpenID,
			UnionID:  p.UnionID,
			UserID:   u.ID,
		}
		if err := tx.Create(&b).Error; err != nil {
			return err
		}
		created = &u
		return nil
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (s *WeChatService) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求微信接口失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("解析微信接口响应失败: %w", err)
	}
	return nil
}
