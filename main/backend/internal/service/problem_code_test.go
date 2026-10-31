package service

import (
	"testing"

	"akioi/internal/model"
)

func TestFormatProblemDisplayID(t *testing.T) {
	bank := &model.ProblemBank{Prefix: "A", CodeLength: 4}
	cases := map[int]string{
		0:     "A0000",
		2:     "A0002",
		99:    "A0099",
		12345: "A12345",
	}
	for seq, want := range cases {
		if got := FormatProblemDisplayID(bank, seq); got != want {
			t.Fatalf("FormatProblemDisplayID(%d) = %q, want %q", seq, got, want)
		}
	}

	lower := &model.ProblemBank{Prefix: "ab", CodeLength: 2}
	if got := FormatProblemDisplayID(lower, 7); got != "ab07" {
		t.Fatalf("FormatProblemDisplayID(7) = %q, want %q", got, "ab07")
	}
}

func TestProblemDisplayIDSeq(t *testing.T) {
	cases := []struct {
		id     string
		prefix string
		want   int
		ok     bool
	}{
		{"A0000", "A", 0, true},
		{"A0002", "A", 2, true},
		{"A12345", "A", 12345, true},
		{"B0001", "A", 0, false},
		{"A", "A", 0, false},
		{"A00x1", "A", 0, false},
		{"A-1", "A", 0, false},
	}
	for _, c := range cases {
		got, ok := ProblemDisplayIDSeq(c.id, c.prefix)
		if ok != c.ok || (ok && got != c.want) {
			t.Fatalf("ProblemDisplayIDSeq(%q, %q) = (%d, %v), want (%d, %v)", c.id, c.prefix, got, ok, c.want, c.ok)
		}
	}
}

func TestValidProblemDisplayID(t *testing.T) {
	bank := &model.ProblemBank{Prefix: "A", CodeLength: 4}
	if !ValidProblemDisplayID(bank, "A0000") {
		t.Fatal("A0000 should be valid")
	}
	if ValidProblemDisplayID(bank, "A001") {
		t.Fatal("A001 is shorter than the code length")
	}
	if ValidProblemDisplayID(bank, "B0000") {
		t.Fatal("B0000 does not match the bank prefix")
	}
}

func TestRenumberProblemDisplayIDs(t *testing.T) {
	cases := []struct {
		name      string
		bank      *model.ProblemBank
		oldPrefix string
		current   []string
		want      []string
	}{
		{
			name:      "换前缀保留序号",
			bank:      &model.ProblemBank{Prefix: "B", CodeLength: 4},
			oldPrefix: "A",
			current:   []string{"A0000", "A0002", "A0010"},
			want:      []string{"B0000", "B0002", "B0010"},
		},
		{
			name:      "加长补零位数",
			bank:      &model.ProblemBank{Prefix: "A", CodeLength: 6},
			oldPrefix: "A",
			current:   []string{"A0000", "A0001"},
			want:      []string{"A000000", "A000001"},
		},
		{
			name:      "缩短补零位数",
			bank:      &model.ProblemBank{Prefix: "A", CodeLength: 4},
			oldPrefix: "A",
			current:   []string{"A000005", "A000012"},
			want:      []string{"A0005", "A0012"},
		},
		{
			name:      "无法解析的题号顺延",
			bank:      &model.ProblemBank{Prefix: "A", CodeLength: 4},
			oldPrefix: "A",
			current:   []string{"A0000", "手工题号"},
			want:      []string{"A0000", "A0001"},
		},
		{
			name:      "序号重复时顺延到空闲位",
			bank:      &model.ProblemBank{Prefix: "A", CodeLength: 4},
			oldPrefix: "A",
			current:   []string{"A0001", "A00001"},
			want:      []string{"A0001", "A0002"},
		},
	}
	for _, c := range cases {
		got := RenumberProblemDisplayIDs(c.bank, c.oldPrefix, c.current)
		if len(got) != len(c.want) {
			t.Fatalf("%s: got %v, want %v", c.name, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("%s: got %v, want %v", c.name, got, c.want)
			}
		}
	}
}
