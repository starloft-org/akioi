// 运维 CLI：迁移、账号维护、批量导入与重测。
package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"strings"

	"akioi/internal/config"
	"akioi/internal/database"
	"akioi/internal/logger"
	"akioi/internal/migrate"
	"akioi/internal/model"
	"akioi/internal/redis"
	"akioi/internal/service"

	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

const usage = `用法: akioi-cli <command> [options]

命令:
  migrate                执行未应用的数据库迁移并补齐表结构
  status                 显示迁移状态
  reset-password         重置账号密码   -user <用户名> -password <新密码>
  set-role               调整账号角色   -user <用户名> -role <admin|user>
  create-user            创建账号       -user <用户名> -password <密码> [-nickname 昵称] [-role 角色]
  import-users           批量导入 CSV   -file <路径> [-default-password 密码]
  rejudge                重新评测       [-problem <题号>] [-contest <比赛ID>] [-ids 1,2,3]
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "配置加载失败:", err)
		os.Exit(1)
	}
	if err := logger.Init(cfg.LogDir, cfg.LogMaxMB); err != nil {
		fmt.Fprintln(os.Stderr, "日志初始化失败:", err)
		os.Exit(1)
	}

	switch cmd {
	case "migrate":
		runMigrate(cfg)
	case "status":
		runStatus(cfg)
	case "reset-password":
		runResetPassword(cfg, args)
	case "set-role":
		runSetRole(cfg, args)
	case "create-user":
		runCreateUser(cfg, args)
	case "import-users":
		runImportUsers(cfg, args)
	case "rejudge":
		runRejudge(cfg, args)
	default:
		fmt.Print(usage)
		os.Exit(2)
	}
}

func withDB(cfg *config.Config) *gorm.DB {
	db, err := database.Connect(cfg)
	if err != nil {
		fatal("连接数据库失败: " + err.Error())
	}
	return db
}

func withRedis(cfg *config.Config) *goredis.Client {
	rdb, err := redis.Connect(cfg)
	if err != nil {
		fatal("连接 Redis 失败: " + err.Error())
	}
	return rdb
}

func runMigrate(cfg *config.Config) {
	if err := migrate.Run(database.MigrateDSN(cfg)); err != nil {
		fatal("执行迁移失败: " + err.Error())
	}
	db := withDB(cfg)
	if err := database.AutoMigrate(db); err != nil {
		fatal("补齐表结构失败: " + err.Error())
	}
	fmt.Println("数据库迁移完成")
}

func runStatus(cfg *config.Config) {
	pending, current, err := migrate.Pending(database.MigrateDSN(cfg))
	if err != nil {
		fatal("读取迁移状态失败: " + err.Error())
	}
	fmt.Printf("当前版本: %d, 待应用版本: %v\n", current, pending)
}

func runResetPassword(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("reset-password", flag.ExitOnError)
	username := fs.String("user", "", "用户名")
	password := fs.String("password", "", "新密码")
	_ = fs.Parse(args)

	if *username == "" || *password == "" {
		fatal("必须提供 -user 与 -password")
	}
	if len(*password) < 6 {
		fatal("密码长度至少 6 位")
	}
	db := withDB(cfg)
	var u model.User
	if err := db.Where("username = ?", *username).First(&u).Error; err != nil {
		fatal("用户不存在: " + *username)
	}
	auth := service.NewAuth(db, withRedis(cfg), cfg)
	if err := auth.ResetPassword(u.ID, *password); err != nil {
		fatal("重置失败: " + err.Error())
	}
	fmt.Println("密码已重置:", *username)
}

func runSetRole(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("set-role", flag.ExitOnError)
	username := fs.String("user", "", "用户名")
	role := fs.String("role", "", "角色")
	_ = fs.Parse(args)

	if *username == "" || *role == "" {
		fatal("必须提供 -user 与 -role")
	}
	if !model.ValidRole(*role) {
		fatal("角色只能是 admin 或 user")
	}
	db := withDB(cfg)
	res := db.Model(&model.User{}).Where("username = ?", *username).UpdateColumn("role", *role)
	if res.Error != nil {
		fatal("修改失败: " + res.Error.Error())
	}
	if res.RowsAffected == 0 {
		fatal("用户不存在: " + *username)
	}
	fmt.Printf("已将 %s 的角色设置为 %s\n", *username, *role)
}

func runCreateUser(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("create-user", flag.ExitOnError)
	username := fs.String("user", "", "用户名")
	password := fs.String("password", "", "密码")
	nickname := fs.String("nickname", "", "昵称")
	role := fs.String("role", model.RoleUser, "角色")
	_ = fs.Parse(args)

	if *username == "" || *password == "" {
		fatal("必须提供 -user 与 -password")
	}
	if len(*password) < 6 {
		fatal("密码长度至少 6 位")
	}
	if !model.ValidRole(*role) {
		fatal("角色只能是 admin 或 user")
	}
	db := withDB(cfg)
	var exists int64
	db.Model(&model.User{}).Where("username = ?", *username).Count(&exists)
	if exists > 0 {
		fatal("用户已存在: " + *username)
	}
	auth := service.NewAuth(db, withRedis(cfg), cfg)
	hash, err := auth.HashPassword(*password)
	if err != nil {
		fatal("密码加密失败: " + err.Error())
	}
	if *nickname == "" {
		*nickname = *username
	}
	u := model.User{
		Username:     *username,
		PasswordHash: hash,
		Nickname:     *nickname,
		Role:         *role,
		Status:       model.UserStatusActive,
	}
	if err := db.Create(&u).Error; err != nil {
		fatal("创建失败: " + err.Error())
	}
	fmt.Printf("已创建账号 %s（角色 %s）\n", u.Username, u.Role)
}

func runImportUsers(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("import-users", flag.ExitOnError)
	file := fs.String("file", "", "CSV 路径")
	defaultPassword := fs.String("default-password", "", "缺少密码列时的默认密码")
	_ = fs.Parse(args)

	if *file == "" {
		fatal("必须提供 -file")
	}
	f, err := os.Open(*file)
	if err != nil {
		fatal("打开文件失败: " + err.Error())
	}
	defer func() { _ = f.Close() }()

	reader := csv.NewReader(f)
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		fatal("解析 CSV 失败: " + err.Error())
	}
	db := withDB(cfg)
	auth := service.NewAuth(db, withRedis(cfg), cfg)

	imported, skipped := 0, 0
	for i, row := range records {
		if len(row) == 0 {
			continue
		}
		username := strings.TrimSpace(row[0])
		if username == "" || strings.EqualFold(username, "username") {
			continue
		}
		password := *defaultPassword
		if len(row) > 1 && strings.TrimSpace(row[1]) != "" {
			password = strings.TrimSpace(row[1])
		}
		if len(password) < 6 {
			fmt.Printf("第 %d 行跳过：密码缺失或过短\n", i+1)
			skipped++
			continue
		}
		nickname := username
		if len(row) > 2 && strings.TrimSpace(row[2]) != "" {
			nickname = strings.TrimSpace(row[2])
		}
		phone := ""
		if len(row) > 3 {
			phone = strings.TrimSpace(row[3])
			if phone != "" && !service.ValidPhone(phone) {
				fmt.Printf("第 %d 行跳过：手机号格式不正确\n", i+1)
				skipped++
				continue
			}
		}
		role := model.RoleUser
		if len(row) > 4 && strings.TrimSpace(row[4]) != "" {
			role = strings.TrimSpace(row[4])
		}
		if !model.ValidRole(role) {
			fmt.Printf("第 %d 行跳过：角色只能是 admin 或 user\n", i+1)
			skipped++
			continue
		}
		var exists int64
		db.Model(&model.User{}).Where("username = ?", username).Count(&exists)
		if exists > 0 {
			skipped++
			continue
		}
		hash, err := auth.HashPassword(password)
		if err != nil {
			fmt.Printf("第 %d 行跳过：密码加密失败\n", i+1)
			skipped++
			continue
		}
		u := model.User{
			Username: username, PasswordHash: hash, Nickname: nickname,
			Phone: model.PhonePointer(phone), Role: role, Status: model.UserStatusActive,
		}
		if err := db.Create(&u).Error; err != nil {
			fmt.Printf("第 %d 行跳过：%v\n", i+1, err)
			skipped++
			continue
		}
		imported++
	}
	fmt.Printf("导入完成：成功 %d，跳过 %d\n", imported, skipped)
}

func runRejudge(cfg *config.Config, args []string) {
	fs := flag.NewFlagSet("rejudge", flag.ExitOnError)
	problemID := fs.Uint("problem", 0, "题目 ID")
	contestID := fs.Uint("contest", 0, "比赛 ID")
	ids := fs.String("ids", "", "提交 ID，逗号分隔")
	_ = fs.Parse(args)

	db := withDB(cfg)
	judgeSvc := service.NewJudge(db, cfg, withRedis(cfg))
	ctx := context.Background()

	var (
		count int
		err   error
	)
	switch {
	case *ids != "":
		count, err = judgeSvc.Rejudge(ctx, service.ParseIDs(*ids))
	case *problemID > 0:
		count, err = judgeSvc.RejudgeProblem(ctx, uint(*problemID))
	case *contestID > 0:
		count, err = judgeSvc.RejudgeContest(ctx, uint(*contestID))
	default:
		fatal("必须提供 -ids、-problem 或 -contest 之一")
	}
	if err != nil {
		fatal("投递重测失败: " + err.Error())
	}
	fmt.Printf("已投递重测任务 %d 条\n", count)
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(1)
}
