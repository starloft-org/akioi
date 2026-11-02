package service

import (
	"time"

	"akioi/internal/model"

	"gorm.io/gorm"
)

// RankService 计算比赛榜单，OI 赛制按测试点累加得分并取同题最高分。
type RankService struct {
	db *gorm.DB
}

func NewRank(db *gorm.DB) *RankService {
	return &RankService{db: db}
}

// Compute 计算比赛榜单。frozen 为 true 时仅统计封榜时刻之前的提交；
// revealAll 为 true（比赛结束或管理员查看）时统计全部提交。
func (s *RankService) Compute(contest *model.Contest, now time.Time, revealAll bool) ([]model.RankItem, bool, error) {
	var cps []model.ContestProblem
	if err := s.db.Where("contest_id = ?", contest.ID).Order("order_index asc, id asc").Find(&cps).Error; err != nil {
		return nil, false, err
	}
	if len(cps) == 0 {
		return []model.RankItem{}, false, nil
	}

	problemIDs := make([]uint, 0, len(cps))
	labelOf := make(map[uint]string, len(cps))
	fullScore := make(map[uint]int, len(cps))
	order := make([]uint, 0, len(cps))
	for _, cp := range cps {
		problemIDs = append(problemIDs, cp.ProblemID)
		labelOf[cp.ProblemID] = cp.Label
		order = append(order, cp.ProblemID)
	}

	var problems []model.Problem
	if err := s.db.Where("id IN ?", problemIDs).Find(&problems).Error; err != nil {
		return nil, false, err
	}
	for i := range problems {
		fullScore[problems[i].ID] = problems[i].FullScore()
	}

	frozen := contest.IsFrozen(now)
	query := s.db.Where("contest_id = ?", contest.ID)
	if frozen && !revealAll {
		query = query.Where("created_at <= ?", contest.FreezeAt())
	}

	var adminIDs []uint
	if err := s.db.Model(&model.User{}).
		Where("role IN ?", []string{model.RoleAdmin, model.RoleSuperAdmin}).
		Pluck("id", &adminIDs).Error; err != nil {
		return nil, false, err
	}
	admins := make(map[uint]bool, len(adminIDs))
	for _, id := range adminIDs {
		admins[id] = true
	}

	var subs []model.Submission
	if err := query.Order("created_at asc, id asc").Find(&subs).Error; err != nil {
		return nil, frozen, err
	}

	type cell struct {
		score      int
		attempts   int
		accepted   bool
		acceptedAt int64
	}
	userCells := make(map[uint]map[uint]*cell)
	userLast := make(map[uint]int64)
	for i := range subs {
		sub := &subs[i]
		if admins[sub.UserID] {
			continue
		}
		cells, ok := userCells[sub.UserID]
		if !ok {
			cells = make(map[uint]*cell, len(order))
			userCells[sub.UserID] = cells
		}
		c, ok := cells[sub.ProblemID]
		if !ok {
			c = &cell{}
			cells[sub.ProblemID] = c
		}
		c.attempts++
		if sub.Score > c.score {
			c.score = sub.Score
		}
		if !c.accepted && fullScore[sub.ProblemID] > 0 && sub.Score >= fullScore[sub.ProblemID] {
			c.accepted = true
			c.acceptedAt = sub.CreatedAt.Unix()
		}
		ts := sub.CreatedAt.Unix()
		if ts > userLast[sub.UserID] {
			userLast[sub.UserID] = ts
		}
	}

	userIDs := make([]uint, 0, len(userCells))
	for id := range userCells {
		userIDs = append(userIDs, id)
	}
	names := make(map[uint]model.User, len(userIDs))
	if len(userIDs) > 0 {
		var users []model.User
		if err := s.db.Where("id IN ?", userIDs).Find(&users).Error; err != nil {
			return nil, frozen, err
		}
		for i := range users {
			names[users[i].ID] = users[i]
		}
	}

	items := make([]model.RankItem, 0, len(userIDs))
	for uid, cells := range userCells {
		item := model.RankItem{UserID: uid, LastSubmit: userLast[uid]}
		if u, ok := names[uid]; ok {
			item.Username = u.Username
			item.Nickname = u.Nickname
		}
		for _, pid := range order {
			pc := model.RankProblemCell{ProblemID: pid, Label: labelOf[pid]}
			if c, ok := cells[pid]; ok {
				pc.Score = c.score
				pc.Attempts = c.attempts
				pc.Accepted = c.accepted
				pc.AcceptedAt = c.acceptedAt
			}
			item.Problems = append(item.Problems, pc)
			item.TotalScore += pc.Score
			if pc.Accepted {
				item.AcceptedNum++
			}
		}
		items = append(items, item)
	}

	sortRankItems(items)
	for i := range items {
		items[i].Rank = i + 1
	}
	return items, frozen, nil
}

// sortRankItems 按总分降序、通过题数降序、最后提交时间升序排列。
func sortRankItems(items []model.RankItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && lessRank(items[j], items[j-1]); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

func lessRank(a, b model.RankItem) bool {
	if a.TotalScore != b.TotalScore {
		return a.TotalScore > b.TotalScore
	}
	if a.AcceptedNum != b.AcceptedNum {
		return a.AcceptedNum > b.AcceptedNum
	}
	return a.LastSubmit < b.LastSubmit
}
