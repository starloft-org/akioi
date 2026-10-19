package model

import (
	"encoding/json"
	"time"
)

type User struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Username     string     `gorm:"size:32;uniqueIndex;not null" json:"username"`
	PasswordHash string     `gorm:"size:100;not null" json:"-"`
	Phone        *string    `gorm:"size:20;uniqueIndex" json:"-"`
	Email        *string    `gorm:"size:255;uniqueIndex" json:"-"`
	Nickname     string     `gorm:"size:32" json:"nickname"`
	Signature    string     `gorm:"size:255" json:"signature"`
	School       string     `gorm:"size:64" json:"school"`
	Grade        string     `gorm:"size:32" json:"grade"`
	Avatar       string     `gorm:"size:255" json:"avatar"`
	Role         string     `gorm:"size:16;index;not null;default:user" json:"role"`
	Status       int        `gorm:"not null;default:1" json:"status"`
	AuthVersion  int        `gorm:"not null;default:0" json:"-"`
	LastLoginAt  *time.Time `json:"last_login_at"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

func (User) TableName() string { return "users" }

// IsAdmin 表示拥有后台全部权限。
func (u *User) IsAdmin() bool { return u.Role == RoleAdmin }

// OAuthBinding 记录第三方登录账号与站内用户的绑定关系。
type OAuthBinding struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Provider  string    `gorm:"size:16;uniqueIndex:uk_provider_openid;not null" json:"provider"`
	OpenID    string    `gorm:"size:64;uniqueIndex:uk_provider_openid;not null" json:"open_id"`
	UnionID   string    `gorm:"size:64" json:"union_id"`
	UserID    uint      `gorm:"index;not null" json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (OAuthBinding) TableName() string { return "oauth_bindings" }

// PublicUser 是返回给前台的用户视图，不含密码等敏感字段。
type PublicUser struct {
	ID          uint       `json:"id"`
	Username    string     `json:"username"`
	Nickname    string     `json:"nickname"`
	Signature   string     `json:"signature"`
	School      string     `json:"school"`
	Grade       string     `json:"grade"`
	Avatar      string     `json:"avatar"`
	Role        string     `json:"role"`
	Status      int        `json:"status"`
	LastLoginAt *time.Time `json:"last_login_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

func NewPublicUser(u *User) PublicUser {
	return PublicUser{
		ID:          u.ID,
		Username:    u.Username,
		Nickname:    u.Nickname,
		Signature:   u.Signature,
		School:      u.School,
		Grade:       u.Grade,
		Avatar:      u.Avatar,
		Role:        u.Role,
		Status:      u.Status,
		LastLoginAt: u.LastLoginAt,
		CreatedAt:   u.CreatedAt,
	}
}

// PrivateUser 是仅返回给用户本人的资料视图，包含绑定手机号。
type PrivateUser struct {
	PublicUser
	Phone string `json:"phone"`
}

func NewPrivateUser(u *User) PrivateUser {
	return PrivateUser{PublicUser: NewPublicUser(u), Phone: PhoneString(u.Phone)}
}

// PhonePointer 将空手机号表示为 NULL，确保唯一索引允许多个未绑定手机号的微信用户。
func PhonePointer(phone string) *string {
	if phone == "" {
		return nil
	}
	return &phone
}

func PhoneString(phone *string) string {
	if phone == nil {
		return ""
	}
	return *phone
}

func marshalJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func unmarshalJSON(s string, out any) {
	if s == "" {
		return
	}
	_ = json.Unmarshal([]byte(s), out)
}
