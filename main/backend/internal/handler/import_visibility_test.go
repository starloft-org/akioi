package handler

import (
	"testing"

	"akioi/internal/model"
	"akioi/internal/service"
)

func importedVisibility(role string, pkgVisibility int, owner importOwner) int {
	prep := &preparedProblem{pkg: &service.ImportedProblem{Config: service.ImportConfig{Title: "t", Visibility: pkgVisibility}}}
	u := &model.User{ID: 1, Role: role}
	p := buildImportedProblem(prep, u, &model.ProblemBank{ID: 1}, "A0000", owner)
	return p.Visibility
}

func TestBuildImportedProblemHidesOnlyForAdmin(t *testing.T) {
	personal := importOwner{OwnerID: 2, Access: model.ProblemAccessMembers}
	public := importOwnerPublic(0)

	if got := importedVisibility(model.RoleUser, model.ProblemVisibilityHidden, personal); got != model.ProblemVisibilityPublic {
		t.Fatalf("普通用户导入隐藏包应强制公开，得到 %d", got)
	}
	if got := importedVisibility(model.RoleUser, model.ProblemVisibilityPublic, importOwnerPublic(model.ProblemVisibilityHidden)); got != model.ProblemVisibilityPublic {
		t.Fatalf("普通用户选了隐藏也应强制公开，得到 %d", got)
	}
	if got := importedVisibility(model.RoleSuperAdmin, model.ProblemVisibilityHidden, public); got != model.ProblemVisibilityHidden {
		t.Fatalf("管理员导入隐藏包应保持隐藏，得到 %d", got)
	}
	if got := importedVisibility(model.RoleAdmin, model.ProblemVisibilityPublic, importOwnerPublic(model.ProblemVisibilityHidden)); got != model.ProblemVisibilityHidden {
		t.Fatalf("管理员选择隐藏应生效，得到 %d", got)
	}
	if got := importedVisibility(model.RoleSuperAdmin, model.ProblemVisibilityPublic, public); got != model.ProblemVisibilityPublic {
		t.Fatalf("管理员未选隐藏应为公开，得到 %d", got)
	}
}
