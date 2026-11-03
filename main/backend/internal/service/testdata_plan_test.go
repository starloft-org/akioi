package service

import (
	"testing"

	"akioi/internal/model"
)

// newPlanTestService 构造只用于校验计划的测试数据服务，不连数据库。
func newPlanTestService() *TestdataService {
	s := &SettingService{}
	s.current.Store(DefaultRuntimeSettings())
	return &TestdataService{settings: s}
}

func filesOf(pairs map[string]string) map[string][]byte {
	out := make(map[string][]byte, len(pairs))
	for name, content := range pairs {
		out[name] = []byte(content)
	}
	return out
}

// 任意文件名都能按同名配成一组，并按自然顺序排序。
func TestPlanTestdataAnyName(t *testing.T) {
	files := filesOf(map[string]string{
		"sample1.in":  "a",
		"sample1.out": "b",
		"10.in":       "c",
		"10.out":      "d",
		"2.in":        "e",
		"2.out":       "f",
		"题面.md":       "# 题目",
	})
	plan, err := newPlanTestService().planTestdata(files, nil)
	if err != nil {
		t.Fatalf("planTestdata 失败: %v", err)
	}
	got := []string{plan.cases[0].name, plan.cases[1].name, plan.cases[2].name}
	want := []string{"2", "10", "sample1"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("排序结果 = %v, 期望 %v", got, want)
		}
	}
	total := 0
	for _, c := range plan.cases {
		total += c.score
	}
	if total != 100 {
		t.Fatalf("默认总分 = %d, 期望 100", total)
	}
}

// 缺少输出文件时要给出可读的错误。
func TestPlanTestdataMissingOutput(t *testing.T) {
	files := filesOf(map[string]string{"a.in": "x", "b.in": "y", "b.out": "z"})
	if _, err := newPlanTestService().planTestdata(files, nil); err == nil {
		t.Fatal("缺少 a.out 应当报错")
	}
}

// 子任务按组计分，独立测试点用 codes 里指定的分值。
func TestPlanTestdataSubtasks(t *testing.T) {
	files := filesOf(map[string]string{
		"a.in": "1", "a.out": "1",
		"b.in": "2", "b.out": "2",
		"c.in": "3", "c.out": "3",
		"subtasks.json": `{"scores":{"c":10},"subtasks":[{"name":"小数据","score":40,"cases":["a","b"]}]}`,
	})
	plan, err := newPlanTestService().planTestdata(files, nil)
	if err != nil {
		t.Fatalf("planTestdata 失败: %v", err)
	}
	if len(plan.subtasks) != 1 {
		t.Fatalf("子任务数量 = %d, 期望 1", len(plan.subtasks))
	}
	if plan.subtasks[0].Cases[0] != 1 || plan.subtasks[0].Cases[1] != 2 {
		t.Fatalf("子任务包含的测试点 = %v, 期望 [1 2]", plan.subtasks[0].Cases)
	}
	if plan.cases[0].score != 0 || plan.cases[1].score != 0 {
		t.Fatalf("子任务内的测试点自身不应计分, 实得 %d / %d", plan.cases[0].score, plan.cases[1].score)
	}
	if plan.cases[2].score != 10 {
		t.Fatalf("独立测试点分值 = %d, 期望 10", plan.cases[2].score)
	}

	prob := model.Problem{}
	prob.SetScores([]int{plan.cases[0].score, plan.cases[1].score, plan.cases[2].score})
	prob.SetSubtasks(plan.subtasks)
	if got := prob.FullScore(); got != 50 {
		t.Fatalf("满分 = %d, 期望 50", got)
	}
}

// subtasks.json 引用了不存在的测试点时要报错。
func TestPlanTestdataUnknownSubtaskCase(t *testing.T) {
	files := filesOf(map[string]string{
		"a.in": "1", "a.out": "1",
		"subtasks.json": `{"subtasks":[{"name":"组","score":100,"cases":["nope"]}]}`,
	})
	if _, err := newPlanTestService().planTestdata(files, nil); err == nil {
		t.Fatal("引用不存在的测试点应当报错")
	}
}

func TestNaturalLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"2", "10", true},
		{"10", "2", false},
		{"abc", "abd", true},
		{"A", "a", false},
		{"sample", "sample2", true},
	}
	for _, c := range cases {
		if got := naturalLess(c.a, c.b); got != c.want {
			t.Fatalf("naturalLess(%q, %q) = %v, 期望 %v", c.a, c.b, got, c.want)
		}
	}
}

func TestSplitScore(t *testing.T) {
	got := splitScore(100, 3)
	// 余数从后往前补：前面分值更整齐。
	want := []int{33, 33, 34}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitScore(100, 3) = %v, 期望 %v", got, want)
		}
	}
}
