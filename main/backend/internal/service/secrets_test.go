package service

import (
	"strings"
	"testing"
)

func TestSecretsMerge(t *testing.T) {
	base := Secrets{
		JWTSecret:       "jwt",
		WeChatAppID:     "wx",
		WeChatAppSecret: "wx-secret",
		SMSAPIKey:       "sms-key",
		SMSAPISecret:    "sms-secret",
		SMTPHost:        "smtp.example.com",
		SMTPPort:        465,
		SMTPPassword:    "smtp-pass",
		SMTPFrom:        "noreply@example.com",
		JudgeSyncSecret: "judge",
	}

	cases := []struct {
		name  string
		patch SecretsPatch
		want  func(t *testing.T, got Secrets)
	}{
		{
			name:  "空增量保持原值",
			patch: SecretsPatch{},
			want: func(t *testing.T, got Secrets) {
				if got.JWTSecret != "jwt" || got.SMSAPIKey != "sms-key" || got.SMTPPassword != "smtp-pass" {
					t.Fatalf("空增量不应改动密钥: %+v", got)
				}
			},
		},
		{
			name:  "密钥字段留空表示不修改",
			patch: SecretsPatch{WeChatAppSecret: "  ", SMSAPISecret: ""},
			want: func(t *testing.T, got Secrets) {
				if got.WeChatAppSecret != "wx-secret" || got.SMSAPISecret != "sms-secret" {
					t.Fatalf("留空不应清空密钥: %+v", got)
				}
			},
		},
		{
			name:  "提交新密钥覆盖旧值",
			patch: SecretsPatch{SMSAPIKey: " new-key ", SMTPPassword: "new-pass"},
			want: func(t *testing.T, got Secrets) {
				if got.SMSAPIKey != "new-key" || got.SMTPPassword != "new-pass" {
					t.Fatalf("新密钥未生效: %+v", got)
				}
			},
		},
		{
			name:  "非敏感字段可清空",
			patch: SecretsPatch{SMTPHost: "", SMTPFrom: "", SMTPPort: 0},
			want: func(t *testing.T, got Secrets) {
				if got.SMTPHost != "" || got.SMTPFrom != "" || got.SMTPPort != 0 {
					t.Fatalf("非敏感字段应能清空: %+v", got)
				}
			},
		},
		{
			name:  "清除分组",
			patch: SecretsPatch{Clear: []string{"wechat", "smtp"}},
			want: func(t *testing.T, got Secrets) {
				if got.WeChatAppID != "" || got.WeChatAppSecret != "" {
					t.Fatalf("微信密钥未清除: %+v", got)
				}
				if got.SMTPHost != "" || got.SMTPPassword != "" {
					t.Fatalf("邮件配置未清除: %+v", got)
				}
				if got.SMSAPIKey != "sms-key" {
					t.Fatalf("未指定的分组不应被清除: %+v", got)
				}
			},
		},
		{
			name:  "重新生成登录密钥",
			patch: SecretsPatch{RegenerateJWT: true},
			want: func(t *testing.T, got Secrets) {
				if got.JWTSecret == "jwt" || len(got.JWTSecret) != 64 {
					t.Fatalf("登录密钥应被替换为 32 字节随机值: %q", got.JWTSecret)
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := base.Merge(tc.patch)
			if err != nil {
				t.Fatalf("Merge error: %v", err)
			}
			tc.want(t, got)
		})
	}
}

func TestSecretsMergeUnknownClearGroup(t *testing.T) {
	if _, err := (Secrets{}).Merge(SecretsPatch{Clear: []string{"unknown"}}); err == nil {
		t.Fatal("未知清除分组应返回错误")
	}
}

func TestSecretsMergeFillsJWTSecret(t *testing.T) {
	got, err := Secrets{}.Merge(SecretsPatch{})
	if err != nil {
		t.Fatalf("Merge error: %v", err)
	}
	if len(got.JWTSecret) != 64 || strings.TrimSpace(got.JWTSecret) == "" {
		t.Fatalf("登录密钥为空时应自动生成: %q", got.JWTSecret)
	}
}

func TestSecretsStatus(t *testing.T) {
	status := Secrets{
		JWTSecret:       "jwt",
		WeChatAppID:     "wx",
		WeChatAppSecret: "secret",
		SMSAPIKey:       "key",
		SMTPHost:        "smtp.example.com",
		SMTPFrom:        "noreply@example.com",
		SMTPPassword:    "pass",
	}.Status()
	cases := map[string]bool{
		"jwt_secret_configured":          true,
		"wechat_credentials_configured":  true,
		"wechat_open_configured":         false,
		"sms_credentials_configured":     false,
		"smtp_credentials_configured":    true,
		"captcha_credentials_configured": true,
		"judge_sync_secret_configured":   false,
	}
	for key, want := range cases {
		if got := status[key]; got != want {
			t.Fatalf("status[%s] = %v, want %v", key, got, want)
		}
	}
}

func TestSecretsValidate(t *testing.T) {
	settings := DefaultRuntimeSettings()
	settings.SMSEnabled = true
	if err := (Secrets{}).Validate(settings); err == nil {
		t.Fatal("启用短信服务但缺少密钥时应返回错误")
	}
	secrets := Secrets{SMSAPIKey: "key", SMSAPISecret: "secret"}
	if err := secrets.Validate(settings); err != nil {
		t.Fatalf("凭据齐全时不应报错: %v", err)
	}
	settings.JudgeNodeEndpoints = []string{"http://127.0.0.1:9000"}
	if err := secrets.Validate(settings); err == nil {
		t.Fatal("配置评测节点但缺少同步密钥时应返回错误")
	}
}
