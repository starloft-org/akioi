package service

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"akioi/internal/config"
	"akioi/internal/model"

	"gorm.io/gorm"
)

// TestdataService 管理测试数据文件与元数据：文件落文件系统，库内只存清单与哈希。
type TestdataService struct {
	db       *gorm.DB
	cfg      *config.Config
	settings *SettingService
}

func NewTestdata(db *gorm.DB, cfg *config.Config, settings *SettingService) *TestdataService {
	return &TestdataService{db: db, cfg: cfg, settings: settings}
}

// Dir 返回某题的测试数据目录。
func (s *TestdataService) Dir(problemID uint) string {
	return filepath.Join(s.cfg.Judge.TestdataDir, strconv.FormatUint(uint64(problemID), 10))
}

// InputPath 返回某测试点输入文件路径。
func (s *TestdataService) InputPath(problemID uint, index int) string {
	return filepath.Join(s.Dir(problemID), strconv.Itoa(index)+".in")
}

// OutputPath 返回某测试点标准输出文件路径。
func (s *TestdataService) OutputPath(problemID uint, index int) string {
	return filepath.Join(s.Dir(problemID), strconv.Itoa(index)+".out")
}

// List 返回测试点元数据。
func (s *TestdataService) List(problemID uint) ([]model.Testcase, error) {
	var out []model.Testcase
	err := s.db.Where("problem_id = ?", problemID).Order("case_index asc").Find(&out).Error
	if out == nil {
		out = []model.Testcase{}
	}
	return out, err
}

// testdataPlan 是校验通过的测试数据布局：排序后的测试点文件、分值与子任务定义。
type testdataPlan struct {
	cases    []plannedCase
	subtasks []model.Subtask
}

// plannedCase 是一个测试点：原始文件名、输入输出内容、分值与所属子任务（0 为独立测试点）。
type plannedCase struct {
	name  string
	in    []byte
	out   []byte
	score int
	group int
}

// subtaskSpec 对应上传包里的 subtasks.json，用于声明子任务与按名字指定分值。
type subtaskSpec struct {
	Scores   map[string]int `json:"scores"`
	Subtasks []struct {
		Name  string   `json:"name"`
		Score int      `json:"score"`
		Cases []string `json:"cases"`
	} `json:"subtasks"`
}

// planTestdata 校验测试数据：.in 与 .out（或 .ans）按同名配对，文件名不限格式，
// 排序按自然顺序；可选的 subtasks.json 声明子任务与各测试点的分值。
func (s *TestdataService) planTestdata(files map[string][]byte, scores []int) (*testdataPlan, error) {
	pairs, names := pairTestdataFiles(files)
	if len(pairs) == 0 {
		return nil, fmt.Errorf("未找到测试数据文件：任意文件名的一对 .in 与 .out（或 .ans）即可，例如 sample.in 与 sample.out")
	}
	ordered := make([]string, 0, len(pairs))
	for key := range pairs {
		ordered = append(ordered, key)
	}
	sort.Slice(ordered, func(i, j int) bool { return naturalLess(names[ordered[i]], names[ordered[j]]) })
	maxTestcases := s.settings.Current().MaxTestcases
	if len(ordered) > maxTestcases {
		return nil, fmt.Errorf("测试点数量 %d 超过上限 %d", len(ordered), maxTestcases)
	}

	cases := make([]plannedCase, len(ordered))
	indexOf := make(map[string]int, len(ordered))
	for i, key := range ordered {
		pair := pairs[key]
		if _, ok := pair["in"]; !ok {
			return nil, fmt.Errorf("测试点 %s 缺少输入文件（%s.in）", names[key], names[key])
		}
		if _, ok := pair["out"]; !ok {
			return nil, fmt.Errorf("测试点 %s 缺少输出文件（%s.out 或 %s.ans）", names[key], names[key], names[key])
		}
		cases[i] = plannedCase{name: names[key], in: pair["in"], out: pair["out"]}
		indexOf[key] = i
	}

	spec, err := parseSubtaskSpec(files)
	if err != nil {
		return nil, err
	}
	groups, subtasks, err := buildSubtasks(spec, cases, indexOf)
	if err != nil {
		return nil, err
	}
	applyCaseScores(cases, groups, indexOf, spec, scores, subtasks)
	return &testdataPlan{cases: cases, subtasks: subtasks}, nil
}

// pairTestdataFiles 把上传的文件按「去掉扩展名后的同名」配成 (in, out) 对。
func pairTestdataFiles(files map[string][]byte) (map[string]map[string][]byte, map[string]string) {
	pairs := make(map[string]map[string][]byte)
	names := make(map[string]string)
	for raw, content := range files {
		base := filepath.Base(raw)
		lower := strings.ToLower(base)
		ext := filepath.Ext(lower)
		var kind string
		switch ext {
		case ".in":
			kind = "in"
		case ".out", ".ans":
			kind = "out"
		default:
			continue
		}
		stem := base[:len(base)-len(ext)]
		if stem == "" {
			continue
		}
		key := strings.ToLower(stem)
		if pairs[key] == nil {
			pairs[key] = make(map[string][]byte)
			names[key] = stem
		}
		pairs[key][kind] = content
	}
	return pairs, names
}

// parseSubtaskSpec 读取上传文件里的 subtasks.json；没有这个文件就返回 nil。
func parseSubtaskSpec(files map[string][]byte) (*subtaskSpec, error) {
	for name, content := range files {
		if !strings.EqualFold(filepath.Base(name), "subtasks.json") {
			continue
		}
		var spec subtaskSpec
		if err := json.Unmarshal(content, &spec); err != nil {
			return nil, fmt.Errorf("subtasks.json 格式错误: %w", err)
		}
		return &spec, nil
	}
	return nil, nil
}

// buildSubtasks 把 subtasks.json 的定义解析成测试点分组与子任务清单。
func buildSubtasks(spec *subtaskSpec, cases []plannedCase,
	indexOf map[string]int) ([]int, []model.Subtask, error) {

	groups := make([]int, len(cases))
	subtasks := []model.Subtask{}
	if spec == nil {
		return groups, subtasks, nil
	}
	for i, def := range spec.Subtasks {
		name := strings.TrimSpace(def.Name)
		if name == "" {
			return nil, nil, fmt.Errorf("第 %d 个子任务缺少名称", i+1)
		}
		if def.Score < 0 {
			return nil, nil, fmt.Errorf("子任务 %s 的分数不能为负数", name)
		}
		if len(def.Cases) == 0 {
			return nil, nil, fmt.Errorf("子任务 %s 没有指定测试点", name)
		}
		ids := make([]int, 0, len(def.Cases))
		for _, raw := range def.Cases {
			key := strings.ToLower(strings.TrimSpace(raw))
			idx, ok := indexOf[key]
			if !ok {
				return nil, nil, fmt.Errorf("子任务 %s 引用了不存在的测试点 %s", name, raw)
			}
			if groups[idx] != 0 {
				return nil, nil, fmt.Errorf("测试点 %s 被多个子任务引用", cases[idx].name)
			}
			groups[idx] = i + 1
			ids = append(ids, idx+1)
		}
		sort.Ints(ids)
		subtasks = append(subtasks, model.Subtask{ID: i + 1, Name: name, Score: def.Score, Cases: ids})
	}
	return groups, subtasks, nil
}

// applyCaseScores 决定每个测试点的分值：子任务内的测试点自身不计分（由子任务整体给分），
// 独立测试点的分值来自 subtasks.json 的 scores、位置参数 scores，都没有时把剩余满分均分。
func applyCaseScores(cases []plannedCase, groups []int, indexOf map[string]int,
	spec *subtaskSpec, scores []int, subtasks []model.Subtask) {

	assigned := make([]int, len(cases))
	if spec != nil && len(spec.Scores) > 0 {
		for key, score := range spec.Scores {
			idx, ok := indexOf[strings.ToLower(strings.TrimSpace(key))]
			if !ok || score < 0 {
				continue
			}
			assigned[idx] = score
		}
	} else if len(scores) == len(cases) {
		for i, score := range scores {
			if score >= 0 {
				assigned[i] = score
			}
		}
	}

	grouped := 0
	for _, st := range subtasks {
		grouped += st.Score
	}
	budget := 100 - grouped
	if budget < 0 {
		budget = 0
	}
	independent := make([]int, 0, len(cases))
	for i := range cases {
		if groups[i] != 0 {
			assigned[i] = 0
			continue
		}
		independent = append(independent, i)
	}
	share := splitScore(budget, len(independent))
	for pos, idx := range independent {
		if assigned[idx] == 0 {
			assigned[idx] = share[pos]
		}
	}
	for i := range cases {
		cases[i].score = assigned[i]
		cases[i].group = groups[i]
	}
}

// naturalLess 按自然顺序比较文件名，让 2.in 排在 10.in 前面。
func naturalLess(a, b string) bool {
	ra, rb := []rune(a), []rune(b)
	i, j := 0, 0
	for i < len(ra) && j < len(rb) {
		if unicode.IsDigit(ra[i]) && unicode.IsDigit(rb[j]) {
			i2, j2 := i, j
			for i2 < len(ra) && unicode.IsDigit(ra[i2]) {
				i2++
			}
			for j2 < len(rb) && unicode.IsDigit(rb[j2]) {
				j2++
			}
			na := strings.TrimLeft(string(ra[i:i2]), "0")
			nb := strings.TrimLeft(string(rb[j:j2]), "0")
			if len(na) != len(nb) {
				return len(na) < len(nb)
			}
			if na != nb {
				return na < nb
			}
			i, j = i2, j2
			continue
		}
		ca, cb := unicode.ToLower(ra[i]), unicode.ToLower(rb[j])
		if ca != cb {
			return ca < cb
		}
		i++
		j++
	}
	return len(ra)-i < len(rb)-j
}

// Validate 只校验测试数据，不落盘也不写库，供导入前整体校验使用。
func (s *TestdataService) Validate(files map[string][]byte) error {
	_, err := s.planTestdata(files, nil)
	return err
}

// writeFiles 把测试数据写到题目目录，返回测试点清单与数据哈希。
// 落盘文件名统一成 {序号}.in / {序号}.out，顺序即自然排序后的顺序。
func (p *testdataPlan) writeFiles(dir string) ([]model.Testcase, string, error) {
	if err := os.RemoveAll(dir); err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", err
	}
	cases := make([]model.Testcase, 0, len(p.cases))
	dataHasher := sha256.New()
	for i, c := range p.cases {
		newIdx := i + 1
		if err := os.WriteFile(filepath.Join(dir, strconv.Itoa(newIdx)+".in"), c.in, 0o644); err != nil {
			return nil, "", err
		}
		if err := os.WriteFile(filepath.Join(dir, strconv.Itoa(newIdx)+".out"), c.out, 0o644); err != nil {
			return nil, "", err
		}
		inHash := hashBytes(c.in)
		outHash := hashBytes(c.out)
		_, _ = dataHasher.Write([]byte(inHash + outHash))
		cases = append(cases, model.Testcase{
			ProblemID:  0,
			Index:      newIdx,
			Name:       c.name,
			GroupID:    c.group,
			Score:      c.score,
			InputSize:  int64(len(c.in)),
			OutputSize: int64(len(c.out)),
			InputHash:  inHash,
			OutputHash: outHash,
		})
	}
	return cases, hex.EncodeToString(dataHasher.Sum(nil)), nil
}

// persist 在给定连接或事务上写入测试点记录并回填题目的统计字段。
func (s *TestdataService) persistTestdata(db *gorm.DB, prob *model.Problem,
	cases []model.Testcase, dataHash string, subtasks []model.Subtask) error {

	for i := range cases {
		cases[i].ProblemID = prob.ID
	}
	scores := make([]int, len(cases))
	for i := range cases {
		scores[i] = cases[i].Score
	}
	prob.TestcaseCount = len(cases)
	prob.SetScores(scores)
	prob.SetSubtasks(subtasks)
	prob.DataHash = dataHash
	if err := db.Where("problem_id = ?", prob.ID).Delete(&model.Testcase{}).Error; err != nil {
		return err
	}
	if err := db.Create(&cases).Error; err != nil {
		return err
	}
	return db.Model(&model.Problem{}).Where("id = ?", prob.ID).Updates(map[string]any{
		"testcase_count":  prob.TestcaseCount,
		"testcase_scores": prob.ScoresJSON,
		"subtasks":        prob.SubtasksJSON,
		"data_hash":       prob.DataHash,
	}).Error
}

// Stage 在调用方给定的事务里写入测试数据文件与测试点记录。
// 事务回滚时数据库记录会撤销，调用方需要自行删除题目目录。
func (s *TestdataService) Stage(tx *gorm.DB, prob *model.Problem, files map[string][]byte, scores []int) error {
	plan, err := s.planTestdata(files, scores)
	if err != nil {
		return err
	}
	dir := s.Dir(prob.ID)
	cases, dataHash, err := plan.writeFiles(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	if err := s.persistTestdata(tx, prob, cases, dataHash, plan.subtasks); err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	return nil
}

// Replace 用新上传的文件整体替换测试数据，落盘文件名为 {序号}.in / {序号}.out（或 .ans）。
func (s *TestdataService) Replace(prob *model.Problem, files map[string][]byte, scores []int) error {
	plan, err := s.planTestdata(files, scores)
	if err != nil {
		return err
	}
	dir := s.Dir(prob.ID)
	cases, dataHash, err := plan.writeFiles(dir)
	if err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		return s.persistTestdata(tx, prob, cases, dataHash, plan.subtasks)
	}); err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	return nil
}

// Delete 删除某题全部测试数据。
func (s *TestdataService) Delete(problemID uint) error {
	if err := os.RemoveAll(s.Dir(problemID)); err != nil {
		return err
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("problem_id = ?", problemID).Delete(&model.Testcase{}).Error; err != nil {
			return err
		}
		return tx.Model(&model.Problem{}).Where("id = ?", problemID).Updates(map[string]any{
			"testcase_count":  0,
			"testcase_scores": "",
			"subtasks":        "",
			"data_hash":       "",
		}).Error
	})
}

// Zip 打包测试数据，供管理员下载。
func (s *TestdataService) Zip(problemID uint) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := writeZip(buf, s.Dir(problemID)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// writeZip 把目录下的文件写入 zip 流。
func writeZip(w io.Writer, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(w)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			_ = zw.Close()
			return err
		}
		fw, err := zw.Create(e.Name())
		if err != nil {
			_ = zw.Close()
			return err
		}
		if _, err := fw.Write(content); err != nil {
			_ = zw.Close()
			return err
		}
	}
	return zw.Close()
}

// ExtractZip 展开上传的 zip 数据包，返回文件名到内容的映射。
func ExtractZip(content []byte) (map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("数据包不是合法的 zip 文件: %w", err)
	}
	out := make(map[string][]byte)
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		buf := new(bytes.Buffer)
		if _, err := buf.ReadFrom(rc); err != nil {
			_ = rc.Close()
			return nil, err
		}
		_ = rc.Close()
		out[f.Name] = buf.Bytes()
	}
	return out, nil
}

// splitScore 把 total 分均分到 n 个测试点，余数分给前若干个点。
func splitScore(total, n int) []int {
	out := make([]int, n)
	if n <= 0 {
		return out
	}
	if total < 0 {
		total = 0
	}
	base := total / n
	rest := total % n
	for i := range out {
		out[i] = base
		if i < rest {
			out[i]++
		}
	}
	return out
}

func hashBytes(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
