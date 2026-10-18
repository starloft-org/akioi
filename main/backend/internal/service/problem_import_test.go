package service

import "testing"

func TestParseProblemPackageLuoguMarkdown(t *testing.T) {
	fence := "```"
	md := "# P1001 A+B Problem\n\n" +
		"## 题目描述\n\n输入两个整数 $a, b$，输出它们的和。\n\n" +
		"## 输入格式\n\n两个以空格分开的整数。\n\n" +
		"## 输出格式\n\n一个整数。\n\n" +
		"## 样例 #1\n\n### 样例输入 #1\n\n" + fence + "\n1 2\n" + fence + "\n\n" +
		"### 样例输出 #1\n\n" + fence + "\n3\n" + fence + "\n\n" +
		"## 提示\n\n$-10^9 \\le a, b \\le 10^9$\n"

	pkg, err := ParseProblemPackage(map[string][]byte{
		"problem.md": []byte(md),
		"1.in":       []byte("1 2"),
		"1.out":      []byte("3"),
	})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if pkg.Config.Title != "A+B Problem" {
		t.Fatalf("标题解析错误: %q", pkg.Config.Title)
	}
	if pkg.Description != "输入两个整数 $a, b$，输出它们的和。" {
		t.Fatalf("题目描述解析错误: %q", pkg.Description)
	}
	if pkg.InputFormat != "两个以空格分开的整数。" {
		t.Fatalf("输入格式解析错误: %q", pkg.InputFormat)
	}
	if pkg.OutputFormat != "一个整数。" {
		t.Fatalf("输出格式解析错误: %q", pkg.OutputFormat)
	}
	if pkg.Hint != "$-10^9 \\le a, b \\le 10^9$" {
		t.Fatalf("提示解析错误: %q", pkg.Hint)
	}
	if len(pkg.Samples) != 1 {
		t.Fatalf("样例数量错误: %d", len(pkg.Samples))
	}
	if pkg.Samples[0].Input != "1 2" || pkg.Samples[0].Output != "3" {
		t.Fatalf("样例内容错误: %+v", pkg.Samples[0])
	}
}

func TestParseProblemPackageMergedIO(t *testing.T) {
	md := "# A0001 区间求和\n\n" +
		"## 题目描述\n\n求区间和。\n\n" +
		"## 输入输出格式\n\n### 输入格式\n\n第一行两个整数。\n\n" +
		"### 输出格式\n\n一个整数。\n"

	pkg, err := ParseProblemPackage(map[string][]byte{"problem.md": []byte(md)})
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if pkg.Title != "区间求和" {
		t.Fatalf("标题未去掉题号: %q", pkg.Title)
	}
	if pkg.InputFormat != "第一行两个整数。" {
		t.Fatalf("合并小节的输入格式解析错误: %q", pkg.InputFormat)
	}
	if pkg.OutputFormat != "一个整数。" {
		t.Fatalf("合并小节的输出格式解析错误: %q", pkg.OutputFormat)
	}
}

func TestParseProblemPackageMissingMarkdown(t *testing.T) {
	if _, err := ParseProblemPackage(map[string][]byte{"1.in": []byte("1")}); err == nil {
		t.Fatal("缺少题面时应当报错")
	}
}
