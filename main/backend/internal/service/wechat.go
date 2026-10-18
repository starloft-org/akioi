package service

import (
	"context"
	"encoding/json"
	"errors"
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
	// 公众号网页授权，必须在微信内置浏览器打开；PC 端靠二维码引导手机扫码。
	wechatAuthorizeURL = "https://open.weixin.qq.com/connect/oauth2/authorize"
	wechatTokenURL     = "https://api.weixin.qq.com/sns/oauth2/access_token"
	wechatUserInfoURL  = "https://api.weixin.qq.com/sns/userinfo"
	wechatProvider     = "wechat"
	wechatStateTTL     = 5 * time.Minute
)

var (
	// ErrWechatBoundToOther 表示该微信已被其他站内账号占用。
	ErrWechatBoundToOther = errors.New("该微信已绑定其他账号")
	// ErrWechatAlreadyBound 表示当前账号已经绑定过微信。
	ErrWechatAlreadyBound = errors.New("当前账号已绑定微信，请先解除绑定")
	// ErrWechatNotBound 表示当前账号没有微信绑定。
	ErrWechatNotBound = errors.New("当前账号未绑定微信")
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

// WeChatState 是随 state 一起保存在 Redis 里的一次性上下文。
type WeChatState struct {
	Purpose  string `json:"purpose"`  // login：扫码登录；bind：绑定到已登录账号
	UserID   uint   `json:"user_id"`  // bind 时对应的站内用户
	Redirect string `json:"redirect"` // 前端落点
	From     string `json:"from"`     // pc：二维码扫码；h5：微信内直接授权
}

// SaveState 记录一次性 state，回调时校验。
func (s *WeChatService) SaveState(ctx context.Context, state string, st WeChatState) error {
	payload, err := json.Marshal(st)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, redis.KeyWeChatStatePrefix+state, payload, wechatStateTTL).Err()
}

// ConsumeState 校验并删除 state，返回发起时的上下文。
func (s *WeChatService) ConsumeState(ctx context.Context, state string) (WeChatState, bool) {
	var st WeChatState
	if state == "" {
		return st, false
	}
	key := redis.KeyWeChatStatePrefix + state
	raw, err := s.rdb.Get(ctx, key).Result()
	if err != nil {
		return st, false
	}
	s.rdb.Del(ctx, key)
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		return st, false
	}
	return st, true
}

func (s *WeChatService) AuthorizeURL(state string) string {
	q := url.Values{}
	q.Set("appid", s.cfg.WeChat.AppID)
	q.Set("redirect_uri", s.settings.Current().WeChatRedirectURI)
	q.Set("response_type", "code")
	// 公众号网页授权：snsapi_userinfo 能拿到昵称头像，首次会弹确认页。
	q.Set("scope", "snsapi_userinfo")
	q.Set("state", state)
	return wechatAuthorizeURL + "?" + q.Encode() + "#wechat_redirect"
}

// ScanResult 暂存扫码授权的结果，供 PC 端轮询取走。
type ScanResult struct {
	Token    string `json:"token"`
	UserID   uint   `json:"user_id"`
	Bound    bool   `json:"bound"`
	Redirect string `json:"redirect"`
}

// SetScanResult 写入一次扫码结果，等待发起端轮询。
func (s *WeChatService) SetScanResult(ctx context.Context, state string, r ScanResult) error {
	payload, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return s.rdb.Set(ctx, scanResultKey(state), payload, wechatStateTTL).Err()
}

// GetScanResult 读取扫码结果。结果随 state 的 TTL 自然过期，不主动删除，
// 避免 PC 端偶发的请求失败把结果消费掉。
func (s *WeChatService) GetScanResult(ctx context.Context, state string) (*ScanResult, bool) {
	if state == "" {
		return nil, false
	}
	raw, err := s.rdb.Get(ctx, scanResultKey(state)).Result()
	if err != nil {
		return nil, false
	}
	var r ScanResult
	if err := json.Unmarshal([]byte(raw), &r); err != nil {
		return nil, false
	}
	return &r, true
}

func scanResultKey(state string) string {
	return redis.KeyWeChatStatePrefix + "scan:" + state
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

// BindTo 把微信账号绑定到指定站内用户。
func (s *WeChatService) BindTo(userID uint, p *wechatProfile) error {
	var count int64
	if err := s.db.Model(&model.OAuthBinding{}).
		Where("provider = ? AND open_id = ? AND user_id <> ?", wechatProvider, p.OpenID, userID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrWechatBoundToOther
	}
	if err := s.db.Model(&model.OAuthBinding{}).
		Where("provider = ? AND user_id = ?", wechatProvider, userID).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return ErrWechatAlreadyBound
	}
	return s.db.Create(&model.OAuthBinding{
		Provider: wechatProvider,
		OpenID:   p.OpenID,
		UnionID:  p.UnionID,
		UserID:   userID,
	}).Error
}

// Unbind 解除指定用户的微信绑定。
func (s *WeChatService) Unbind(userID uint) error {
	res := s.db.Where("provider = ? AND user_id = ?", wechatProvider, userID).Delete(&model.OAuthBinding{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrWechatNotBound
	}
	return nil
}

// Bound 判断用户是否已绑定微信。
func (s *WeChatService) Bound(userID uint) bool {
	var count int64
	if err := s.db.Model(&model.OAuthBinding{}).
		Where("provider = ? AND user_id = ?", wechatProvider, userID).
		Count(&count).Error; err != nil {
		return false
	}
	return count > 0
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
