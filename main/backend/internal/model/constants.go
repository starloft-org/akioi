package model

// 用户角色
const (
	RoleAdmin = "admin"
	RoleUser  = "user"
)

// ValidRole 判断角色是否为系统当前支持的角色。
func ValidRole(role string) bool {
	return role == RoleAdmin || role == RoleUser
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

// 难度
const (
	DifficultyEntry    = "入门"
	DifficultyPopularM = "普及-"
	DifficultyPopular  = "普及"
	DifficultyImprove  = "提高"
	DifficultyProvince = "省选"
	DifficultyNOI      = "NOI"
)

// 比赛状态（按当前时间与起止时间计算）
const (
	ContestStatusPending = "pending"
	ContestStatusRunning = "running"
	ContestStatusEnded   = "ended"
)
