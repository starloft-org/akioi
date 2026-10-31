package model

import "testing"

func TestProblemKind(t *testing.T) {
	cases := []struct {
		name    string
		problem Problem
		kind    string
		team    bool
	}{
		{"public", Problem{}, ProblemKindPublic, false},
		{"team", Problem{TeamID: 3}, ProblemKindTeam, true},
		{"personal", Problem{OwnerID: 7}, ProblemKindPersonal, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.problem.Kind(); got != c.kind {
				t.Fatalf("Kind() = %q, want %q", got, c.kind)
			}
			if got := c.problem.IsTeamProblem(); got != c.team {
				t.Fatalf("IsTeamProblem() = %v, want %v", got, c.team)
			}
			wantPersonal := c.kind == ProblemKindPersonal
			if got := c.problem.IsPersonalProblem(); got != wantPersonal {
				t.Fatalf("IsPersonalProblem() = %v, want %v", got, wantPersonal)
			}
		})
	}
}

func TestValidProblemAccess(t *testing.T) {
	cases := map[int]bool{
		ProblemAccessPublic:  true,
		ProblemAccessMembers: true,
		0:                    false,
		3:                    false,
	}
	for access, want := range cases {
		if got := ValidProblemAccess(access); got != want {
			t.Fatalf("ValidProblemAccess(%d) = %v, want %v", access, got, want)
		}
	}
}
