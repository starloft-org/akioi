package judge

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
	"strings"

	"akioi/internal/model"
)

// CompareResult 是输出比对的结论。
type CompareResult struct {
	Accepted bool
	Message  string
}

// Compare 按比对模式比较标准输出与选手输出；消息中不含标准答案内容。
func Compare(mode string, eps float64, expected, actual []byte) CompareResult {
	switch mode {
	case model.CompareModeExact:
		return compareExact(expected, actual)
	case model.CompareModeFloat:
		if math.IsNaN(eps) || math.IsInf(eps, 0) || eps < 0 {
			eps = 0
		}
		return compareFloat(expected, actual, eps)
	default:
		return compareDefault(expected, actual)
	}
}

// compareExact 逐字节比较，仅忽略文件末尾换行差异。
func compareExact(expected, actual []byte) CompareResult {
	e := bytes.TrimRight(expected, "\r\n")
	a := bytes.TrimRight(actual, "\r\n")
	if bytes.Equal(e, a) {
		return CompareResult{Accepted: true}
	}
	if len(e) != len(a) {
		return CompareResult{Message: fmt.Sprintf("输出长度不一致（标准 %d 字节，实际 %d 字节）", len(e), len(a))}
	}
	for i := 0; i < len(e); i++ {
		if e[i] != a[i] {
			return CompareResult{Message: fmt.Sprintf("第 %d 个字符不一致", i+1)}
		}
	}
	return CompareResult{Message: "输出不一致"}
}

// compareDefault 忽略行尾空白与末尾空行。
func compareDefault(expected, actual []byte) CompareResult {
	e := normalizeLines(expected)
	a := normalizeLines(actual)
	if len(e) != len(a) {
		return CompareResult{Message: fmt.Sprintf("行数不一致（标准 %d 行，实际 %d 行）", len(e), len(a))}
	}
	for i := range e {
		if e[i] != a[i] {
			return CompareResult{Message: fmt.Sprintf("第 %d 行不一致", i+1)}
		}
	}
	return CompareResult{Accepted: true}
}

// compareFloat 按空白切分逐个 token 比较，数字按误差比较，非数字按字符串比较。
func compareFloat(expected, actual []byte, eps float64) CompareResult {
	e := strings.Fields(string(expected))
	a := strings.Fields(string(actual))
	if len(e) != len(a) {
		return CompareResult{Message: fmt.Sprintf("输出 token 数不一致（标准 %d，实际 %d）", len(e), len(a))}
	}
	for i := range e {
		if e[i] == a[i] {
			continue
		}
		ef, errE := strconv.ParseFloat(e[i], 64)
		af, errA := strconv.ParseFloat(a[i], 64)
		if math.IsNaN(ef) || math.IsInf(ef, 0) || math.IsNaN(af) || math.IsInf(af, 0) {
			return CompareResult{Message: fmt.Sprintf("第 %d 个数字不是有限数值", i+1)}
		}
		if errE != nil || errA != nil {
			return CompareResult{Message: fmt.Sprintf("第 %d 个 token 不一致", i+1)}
		}
		diff := ef - af
		if diff < 0 {
			diff = -diff
		}
		if diff > eps {
			return CompareResult{Message: fmt.Sprintf("第 %d 个数字超出误差范围", i+1)}
		}
	}
	return CompareResult{Accepted: true}
}

func normalizeLines(b []byte) []string {
	raw := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	out := make([]string, 0, len(raw))
	for _, line := range raw {
		out = append(out, strings.TrimRight(line, " \t"))
	}
	for len(out) > 0 && out[len(out)-1] == "" {
		out = out[:len(out)-1]
	}
	return out
}
