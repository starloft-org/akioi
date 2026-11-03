package handler

import (
	"testing"

	"akioi/internal/model"
)

func TestMaskedPhone(t *testing.T) {
	u := &model.User{Phone: model.PhonePointer("13812345678")}
	if got := maskedPhone(u, true); got != "13812345678" {
		t.Fatalf("有用户管理权限时应看到完整手机号，得到 %q", got)
	}
	if got := maskedPhone(u, false); got != "138****5678" {
		t.Fatalf("隐藏手机号格式不正确：%q", got)
	}
	if got := maskedPhone(&model.User{}, false); got != "" {
		t.Fatalf("空手机号应原样返回，得到 %q", got)
	}
}
