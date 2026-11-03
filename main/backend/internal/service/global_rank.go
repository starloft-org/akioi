package service

import (
	"math"
	"sort"
	"time"

	"akioi/internal/model"
)

// GlobalRankItem 是公开题库的积分榜条目。
type GlobalRankItem struct {
	Rank        int     `json:"rank"`
	UserID      uint    `json:"user_id"`
	Username    string  `json:"username"`
	Nickname    string  `json:"nickname"`
	Points      float64 `json:"points"`
	SolvedCount int     `json:"solved_count"`
}

type bestScoreRow struct {
	UserID    uint
	ProblemID uint
	BestScore int
}

// ComputeGlobal 计算公开题目的全站积分榜。
func (s *RankService) ComputeGlobal() ([]GlobalRankItem, error) {
	var users []model.User
	if err := s.db.Where("status = ?", model.UserStatusActive).
		Where("role NOT IN ?", []string{model.RoleAdmin, model.RoleSuperAdmin}).
		Order("id asc").Find(&users).Error; err != nil {
		return nil, err
	}
	// 只有公共题库里的公共题计入全站积分：用户自建题目与团队/个人题目不计分，
	// 避免用户给自己出的题设高分刷榜。
	var problems []model.Problem
	if err := s.db.Where("visibility = ?", model.ProblemVisibilityPublic).
		Where("owner_id = 0 AND team_id = 0 AND access = ?", model.ProblemAccessPublic).
		Find(&problems).Error; err != nil {
		return nil, err
	}

	problemByID := make(map[uint]model.Problem, len(problems))
	problemIDs := make([]uint, 0, len(problems))
	for i := range problems {
		problemByID[problems[i].ID] = problems[i]
		problemIDs = append(problemIDs, problems[i].ID)
	}

	rows := make([]bestScoreRow, 0)
	if len(problemIDs) > 0 {
		if err := s.db.Model(&model.Submission{}).
			Select("user_id, problem_id, MAX(score) AS best_score").
			Where("problem_id IN ?", problemIDs).
			// 进行中的比赛提交不进入全站积分，比赛结束后才计入。
			Where("contest_id = 0 OR contest_id IN (?)",
				s.db.Model(&model.Contest{}).Select("id").Where("end_at < ?", time.Now())).
			Group("user_id, problem_id").Scan(&rows).Error; err != nil {
			return nil, err
		}
	}

	items := make([]GlobalRankItem, 0, len(users))
	index := make(map[uint]int, len(users))
	for i := range users {
		index[users[i].ID] = i
		items = append(items, GlobalRankItem{
			UserID: users[i].ID, Username: users[i].Username, Nickname: users[i].Nickname,
		})
	}
	for _, row := range rows {
		i, ok := index[row.UserID]
		p, found := problemByID[row.ProblemID]
		if !ok || !found {
			continue
		}
		full := problemFullScore(&p)
		if full <= 0 || p.Points <= 0 || row.BestScore <= 0 {
			continue
		}
		best := row.BestScore
		if best > full {
			best = full
		}
		items[i].Points += float64(p.Points) * float64(best) / float64(full)
		if best >= full {
			items[i].SolvedCount++
		}
	}
	for i := range items {
		items[i].Points = math.Round(items[i].Points*100) / 100
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Points != items[j].Points {
			return items[i].Points > items[j].Points
		}
		if items[i].SolvedCount != items[j].SolvedCount {
			return items[i].SolvedCount > items[j].SolvedCount
		}
		return items[i].UserID < items[j].UserID
	})
	for i := range items {
		items[i].Rank = i + 1
	}
	return items, nil
}

func problemFullScore(p *model.Problem) int {
	return p.FullScore()
}
