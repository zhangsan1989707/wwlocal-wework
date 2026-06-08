package service

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
	"wwlocal-wework/config"
	"wwlocal-wework/internal/model"
	"wwlocal-wework/internal/repository"
)

type UserService struct {
	userRepo    *repository.UserRepository
	contactRepo *repository.ContactRepository
	authCfg     *config.AuthConfig
}

type AuthUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type UserWithScopes struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	Enabled  bool   `json:"enabled"`
	DeptIDs  []int  `json:"dept_ids"`
}

type DataScope struct {
	UserID       int64  `json:"user_id"`
	Username     string `json:"username"`
	Role         string `json:"role"`
	DeptIDs      []int  `json:"dept_ids"`
	Unrestricted bool   `json:"unrestricted"`
}

func NewUserService(userRepo *repository.UserRepository, contactRepo *repository.ContactRepository, authCfg *config.AuthConfig) *UserService {
	return &UserService{userRepo: userRepo, contactRepo: contactRepo, authCfg: authCfg}
}

func (s *UserService) EnsureInitialAdmin() error {
	count, err := s.userRepo.Count()
	if err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(s.authCfg.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.userRepo.Create(&model.User{
		Username:     s.authCfg.Username,
		PasswordHash: string(hash),
		Role:         model.RoleSuperAdmin,
		Enabled:      true,
	})
}

func (s *UserService) Authenticate(username, password string) (*AuthUser, error) {
	user, err := s.userRepo.GetByUsername(username)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	if !user.Enabled {
		return nil, fmt.Errorf("user disabled")
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	return &AuthUser{ID: user.ID, Username: user.Username, Role: user.Role}, nil
}

func (s *UserService) ChangePassword(userID int64, oldPassword, newPassword string) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(oldPassword)) != nil {
		return fmt.Errorf("旧密码不正确")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hash)
	return s.userRepo.Update(user)
}

func (s *UserService) ListUsers() ([]UserWithScopes, error) {
	users, err := s.userRepo.List()
	if err != nil {
		return nil, err
	}
	result := make([]UserWithScopes, 0, len(users))
	for _, user := range users {
		deptIDs, err := s.userRepo.GetScopes(user.ID)
		if err != nil {
			return nil, err
		}
		result = append(result, UserWithScopes{
			ID:       user.ID,
			Username: user.Username,
			Role:     user.Role,
			Enabled:  user.Enabled,
			DeptIDs:  deptIDs,
		})
	}
	return result, nil
}

func (s *UserService) CreateUser(username, password, role string, enabled bool, deptIDs []int) (*UserWithScopes, error) {
	if role != model.RoleSuperAdmin && role != model.RoleDeptAdmin {
		return nil, fmt.Errorf("invalid role")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	user := &model.User{Username: username, PasswordHash: string(hash), Role: role, Enabled: enabled}
	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}
	if role == model.RoleDeptAdmin {
		if err := s.userRepo.SetScopes(user.ID, deptIDs); err != nil {
			return nil, err
		}
	}
	scopes, _ := s.userRepo.GetScopes(user.ID)
	return &UserWithScopes{ID: user.ID, Username: user.Username, Role: user.Role, Enabled: user.Enabled, DeptIDs: scopes}, nil
}

func (s *UserService) UpdateUser(userID int64, role string, enabled bool, deptIDs []int) (*UserWithScopes, error) {
	if role != model.RoleSuperAdmin && role != model.RoleDeptAdmin {
		return nil, fmt.Errorf("invalid role")
	}
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	user.Role = role
	user.Enabled = enabled
	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}
	scopes := deptIDs
	if role == model.RoleSuperAdmin {
		scopes = nil
	}
	if err := s.userRepo.SetScopes(user.ID, scopes); err != nil {
		return nil, err
	}
	savedScopes, _ := s.userRepo.GetScopes(user.ID)
	return &UserWithScopes{ID: user.ID, Username: user.Username, Role: user.Role, Enabled: user.Enabled, DeptIDs: savedScopes}, nil
}

func (s *UserService) ResetPassword(userID int64, password string) error {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hash)
	return s.userRepo.Update(user)
}

func (s *UserService) ResolveDataScope(userID int64) (*DataScope, error) {
	user, err := s.userRepo.GetByID(userID)
	if err != nil {
		return nil, err
	}
	scope := &DataScope{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
	}
	if user.Role == model.RoleSuperAdmin {
		scope.Unrestricted = true
		return scope, nil
	}
	rootIDs, err := s.userRepo.GetScopes(user.ID)
	if err != nil {
		return nil, err
	}
	deptIDs, err := s.contactRepo.ExpandDepartmentIDs(rootIDs)
	if err != nil {
		return nil, err
	}
	scope.DeptIDs = deptIDs
	return scope, nil
}

func (s *UserService) IdentifierInDataScope(userID int64, identifier string) (*DataScope, bool, error) {
	scope, err := s.ResolveDataScope(userID)
	if err != nil {
		return nil, false, err
	}
	ok, err := s.contactRepo.IsIdentifierInScope(identifier, scope.DeptIDs, scope.Unrestricted)
	return scope, ok, err
}
