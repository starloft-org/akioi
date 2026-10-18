// Package service 承载业务逻辑，handler 不直接拼 SQL。
package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"akioi/internal/config"
	"akioi/internal/model"
	"akioi/internal/redis"

	"github.com/golang-jwt/jwt/v5"
	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrUnauthorized = errors.New("登录状态已失效")
	ErrUserBanned   = errors.New("账号已被禁用")
	ErrBadPassword  = errors.New("用户名或密码错误")
)

type AuthService struct {
	db       *gorm.DB
	rdb      *goredis.Client
	cfg      *config.Config
	settings *SettingService
}

func NewAuth(db *gorm.DB, rdb *goredis.Client, cfg *config.Config, settings *SettingService) *AuthService {
	return &AuthService{db: db, rdb: rdb, cfg: cfg, settings: settings}
}

func (s *AuthService) HashPassword(pw string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *AuthService) CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

type claims struct {
	UserID      uint `json:"uid"`
	AuthVersion int  `json:"av"`
	jwt.RegisteredClaims
}

// Issue 签发 JWT 并在 Redis 中登记会话，登出即失效。
func (s *AuthService) Issue(u *model.User) (string, error) {
	jti, err := randomHex(16)
	if err != nil {
		return "", err
	}
	ttl := time.Duration(s.settings.Current().JWTTTLHours) * time.Hour
	c := claims{
		UserID:      u.ID,
		AuthVersion: u.AuthVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			Subject:   strconv.FormatUint(uint64(u.ID), 10),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(ttl)),
		},
	}
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, c).SignedString([]byte(s.cfg.JWTSecret))
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.rdb.Set(ctx, redis.KeySessionPrefix+jti, u.ID, ttl).Err(); err != nil {
		return "", fmt.Errorf("写入会话失败: %w", err)
	}
	return token, nil
}

// Authenticate 校验令牌与会话，并返回最新的用户记录。
func (s *AuthService) Authenticate(token string) (*model.User, error) {
	if token == "" {
		return nil, ErrUnauthorized
	}
	var c claims
	parsed, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("非预期的签名算法")
		}
		return []byte(s.cfg.JWTSecret), nil
	})
	if err != nil || !parsed.Valid {
		return nil, ErrUnauthorized
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if _, err := s.rdb.Get(ctx, redis.KeySessionPrefix+c.ID).Result(); err != nil {
		return nil, ErrUnauthorized
	}
	var u model.User
	if err := s.db.First(&u, c.UserID).Error; err != nil {
		return nil, ErrUnauthorized
	}
	if u.Status != model.UserStatusActive {
		return nil, ErrUserBanned
	}
	if u.AuthVersion != c.AuthVersion {
		return nil, ErrUnauthorized
	}
	return &u, nil
}

// ResetPassword 更新密码并提升鉴权版本，使该用户已签发的所有 JWT 立即失效。
func (s *AuthService) ResetPassword(userID uint, password string) error {
	hash, err := s.HashPassword(password)
	if err != nil {
		return err
	}
	return s.db.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
		"password_hash": hash,
		"auth_version":  gorm.Expr("auth_version + 1"),
	}).Error
}

// Revoke 使当前令牌对应的会话失效。
func (s *AuthService) Revoke(token string) error {
	var c claims
	if _, err := jwt.ParseWithClaims(token, &c, func(t *jwt.Token) (any, error) {
		return []byte(s.cfg.JWTSecret), nil
	}); err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return s.rdb.Del(ctx, redis.KeySessionPrefix+c.ID).Err()
}

// NewStateToken 生成一次性随机串，用于第三方登录的 state 参数。
func NewStateToken() (string, error) {
	return randomHex(16)
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
