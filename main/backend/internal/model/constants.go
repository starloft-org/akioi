package model

// 用户角色
// 超级管理员可以管理管理员并分配权限，管理员按权限项访问后台模块。
const (
	RoleSuperAdmin = "superadmin"
	RoleAdmin      = "admin"
	RoleUser       = "user"
)

// ValidRole 判断角色是否为系统当前支持的角色。
func ValidRole(role string) bool {
	return role == RoleSuperAdmin || role == RoleAdmin || role == RoleUser
}

// 用户状态
const (
	UserStatusActive = 1
	UserStatusBanned = 2
)

// 判题状态
const (
	StatusPending             = "Pending"
	StatusJudging             = "Judging"
	StatusAccepted            = "Accepted"
	StatusWrongAnswer         = "WrongAnswer"
	StatusTimeLimitExceeded   = "TimeLimitExceeded"
	StatusMemoryLimitExceeded = "MemoryLimitExceeded"
	StatusRuntimeError        = "RuntimeError"
	StatusCompileError        = "CompileError"
	StatusSystemError         = "SystemError"
	StatusOutputLimitExceeded = "OutputLimitExceeded"
)

// 题目可见性
const (
	ProblemVisibilityPublic = 1
	ProblemVisibilityHidden = 2
)

// 题目归属类型
const (
	ProblemKindPublic   = "public"
	ProblemKindTeam     = "team"
	ProblemKindPersonal = "personal"
)

// 题目访问权限：公开题目所有人可访问，受限题目仅所属团队成员或创建者本人可访问。
const (
	ProblemAccessPublic  = 1
	ProblemAccessMembers = 2
)

// 预置题库的角色：主库放公共题目，团队库与个人库分别放团队题目与个人题目。
// 预置题库不能删除，但可以调整前缀与题号长度；后台自建题库的角色为空值。
const (
	ProblemBankRolePrimary  = "primary"
	ProblemBankRoleTeam     = "team"
	ProblemBankRolePersonal = "personal"
)

// ProblemBankMaxCodeLength 是题库前缀与题号长度之和的上限，对应题号字段的存储宽度。
const ProblemBankMaxCodeLength = 16

// ValidProblemAccess 判断访问权限取值是否合法。
func ValidProblemAccess(access int) bool {
	return access == ProblemAccessPublic || access == ProblemAccessMembers
}

// 比对模式
const (
	CompareModeDefault = "default"
	CompareModeExact   = "exact"
	CompareModeFloat   = "float"
)

// 提交语言
const (
	LangCpp = "cpp"
	LangC   = "c"
)

// SupportedLanguages 返回开放给用户选择的提交语言，键与评测进程的编译配置一一对应。
func SupportedLanguages() []map[string]string {
	return []map[string]string{
		{"key": LangCpp, "name": "C++"},
		{"key": LangC, "name": "C"},
	}
}

// 难度（沿用洛谷的七级体系，与前端 DIFFICULTIES 顺序一致）
const (
	DifficultyEntry        = "入门"
	DifficultyBasic        = "普及-"
	DifficultyIntermediate = "普及/提高-"
	DifficultyAdvanced     = "普及+/提高"
	DifficultyHard         = "提高+/省选-"
	DifficultyExpert       = "省选/NOI-"
	DifficultyMaster       = "NOI/NOI+/CTSC"
)

// DifficultyPoints 是各难度的默认积分，题目未显式指定积分时使用。
var DifficultyPoints = map[string]int{
	DifficultyEntry:        10,
	DifficultyBasic:        20,
	DifficultyIntermediate: 30,
	DifficultyAdvanced:     45,
	DifficultyHard:         60,
	DifficultyExpert:       80,
	DifficultyMaster:       100,
}

// DefaultPoints 返回难度对应的默认积分，难度为空或不在体系内时按中档处理。
func DefaultPoints(difficulty string) int {
	if points, ok := DifficultyPoints[difficulty]; ok {
		return points
	}
	return DifficultyPoints[DifficultyAdvanced]
}

// 比赛状态（按当前时间与起止时间计算）
const (
	ContestStatusPending = "pending"
	ContestStatusRunning = "running"
	ContestStatusEnded   = "ended"
)
