package model

import (
	"encoding/json"
	"strings"
	"time"
)

type User struct {
	ID           uint    `gorm:"primaryKey" json:"id"`
	Username     string  `gorm:"size:32;uniqueIndex;not null" json:"username"`
	PasswordHash string  `gorm:"size:100;not null" json:"-"`
	Phone        *string `gorm:"size:20;uniqueIndex" json:"-"`
	Email        *string `gorm:"size:255;uniqueIndex" json:"-"`
	Nickname     string  `gorm:"size:32" json:"nickname"`
	Signature    string  `gorm:"size:255" json:"signature"`
	School       string  `gorm:"size:64" json:"school"`
	Grade        string  `gorm:"size:32" json:"grade"`
	Avatar       string  `gorm:"size:255" json:"avatar"`
	Role         string  `gorm:"size:16;index;not null;default:user" json:"role"`
	// AdminPermissions 以逗号分隔保存后台权限键，超级管理员不依赖该字段。
	AdminPermissions string     `gorm:"size:255;not null;default:''" json:"-"`
	Status           int        `gorm:"not null;default:1" json:"status"`
	AuthVersion      int        `gorm:"not null;default:0" json:"-"`
	LastLoginAt      *time.Time `json:"last_login_at"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

func (User) TableName() string { return "users" }

// IsAdmin 表示可以进入管理后台；超级管理员始终拥有全部权限。
func (u *User) IsAdmin() bool { return u.Role == RoleAdmin || u.Role == RoleSuperAdmin }

// IsSuperAdmin 表示是超级管理员，可以管理管理员并分配权限。
func (u *User) IsSuperAdmin() bool { return u.Role == RoleSuperAdmin }

// Permissions 返回管理员被分配的权限键，超级管理员返回全部权限。
func (u *User) Permissions() []string {
	if u.IsSuperAdmin() {
		return AllAdminPermissions()
	}
	if u.AdminPermissions == "" {
		return []string{}
	}
	return NormalizeAdminPermissions(strings.Split(u.AdminPermissions, ","))
}

// SetPermissions 规范化并保存权限键，空列表会清空权限。
func (u *User) SetPermissions(keys []string) {
	u.AdminPermissions = strings.Join(NormalizeAdminPermissions(keys), ",")
}

// HasAdminPermission 判断是否拥有指定后台权限，超级管理员恒为真。
func (u *User) HasAdminPermission(key string) bool {
	if u.IsSuperAdmin() {
		return true
	}
	if key == "" || u.AdminPermissions == "" {
		return false
	}
	for _, item := range strings.Split(u.AdminPermissions, ",") {
		if item == key {
			return true
		}
	}
	return false
}

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

// PrivateUser 是仅返回给用户本人的资料视图，包含绑定手机号与后台权限。
type PrivateUser struct {
	PublicUser
	Phone       string   `json:"phone"`
	Permissions []string `json:"permissions"`
}

func NewPrivateUser(u *User) PrivateUser {
	permissions := []string{}
	if u.IsAdmin() {
		permissions = u.Permissions()
	}
	return PrivateUser{PublicUser: NewPublicUser(u), Phone: PhoneString(u.Phone), Permissions: permissions}
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
