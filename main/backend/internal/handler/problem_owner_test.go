package handler

import (
	"testing"

	"akioi/internal/model"
)

func TestCanManageProblemPublicProblemAdminOnly(t *testing.T) {
	h := &Handler{}
	public := &model.Problem{ID: 1, Title: "公共题", BankID: 1}
	user := &model.User{ID: 2, Role: model.RoleUser}
	if h.canManageProblem(user, public) {
		t.Fatal("普通用户不应能管理公共题库的题目")
	}
	if !h.canManageProblem(&model.User{ID: 1, Role: model.RoleAdmin}, public) {
		t.Fatal("管理员应能管理公共题库的题目")
	}
	personal := &model.Problem{ID: 2, OwnerID: 2}
	if !h.canManageProblem(user, personal) {
		t.Fatal("创建者应能管理自己的个人题目")
	}
	if h.canManageProblem(&model.User{ID: 3, Role: model.RoleUser}, personal) {
		t.Fatal("其他用户不应能管理别人的个人题目")
	}
}
