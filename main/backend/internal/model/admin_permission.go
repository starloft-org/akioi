package model

import "strings"

// 管理后台的权限项，与后台菜单一一对应。
// 超级管理员始终拥有全部权限，普通管理员按分配到的权限项访问对应模块。
const (
	AdminPermissionUsers         = "users"
	AdminPermissionProblems      = "problems"
	AdminPermissionProblemBanks  = "problem_banks"
	AdminPermissionContests      = "contests"
	AdminPermissionSubmissions   = "submissions"
	AdminPermissionJudges        = "judges"
	AdminPermissionAnnouncements = "announcements"
	AdminPermissionSolutions     = "solutions"
	AdminPermissionSettings      = "settings"
	AdminPermissionLogs          = "logs"
	AdminPermissionBackups       = "backups"
)

// AdminPermissionOption 描述单个权限项，供后台渲染勾选项。
type AdminPermissionOption struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Group string `json:"group"`
}

// AdminPermissionOptions 按展示顺序返回全部权限项。
func AdminPermissionOptions() []AdminPermissionOption {
	return []AdminPermissionOption{
		{Key: AdminPermissionUsers, Label: "用户管理", Group: "运营"},
		{Key: AdminPermissionAnnouncements, Label: "公告管理", Group: "运营"},
		{Key: AdminPermissionProblems, Label: "题目管理", Group: "内容"},
		{Key: AdminPermissionProblemBanks, Label: "题库管理", Group: "内容"},
		{Key: AdminPermissionContests, Label: "比赛管理", Group: "内容"},
		{Key: AdminPermissionSolutions, Label: "题解审核", Group: "内容"},
		{Key: AdminPermissionSubmissions, Label: "评测记录", Group: "内容"},
		{Key: AdminPermissionJudges, Label: "测评机管理", Group: "内容"},
		{Key: AdminPermissionSettings, Label: "系统设置", Group: "系统"},
		{Key: AdminPermissionBackups, Label: "数据备份", Group: "系统"},
		{Key: AdminPermissionLogs, Label: "日志查看", Group: "系统"},
	}
}

// AllAdminPermissions 返回全部权限项的键名，顺序与 AdminPermissionOptions 一致。
func AllAdminPermissions() []string {
	options := AdminPermissionOptions()
	keys := make([]string, 0, len(options))
	for _, option := range options {
		keys = append(keys, option.Key)
	}
	return keys
}

// ValidAdminPermission 判断键名是否为已知权限项。
func ValidAdminPermission(key string) bool {
	for _, option := range AdminPermissionOptions() {
		if option.Key == key {
			return true
		}
	}
	return false
}

// NormalizeAdminPermissions 去除空白与重复项，丢弃未知键，并按固定顺序返回。
func NormalizeAdminPermissions(keys []string) []string {
	selected := make(map[string]bool, len(keys))
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if ValidAdminPermission(key) {
			selected[key] = true
		}
	}
	result := make([]string, 0, len(selected))
	for _, key := range AllAdminPermissions() {
		if selected[key] {
			result = append(result, key)
		}
	}
	return result
}

// UnknownAdminPermissions 返回列表中无法识别的权限键，用于接口提示。
func UnknownAdminPermissions(keys []string) []string {
	unknown := make([]string, 0)
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key != "" && !ValidAdminPermission(key) {
			unknown = append(unknown, key)
		}
	}
	return unknown
}
