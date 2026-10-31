package model

import "testing"

// TestNormalizeAdminPermissions 覆盖去重、排序与丢弃未知权限项。
func TestNormalizeAdminPermissions(t *testing.T) {
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{name: "空列表", in: nil, want: []string{}},
		{name: "按固定顺序整理", in: []string{AdminPermissionLogs, AdminPermissionUsers}, want: []string{AdminPermissionUsers, AdminPermissionLogs}},
		{name: "去重", in: []string{AdminPermissionProblems, AdminPermissionProblems}, want: []string{AdminPermissionProblems}},
		{name: "丢弃未知项与空白", in: []string{" ", "unknown", AdminPermissionJudges}, want: []string{AdminPermissionJudges}},
		{name: "全部权限", in: AllAdminPermissions(), want: AllAdminPermissions()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := NormalizeAdminPermissions(tc.in)
			if len(got) != len(tc.want) {
				t.Fatalf("权限项数量不符：want %v, got %v", tc.want, got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("权限项顺序不符：want %v, got %v", tc.want, got)
				}
			}
		})
	}
}

// TestUserPermissions 验证不同角色的权限来源与判定。
func TestUserPermissions(t *testing.T) {
	user := &User{Role: RoleUser}
	if len(user.Permissions()) != 0 || user.HasAdminPermission(AdminPermissionProblems) {
		t.Fatalf("普通用户不应拥有后台权限")
	}

	admin := &User{Role: RoleAdmin}
	admin.SetPermissions([]string{AdminPermissionProblems, AdminPermissionProblemBanks, "unknown"})
	if admin.AdminPermissions != AdminPermissionProblems+","+AdminPermissionProblemBanks {
		t.Fatalf("权限序列化结果不符：%s", admin.AdminPermissions)
	}
	if !admin.HasAdminPermission(AdminPermissionProblems) {
		t.Fatalf("管理员应拥有已分配的权限")
	}
	if admin.HasAdminPermission(AdminPermissionSettings) {
		t.Fatalf("管理员不应拥有未分配的权限")
	}
	if !admin.IsAdmin() || admin.IsSuperAdmin() {
		t.Fatalf("管理员的角色判定不符")
	}

	superAdmin := &User{Role: RoleSuperAdmin}
	if !superAdmin.IsSuperAdmin() || !superAdmin.IsAdmin() {
		t.Fatalf("超级管理员的角色判定不符")
	}
	if len(superAdmin.Permissions()) != len(AllAdminPermissions()) {
		t.Fatalf("超级管理员应拥有全部权限")
	}
	if !superAdmin.HasAdminPermission(AdminPermissionSettings) {
		t.Fatalf("超级管理员应通过任意权限判定")
	}
}

// TestUnknownAdminPermissions 验证未知权限项的收集。
func TestUnknownAdminPermissions(t *testing.T) {
	unknown := UnknownAdminPermissions([]string{AdminPermissionUsers, "nope", " "})
	if len(unknown) != 1 || unknown[0] != "nope" {
		t.Fatalf("未知权限收集不符：%v", unknown)
	}
}
