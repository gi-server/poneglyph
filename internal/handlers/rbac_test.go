package handlers

import (
	"testing"
)

func TestRoleRank(t *testing.T) {
	tests := []struct {
		role string
		want int
	}{
		{"developer", 3},
		{"admin", 2},
		{"worker", 1},
		{"unknown", 0},
		{"", 0},
	}
	for _, tt := range tests {
		got := roleRank(tt.role)
		if got != tt.want {
			t.Errorf("roleRank(%q) = %d, want %d", tt.role, got, tt.want)
		}
	}
}

func TestRoleHierarchy(t *testing.T) {
	// developer can create admin and worker
	if !(roleRank("admin") < roleRank("developer")) {
		t.Error("developer should outrank admin")
	}
	if !(roleRank("worker") < roleRank("admin")) {
		t.Error("admin should outrank worker")
	}
	// admin cannot create admin or developer
	if roleRank("admin") < roleRank("admin") {
		t.Error("admin should not be able to create another admin (equal rank)")
	}
	if roleRank("developer") < roleRank("admin") {
		t.Error("admin should not be able to create developer (higher rank)")
	}
}

func TestValidRoles(t *testing.T) {
	valid := []string{"worker", "admin", "developer"}
	for _, r := range valid {
		if !validRoles[r] {
			t.Errorf("expected role %q to be valid", r)
		}
	}
	invalid := []string{"user", "superadmin", "root", ""}
	for _, r := range invalid {
		if validRoles[r] {
			t.Errorf("expected role %q to be invalid", r)
		}
	}
}
