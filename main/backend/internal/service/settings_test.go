package service

import "testing"

func TestNormalizeRuntimeSettings(t *testing.T) {
	settings := DefaultRuntimeSettings()
	settings.AppName = "  测试站  "
	settings.JudgeNodeEndpoints = []string{" https://judge.example.com/ ", "https://judge.example.com"}
	settings.CORSAllowOrigins = []string{"*", "https://web.example.com/"}

	got, err := NormalizeRuntimeSettings(settings)
	if err != nil {
		t.Fatal(err)
	}
	if got.AppName != "测试站" {
		t.Fatalf("站点名称未规范化: %q", got.AppName)
	}
	if len(got.JudgeNodeEndpoints) != 1 || got.JudgeNodeEndpoints[0] != "https://judge.example.com" {
		t.Fatalf("评测节点未去重: %v", got.JudgeNodeEndpoints)
	}
	if len(got.CORSAllowOrigins) != 2 {
		t.Fatalf("跨域来源未保留: %v", got.CORSAllowOrigins)
	}
}

func TestNormalizeRuntimeSettingsRejectsInvalidValues(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*RuntimeSettings)
	}{
		{"empty app name", func(s *RuntimeSettings) { s.AppName = "" }},
		{"short sms interval", func(s *RuntimeSettings) { s.SMSSendIntervalSec = 10 }},
		{"bad node url", func(s *RuntimeSettings) { s.JudgeNodeEndpoints = []string{"judge:9000"} }},
		{"bad cors origin", func(s *RuntimeSettings) { s.CORSAllowOrigins = []string{"example.com"} }},
		{"sms without templates", func(s *RuntimeSettings) { s.SMSEnabled = true }},
		{"wechat without redirect", func(s *RuntimeSettings) { s.WeChatEnabled = true }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			settings := DefaultRuntimeSettings()
			tt.mutate(&settings)
			if _, err := NormalizeRuntimeSettings(settings); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestStaticSettingServiceReturnsSliceCopies(t *testing.T) {
	settings := DefaultRuntimeSettings()
	settings.CORSAllowOrigins = []string{"https://example.com"}
	svc := NewStaticSettingService(settings)

	first := svc.Current()
	first.CORSAllowOrigins[0] = "changed"
	if svc.Current().CORSAllowOrigins[0] != "https://example.com" {
		t.Fatal("Current must not expose mutable slices")
	}
}
