package service

import (
	"errors"
	"reflect"
	"testing"

	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
)

type fakeUserRepo struct {
	users  map[int64]*model.User
	scopes map[int64][]int
}

func (r *fakeUserRepo) Count() (int64, error) { return int64(len(r.users)), nil }
func (r *fakeUserRepo) Create(user *model.User) error {
	r.users[user.ID] = user
	return nil
}
func (r *fakeUserRepo) GetByUsername(username string) (*model.User, error) {
	for _, user := range r.users {
		if user.Username == username {
			return user, nil
		}
	}
	return nil, errors.New("not found")
}
func (r *fakeUserRepo) GetByID(id int64) (*model.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return user, nil
}
func (r *fakeUserRepo) List() ([]model.User, error) { return nil, nil }
func (r *fakeUserRepo) Update(user *model.User) error {
	r.users[user.ID] = user
	return nil
}
func (r *fakeUserRepo) SetScopes(userID int64, deptIDs []int) error {
	r.scopes[userID] = deptIDs
	return nil
}
func (r *fakeUserRepo) GetScopes(userID int64) ([]int, error) {
	return r.scopes[userID], nil
}

type fakeContactScopeRepo struct {
	expanded       []int
	allowed        map[string]bool
	lastDeptIDs    []int
	lastUnlimited  bool
	lastIdentifier string
}

func (r *fakeContactScopeRepo) ExpandDepartmentIDs(rootIDs []int) ([]int, error) {
	return r.expanded, nil
}
func (r *fakeContactScopeRepo) IsIdentifierInScope(identifier string, deptIDs []int, unrestricted bool) (bool, error) {
	r.lastIdentifier = identifier
	r.lastDeptIDs = append([]int(nil), deptIDs...)
	r.lastUnlimited = unrestricted
	if unrestricted {
		return true, nil
	}
	return r.allowed[identifier], nil
}

func TestResolveDataScopeSuperAdmin(t *testing.T) {
	userRepo := &fakeUserRepo{
		users: map[int64]*model.User{
			1: {ID: 1, Username: "admin", Role: model.RoleSuperAdmin},
		},
		scopes: map[int64][]int{},
	}
	contactRepo := &fakeContactScopeRepo{expanded: []int{2, 3}}
	svc := NewUserService(userRepo, contactRepo, &config.AuthConfig{})

	scope, err := svc.ResolveDataScope(1)
	if err != nil {
		t.Fatalf("ResolveDataScope: %v", err)
	}
	if !scope.Unrestricted {
		t.Fatalf("scope.Unrestricted = false, want true")
	}
	if len(scope.DeptIDs) != 0 {
		t.Fatalf("DeptIDs = %v, want empty", scope.DeptIDs)
	}
}

func TestResolveDataScopeDeptAdminExpandsDepartments(t *testing.T) {
	userRepo := &fakeUserRepo{
		users: map[int64]*model.User{
			2: {ID: 2, Username: "dept", Role: model.RoleDeptAdmin},
		},
		scopes: map[int64][]int{2: {10}},
	}
	contactRepo := &fakeContactScopeRepo{expanded: []int{10, 11, 12}}
	svc := NewUserService(userRepo, contactRepo, &config.AuthConfig{})

	scope, err := svc.ResolveDataScope(2)
	if err != nil {
		t.Fatalf("ResolveDataScope: %v", err)
	}
	if scope.Unrestricted {
		t.Fatalf("scope.Unrestricted = true, want false")
	}
	if !reflect.DeepEqual(scope.DeptIDs, []int{10, 11, 12}) {
		t.Fatalf("DeptIDs = %v, want [10 11 12]", scope.DeptIDs)
	}
}

func TestIdentifierInDataScopeUsesResolvedScope(t *testing.T) {
	userRepo := &fakeUserRepo{
		users: map[int64]*model.User{
			2: {ID: 2, Username: "dept", Role: model.RoleDeptAdmin},
		},
		scopes: map[int64][]int{2: {10}},
	}
	contactRepo := &fakeContactScopeRepo{
		expanded: []int{10, 11},
		allowed:  map[string]bool{"u1": true},
	}
	svc := NewUserService(userRepo, contactRepo, &config.AuthConfig{})

	scope, ok, err := svc.IdentifierInDataScope(2, "u1")
	if err != nil {
		t.Fatalf("IdentifierInDataScope: %v", err)
	}
	if !ok {
		t.Fatalf("ok = false, want true")
	}
	if scope.Unrestricted {
		t.Fatalf("scope.Unrestricted = true, want false")
	}
	if !reflect.DeepEqual(contactRepo.lastDeptIDs, []int{10, 11}) {
		t.Fatalf("lastDeptIDs = %v, want [10 11]", contactRepo.lastDeptIDs)
	}
	if contactRepo.lastIdentifier != "u1" {
		t.Fatalf("lastIdentifier = %q, want u1", contactRepo.lastIdentifier)
	}
}

func TestIdentifierInDataScopeSuperAdminUsesUnrestrictedScope(t *testing.T) {
	userRepo := &fakeUserRepo{
		users: map[int64]*model.User{
			1: {ID: 1, Username: "admin", Role: model.RoleSuperAdmin},
		},
		scopes: map[int64][]int{},
	}
	contactRepo := &fakeContactScopeRepo{allowed: map[string]bool{}}
	svc := NewUserService(userRepo, contactRepo, &config.AuthConfig{})

	scope, _, err := svc.IdentifierInDataScope(1, "anyone")
	if err != nil {
		t.Fatalf("IdentifierInDataScope: %v", err)
	}
	if !scope.Unrestricted {
		t.Fatalf("scope.Unrestricted = false, want true")
	}
	if !contactRepo.lastUnlimited {
		t.Fatalf("lastUnlimited = false, want true")
	}
	if len(contactRepo.lastDeptIDs) != 0 {
		t.Fatalf("lastDeptIDs = %v, want empty", contactRepo.lastDeptIDs)
	}
}
