package service

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"

	"akioi/internal/model"
)

// ImportConfig 是题目包里的可选配置文件，字段与后台新建题目保持一致。
type ImportConfig struct {
	Title         string   `json:"title"`
	DisplayID     string   `json:"display_id"`
	Difficulty    string   `json:"difficulty"`
	TimeLimitMS   int      `json:"time_limit_ms"`
	MemoryLimitMB int      `json:"memory_limit_mb"`
	Points        int      `json:"points"`
	Tags          []string `json:"tags"`
	Source        string   `json:"source"`
	Visibility    int      `json:"visibility"`
}

// ImportedProblem 是解析后的题目包内容。
type ImportedProblem struct {
	Config       ImportConfig
	Title        string
	Description  string
	InputFormat  string
	OutputFormat string
	Hint         string
	Samples      []model.Sample
}

var (
	titleLineRe   = regexp.MustCompile(`(?m)^#\s+(.+?)\s*$`)
	sectionLineRe = regexp.MustCompile(`(?m)^##\s+(.+?)\s*$`)
	codeBlockRe   = regexp.MustCompile("(?s)```[^\n]*\n(.*?)```")
	// 洛谷一级标题通常带题号，如 "P1001 A+B Problem"。
	titlePrefixRe = regexp.MustCompile(`^[A-Za-z]+\d+\s+`)
)

// ParseProblemPackage 从 zip 解出的文件集合里解析题目包：
// 题面取 Markdown（优先 problem.md），元信息取可选的 config.json。
func ParseProblemPackage(files map[string][]byte) (*ImportedProblem, error) {
	name := findMarkdownFile(files)
	if name == "" {
		return nil, fmt.Errorf("题目包中未找到 Markdown 题面，请放入 problem.md")
	}

	pkg := &ImportedProblem{}
	if raw, ok := files["config.json"]; ok {
		if err := json.Unmarshal(raw, &pkg.Config); err != nil {
			return nil, fmt.Errorf("config.json 格式错误: %w", err)
		}
	}
	parseLuoguMarkdown(string(files[name]), pkg)
	if pkg.Config.Title == "" {
		pkg.Config.Title = pkg.Title
	}
	if strings.TrimSpace(pkg.Config.Title) == "" {
		return nil, fmt.Errorf("未能确定题目标题，请在题面写一级标题或在 config.json 中指定 title")
	}
	return pkg, nil
}

func findMarkdownFile(files map[string][]byte) string {
	if _, ok := files["problem.md"]; ok {
		return "problem.md"
	}
	for name := range files {
		if strings.EqualFold(path.Ext(name), ".md") && !strings.Contains(name, "/") {
			return name
		}
	}
	for name := range files {
		if strings.EqualFold(path.Ext(name), ".md") {
			return name
		}
	}
	return ""
}

// parseLuoguMarkdown 按洛谷「复制 Markdown」的结构拆出各段落。
func parseLuoguMarkdown(content string, pkg *ImportedProblem) {
	text := strings.ReplaceAll(content, "\r\n", "\n")

	if m := titleLineRe.FindStringSubmatch(text); len(m) == 2 {
		pkg.Title = strings.TrimSpace(titlePrefixRe.ReplaceAllString(m[1], ""))
	}

	sections := sectionLineRe.FindAllStringSubmatchIndex(text, -1)
	for i, m := range sections {
		name := strings.TrimSpace(text[m[2]:m[3]])
		start := m[1]
		end := len(text)
		if i+1 < len(sections) {
			end = sections[i+1][0]
		}
		body := strings.TrimSpace(text[start:end])

		switch {
		case strings.HasPrefix(name, "题目描述"):
			pkg.Description = body
		case strings.HasPrefix(name, "输入输出格式"):
			// 老版洛谷把输入输出合在一节，用三级标题再分。
			if sub := subsection(body, "输入格式"); sub != "" {
				pkg.InputFormat = sub
			}
			if sub := subsection(body, "输出格式"); sub != "" {
				pkg.OutputFormat = sub
			}
		case strings.HasPrefix(name, "输入格式"):
			pkg.InputFormat = body
		case strings.HasPrefix(name, "输出格式"):
			pkg.OutputFormat = body
		case strings.HasPrefix(name, "提示"), strings.HasPrefix(name, "说明"):
			if pkg.Hint == "" {
				pkg.Hint = body
			} else {
				pkg.Hint += "\n\n" + body
			}
		case strings.HasPrefix(name, "样例"):
			in := extractAfterLabel(body, "样例输入")
			out := extractAfterLabel(body, "样例输出")
			if in != "" || out != "" {
				pkg.Samples = append(pkg.Samples, model.Sample{Input: in, Output: out})
			}
		}
	}
}

// subsection 取 ### 标题下的正文，用于拆「输入输出格式」这类合并小节。
func subsection(body, label string) string {
	re := regexp.MustCompile(`(?m)^###\s+` + regexp.QuoteMeta(label) + `\s*$`)
	loc := re.FindStringIndex(body)
	if loc == nil {
		return ""
	}
	rest := body[loc[1]:]
	if next := regexp.MustCompile(`(?m)^###\s+`).FindStringIndex(rest); next != nil {
		rest = rest[:next[0]]
	}
	return strings.TrimSpace(rest)
}

// extractAfterLabel 取标签之后第一个代码块的内容，用作样例输入或输出。
func extractAfterLabel(body, label string) string {
	idx := strings.Index(body, label)
	if idx < 0 {
		return ""
	}
	m := codeBlockRe.FindStringSubmatch(body[idx:])
	if len(m) < 2 {
		return ""
	}
	return strings.TrimRight(m[1], "\n")
}
