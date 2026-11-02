package model

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{" 动态规划 ", "图论", "动态规划", "", "   ", "最短路"})
	want := []string{"动态规划", "图论", "最短路"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("NormalizeTags = %v, want %v", got, want)
	}

	long := strings.Repeat("长", MaxProblemTagRunes+5)
	got = NormalizeTags([]string{long})
	if len([]rune(got[0])) != MaxProblemTagRunes {
		t.Fatalf("tag should be truncated to %d runes, got %d", MaxProblemTagRunes, len([]rune(got[0])))
	}

	many := make([]string, 0, MaxProblemTags+10)
	for i := 0; i < MaxProblemTags+10; i++ {
		many = append(many, "t"+string(rune('a'+i%26)))
	}
	if n := len(NormalizeTags(many)); n != MaxProblemTags {
		t.Fatalf("tag count = %d, want %d", n, MaxProblemTags)
	}

	if got := NormalizeTags(nil); len(got) != 0 {
		t.Fatalf("nil tags should be empty, got %v", got)
	}
}
