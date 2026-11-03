package config

import "testing"

func TestSplitList(t *testing.T) {
	got := splitList(" 127.0.0.1 , ,10.0.0.0/8, ::1 ")
	want := []string{"127.0.0.1", "10.0.0.0/8", "::1"}
	if len(got) != len(want) {
		t.Fatalf("期望 %v，得到 %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("期望 %v，得到 %v", want, got)
		}
	}
	if len(splitList("")) != 0 {
		t.Fatal("空配置应返回空列表")
	}
}
