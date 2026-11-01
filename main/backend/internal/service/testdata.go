package service

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

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

// testdataPlan 是校验通过的测试数据布局：序号、成对的文件与每题得分。
type testdataPlan struct {
	indexes []int
	grouped map[int]map[string][]byte
	scores  []int
}

// planTestdata 校验测试数据是否成对、数量是否超限，不写文件也不动数据库。
func (s *TestdataService) planTestdata(files map[string][]byte, scores []int) (*testdataPlan, error) {
	grouped := make(map[int]map[string][]byte)
	for name, content := range files {
		base := strings.ToLower(filepath.Base(name))
		ext := filepath.Ext(base)
		if ext != ".in" && ext != ".out" && ext != ".ans" {
			continue
		}
		idx, err := strconv.Atoi(strings.TrimSuffix(base, ext))
		if err != nil || idx < 1 {
			continue
		}
		if grouped[idx] == nil {
			grouped[idx] = make(map[string][]byte)
		}
		key := "out"
		if ext == ".in" {
			key = "in"
		}
		grouped[idx][key] = content
	}
	if len(grouped) == 0 {
		return nil, fmt.Errorf("未找到形如 1.in / 1.out 的测试数据文件")
	}

	indexes := make([]int, 0, len(grouped))
	for idx, pair := range grouped {
		if _, ok := pair["in"]; !ok {
			return nil, fmt.Errorf("测试点 %d 缺少输入文件 %d.in", idx, idx)
		}
		if _, ok := pair["out"]; !ok {
			return nil, fmt.Errorf("测试点 %d 缺少输出文件 %d.out", idx, idx)
		}
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	maxTestcases := s.settings.Current().MaxTestcases
	if len(indexes) > maxTestcases {
		return nil, fmt.Errorf("测试点数量 %d 超过上限 %d", len(indexes), maxTestcases)
	}
	return &testdataPlan{indexes: indexes, grouped: grouped, scores: normalizeScores(len(indexes), scores)}, nil
}

// Validate 只校验测试数据，不落盘也不写库，供导入前整体校验使用。
func (s *TestdataService) Validate(files map[string][]byte) error {
	_, err := s.planTestdata(files, nil)
	return err
}

// writeFiles 把测试数据写到题目目录，返回测试点清单与数据哈希。
func (p *testdataPlan) writeFiles(dir string) ([]model.Testcase, string, error) {
	if err := os.RemoveAll(dir); err != nil {
		return nil, "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", err
	}
	cases := make([]model.Testcase, 0, len(p.indexes))
	dataHasher := sha256.New()
	for i, srcIdx := range p.indexes {
		newIdx := i + 1
		in := p.grouped[srcIdx]["in"]
		out := p.grouped[srcIdx]["out"]
		if err := os.WriteFile(filepath.Join(dir, strconv.Itoa(newIdx)+".in"), in, 0o644); err != nil {
			return nil, "", err
		}
		if err := os.WriteFile(filepath.Join(dir, strconv.Itoa(newIdx)+".out"), out, 0o644); err != nil {
			return nil, "", err
		}
		inHash := hashBytes(in)
		outHash := hashBytes(out)
		_, _ = dataHasher.Write([]byte(inHash + outHash))
		cases = append(cases, model.Testcase{
			ProblemID:  0,
			Index:      newIdx,
			Score:      p.scores[i],
			InputSize:  int64(len(in)),
			OutputSize: int64(len(out)),
			InputHash:  inHash,
			OutputHash: outHash,
		})
	}
	return cases, hex.EncodeToString(dataHasher.Sum(nil)), nil
}

// persist 在给定连接或事务上写入测试点记录并回填题目的统计字段。
func (s *TestdataService) persistTestdata(db *gorm.DB, prob *model.Problem, cases []model.Testcase, dataHash string, scores []int) error {
	for i := range cases {
		cases[i].ProblemID = prob.ID
	}
	prob.TestcaseCount = len(cases)
	prob.SetScores(scores)
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
	if err := s.persistTestdata(tx, prob, cases, dataHash, plan.scores); err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	return nil
}

// Replace 用新上传的文件整体替换测试数据，文件名为 {序号}.in / {序号}.out（或 .ans）。
// scores 长度与测试点数不一致时按总分 100 均分。
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
		return s.persistTestdata(tx, prob, cases, dataHash, plan.scores)
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

func normalizeScores(n int, scores []int) []int {
	if n <= 0 {
		return nil
	}
	out := make([]int, n)
	if len(scores) == n {
		total := 0
		for _, s := range scores {
			if s < 0 {
				return evenScores(n)
			}
			total += s
		}
		if total > 0 {
			copy(out, scores)
			return out
		}
	}
	return evenScores(n)
}

// evenScores 把 100 分均分到 n 个测试点，余数分给前若干个点。
func evenScores(n int) []int {
	out := make([]int, n)
	if n <= 0 {
		return out
	}
	base := 100 / n
	rest := 100 % n
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
