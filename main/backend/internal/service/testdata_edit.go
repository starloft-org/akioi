package service

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"akioi/internal/model"

	"gorm.io/gorm"
)

// UpdateScores 修改已有测试点的分值，文件与题号保持不变。
func (s *TestdataService) UpdateScores(problemID uint, scores map[int]int) error {
	cases, err := s.List(problemID)
	if err != nil {
		return err
	}
	if len(cases) == 0 {
		return fmt.Errorf("该题还没有测试数据")
	}
	known := make(map[int]bool, len(cases))
	for _, tc := range cases {
		known[tc.Index] = true
	}
	for index := range scores {
		if !known[index] {
			return fmt.Errorf("测试点 %d 不存在", index)
		}
	}
	updated := make([]model.Testcase, 0, len(cases))
	for _, tc := range cases {
		if value, ok := scores[tc.Index]; ok {
			tc.Score = value
		}
		updated = append(updated, tc)
	}
	return s.db.Transaction(func(tx *gorm.DB) error {
		for _, tc := range updated {
			if err := tx.Model(&model.Testcase{}).
				Where("problem_id = ? AND case_index = ?", problemID, tc.Index).
				Update("score", tc.Score).Error; err != nil {
				return err
			}
		}
		var prob model.Problem
		if err := tx.Select("id").First(&prob, problemID).Error; err != nil {
			return err
		}
		scoresList := make([]int, len(updated))
		for i := range updated {
			scoresList[i] = updated[i].Score
		}
		prob.SetScores(scoresList)
		return tx.Model(&model.Problem{}).Where("id = ?", problemID).
			Update("testcase_scores", prob.ScoresJSON).Error
	})
}

// Reorder 按给定顺序重排测试点：重写 1.in/1.out… 文件名，并同步测试点序号与子任务引用。
func (s *TestdataService) Reorder(problemID uint, order []int) error {
	cases, err := s.List(problemID)
	if err != nil {
		return err
	}
	if len(cases) == 0 {
		return fmt.Errorf("该题还没有测试数据")
	}
	if len(order) != len(cases) {
		return fmt.Errorf("排序参数数量与测试点数量不一致")
	}
	byIndex := make(map[int]model.Testcase, len(cases))
	for _, tc := range cases {
		byIndex[tc.Index] = tc
	}
	seen := make(map[int]bool, len(order))
	inputs := make([][]byte, len(order))
	outputs := make([][]byte, len(order))
	for i, old := range order {
		if seen[old] {
			return fmt.Errorf("排序参数中有重复的测试点")
		}
		if _, ok := byIndex[old]; !ok {
			return fmt.Errorf("排序参数中有不存在的测试点 %d", old)
		}
		seen[old] = true
		input, err := os.ReadFile(s.InputPath(problemID, old))
		if err != nil {
			return fmt.Errorf("读取测试点 %d 的输入失败: %w", old, err)
		}
		output, err := os.ReadFile(s.OutputPath(problemID, old))
		if err != nil {
			return fmt.Errorf("读取测试点 %d 的输出失败: %w", old, err)
		}
		inputs[i] = input
		outputs[i] = output
	}
	dir := s.Dir(problemID)
	tmpDir := dir + ".reorder"
	if err := os.RemoveAll(tmpDir); err != nil {
		return err
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return err
	}
	for i := range order {
		if err := os.WriteFile(filepath.Join(tmpDir, strconv.Itoa(i+1)+".in"), inputs[i], 0o644); err != nil {
			_ = os.RemoveAll(tmpDir)
			return err
		}
		if err := os.WriteFile(filepath.Join(tmpDir, strconv.Itoa(i+1)+".out"), outputs[i], 0o644); err != nil {
			_ = os.RemoveAll(tmpDir)
			return err
		}
	}
	oldToNew := make(map[int]int, len(order))
	for i, old := range order {
		oldToNew[old] = i + 1
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Testcase{}).Where("problem_id = ?", problemID).
			UpdateColumn("case_index", gorm.Expr("-case_index")).Error; err != nil {
			return err
		}
		for i, old := range order {
			if err := tx.Model(&model.Testcase{}).
				Where("problem_id = ? AND case_index = ?", problemID, -old).
				UpdateColumn("case_index", i+1).Error; err != nil {
				return err
			}
		}
		var prob model.Problem
		if err := tx.First(&prob, problemID).Error; err != nil {
			return err
		}
		subtasks := prob.Subtasks()
		for i := range subtasks {
			next := make([]int, 0, len(subtasks[i].Cases))
			for _, old := range subtasks[i].Cases {
				if newIndex, ok := oldToNew[old]; ok {
					next = append(next, newIndex)
				}
			}
			sort.Ints(next)
			subtasks[i].Cases = next
		}
		prob.SetSubtasks(subtasks)
		return tx.Model(&model.Problem{}).Where("id = ?", problemID).
			Update("subtasks", prob.SubtasksJSON).Error
	})
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	if err := os.Rename(tmpDir, dir); err != nil {
		return err
	}
	return s.refreshCaseHashes(problemID)
}

// refreshCaseHashes 重排后重算各测试点的文件大小、哈希以及题目的数据指纹。
func (s *TestdataService) refreshCaseHashes(problemID uint) error {
	cases, err := s.List(problemID)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	for i := range cases {
		input, err := os.ReadFile(s.InputPath(problemID, cases[i].Index))
		if err != nil {
			return err
		}
		output, err := os.ReadFile(s.OutputPath(problemID, cases[i].Index))
		if err != nil {
			return err
		}
		cases[i].InputSize = int64(len(input))
		cases[i].OutputSize = int64(len(output))
		cases[i].InputHash = hashBytes(input)
		cases[i].OutputHash = hashBytes(output)
		_, _ = hasher.Write([]byte(cases[i].InputHash + cases[i].OutputHash))
		if err := s.db.Model(&model.Testcase{}).
			Where("problem_id = ? AND case_index = ?", problemID, cases[i].Index).
			Updates(map[string]any{
				"input_size":  cases[i].InputSize,
				"output_size": cases[i].OutputSize,
				"input_hash":  cases[i].InputHash,
				"output_hash": cases[i].OutputHash,
			}).Error; err != nil {
			return err
		}
	}
	return s.db.Model(&model.Problem{}).Where("id = ?", problemID).
		Update("data_hash", hex.EncodeToString(hasher.Sum(nil))).Error
}
