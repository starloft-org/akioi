package model

import "testing"

func TestValidRole(t *testing.T) {
	tests := []struct {
		role string
		want bool
	}{
		{role: RoleAdmin, want: true},
		{role: RoleUser, want: true},
		{role: "setter", want: false},
		{role: "", want: false},
	}

	for _, tt := range tests {
		if got := ValidRole(tt.role); got != tt.want {
			t.Errorf("ValidRole(%q) = %v, want %v", tt.role, got, tt.want)
		}
	}
}

func TestIsAdmin(t *testing.T) {
	if !(&User{Role: RoleAdmin}).IsAdmin() {
		t.Fatal("admin should have administrator permissions")
	}
	if (&User{Role: RoleUser}).IsAdmin() {
		t.Fatal("user should not have administrator permissions")
	}
	if (&User{Role: "setter"}).IsAdmin() {
		t.Fatal("legacy setter should not have administrator permissions")
	}
}
