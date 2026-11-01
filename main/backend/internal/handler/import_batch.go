package handler

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"akioi/internal/logger"
	"akioi/internal/model"
	"akioi/internal/service"

	"gorm.io/gorm"
)

// preparedProblem 是读完、解压并校验过的题目包；此时还没有写任何数据库记录。
type preparedProblem struct {
	filename    string
	pkg         *service.ImportedProblem
	files       map[string][]byte
	hasTestdata bool
}

// prepareProblem 解析题目包并校验测试数据，任何失败都不会留下副作用。
func (h *Handler) prepareProblem(filename string, content []byte) (*preparedProblem, error) {
	if !strings.EqualFold(filepath.Ext(filename), ".zip") {
		return nil, fmt.Errorf("题目包必须是 zip 文件")
	}
	files, err := service.ExtractZip(content)
	if err != nil {
		return nil, err
	}
	pkg, err := service.ParseProblemPackage(files)
	if err != nil {
		return nil, err
	}
	prep := &preparedProblem{filename: filename, pkg: pkg, files: files}
	for name := range files {
		switch strings.ToLower(filepath.Ext(name)) {
		case ".in", ".out", ".ans":
			prep.hasTestdata = true
		}
	}
	// 带测试数据时先整体校验，缺文件、不成对、超过测试点上限都不会入库
	if prep.hasTestdata {
		if err := h.Testdata.Validate(files); err != nil {
			return nil, fmt.Errorf("测试数据有误: %w", err)
		}
	}
	return prep, nil
}

// buildImportedProblem 把解析结果转成待插入的题目；题号由调用方在事务内分配。
func buildImportedProblem(prep *preparedProblem, u *model.User, bank *model.ProblemBank, displayID string) model.Problem {
	pkg := prep.pkg
	timeLimit := pkg.Config.TimeLimitMS
	if timeLimit < 100 || timeLimit > 30000 {
		timeLimit = 1000
	}
	memoryLimit := pkg.Config.MemoryLimitMB
	if memoryLimit < 16 || memoryLimit > 2048 {
		memoryLimit = 256
	}
	visibility := pkg.Config.Visibility
	if visibility != model.ProblemVisibilityHidden {
		visibility = model.ProblemVisibilityPublic
	}
	difficulty := strings.TrimSpace(pkg.Config.Difficulty)
	points := pkg.Config.Points
	if points <= 0 {
		points = model.DefaultPoints(difficulty)
	}
	p := model.Problem{
		DisplayID:     displayID,
		BankID:        bank.ID,
		Title:         strings.TrimSpace(pkg.Config.Title),
		Description:   pkg.Description,
		InputFormat:   pkg.InputFormat,
		OutputFormat:  pkg.OutputFormat,
		Hint:          pkg.Hint,
		TimeLimitMS:   timeLimit,
		MemoryLimitMB: memoryLimit,
		Difficulty:    difficulty,
		Source:        strings.TrimSpace(pkg.Config.Source),
		Points:        points,
		Visibility:    visibility,
		CompareMode:   model.CompareModeDefault,
		FloatEps:      1e-6,
		CreatedBy:     u.ID,
	}
	p.SetSamples(pkg.Samples)
	p.SetTags(pkg.Config.Tags)
	return p
}

// allocateProblemDisplayIDTx 在给定连接或事务上分配题库内的下一个题号。
func (h *Handler) allocateProblemDisplayIDTx(tx *gorm.DB, bank *model.ProblemBank) (string, error) {
	for attempt := 0; attempt < 5; attempt++ {
		displayID, err := service.NextProblemDisplayID(tx, bank)
		if err != nil {
			return "", err
		}
		var exists int64
		// 已删除题目占用的题号仍算占用，所以查重也用 Unscoped 统计。
		if err := tx.Unscoped().Model(&model.Problem{}).Where("display_id = ?", displayID).Count(&exists).Error; err != nil {
			return "", err
		}
		if exists == 0 {
			return displayID, nil
		}
	}
	return "", fmt.Errorf("题号分配冲突")
}

// importPreparedGroup 在一个事务里写入整组题目与测试数据；任一题失败则整组回滚。
// 测试数据文件会先落盘，回滚时由这里删除，保证失败不留下半成品。
func (h *Handler) importPreparedGroup(ctx context.Context, preps []*preparedProblem, u *model.User, bank *model.ProblemBank, ip string, progress importProgress) ([]importItem, error) {
	items := make([]importItem, len(preps))
	for i, prep := range preps {
		items[i] = importItem{Filename: prep.filename}
	}
	report := func(done int, note string) {
		if progress != nil {
			progress(done, len(preps), note)
		}
	}
	dirs := make([]string, 0, len(preps))
	err := h.DB.Transaction(func(tx *gorm.DB) error {
		for i, prep := range preps {
			report(i, "正在写入 "+prep.filename)
			displayID, err := h.allocateProblemDisplayIDTx(tx, bank)
			if err != nil {
				return fmt.Errorf("%s：生成题号失败：%w", prep.filename, err)
			}
			p := buildImportedProblem(prep, u, bank, displayID)
			if err := tx.Create(&p).Error; err != nil {
				return fmt.Errorf("%s：写入题目失败", prep.filename)
			}
			// 先登记目录，后面任何一步失败都要清理掉已写出的测试数据文件
			dirs = append(dirs, h.Testdata.Dir(p.ID))
			if prep.hasTestdata {
				if err := h.Testdata.Stage(tx, &p, prep.files, nil); err != nil {
					return fmt.Errorf("%s：%w", prep.filename, err)
				}
			}
			items[i].OK = true
			items[i].ProblemID = p.ID
			items[i].DisplayID = p.DisplayID
			items[i].Title = p.Title
			items[i].TestcaseCount = p.TestcaseCount
			report(i+1, "")
		}
		return nil
	})
	if err != nil {
		for _, dir := range dirs {
			_ = os.RemoveAll(dir)
		}
		for i := range items {
			items[i] = importItem{Filename: preps[i].filename, OK: false, Error: err.Error()}
		}
		return items, err
	}

	// 事务提交后再推送测试数据，避免回滚后节点上残留数据
	for i := range items {
		report(i, "正在推送测试数据 "+items[i].DisplayID)
		logger.Adminf("管理员 %s 导入题目 %s (id=%d) ip=%s", u.Username, items[i].DisplayID, items[i].ProblemID, ip)
		items[i].SyncFailed = h.Testdata.SyncToNodes(ctx, items[i].ProblemID)
		report(i+1, "")
	}
	return items, nil
}

// importPackageGroup 读入并校验整组题目包，然后一次性导入；任一题有问题整组都不入库。
func (h *Handler) importPackageGroup(ctx context.Context, staged []stagedImportFile, u *model.User, bank *model.ProblemBank, maxBytes int64, ip string, progress importProgress) ([]importItem, error) {
	preps := make([]*preparedProblem, 0, len(staged))
	for i, file := range staged {
		if progress != nil {
			progress(i, len(staged), "正在解析 "+file.Name)
		}
		content, err := readStagedFile(file, maxBytes)
		if err != nil {
			return failureItems(staged, err.Error()), err
		}
		prep, err := h.prepareProblem(file.Name, content)
		if err != nil {
			return failureItems(staged, fmt.Sprintf("%s：%s", file.Name, err.Error())), err
		}
		preps = append(preps, prep)
		if progress != nil {
			progress(i+1, len(staged), "")
		}
	}
	return h.importPreparedGroup(ctx, preps, u, bank, ip, progress)
}

// readStagedFile 读取临时目录里的上传包并检查大小上限。
func readStagedFile(file stagedImportFile, maxBytes int64) ([]byte, error) {
	info, err := os.Stat(file.Path)
	if err != nil {
		return nil, fmt.Errorf("%s：读取题目包失败", file.Name)
	}
	if maxBytes > 0 && info.Size() > maxBytes {
		return nil, fmt.Errorf("%s：文件超过大小上限", file.Name)
	}
	content, err := os.ReadFile(file.Path)
	if err != nil {
		return nil, fmt.Errorf("%s：读取题目包失败", file.Name)
	}
	return content, nil
}

// failureItems 为整组文件生成同一原因失败结果。
func failureItems(staged []stagedImportFile, reason string) []importItem {
	items := make([]importItem, len(staged))
	for i, file := range staged {
		items[i] = importItem{Filename: file.Name, Error: reason}
	}
	return items
}
