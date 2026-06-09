package repository

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"wwlocal-wework/internal/model"
)

type UserRepository struct {
	DB *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{DB: db}
}

func (r *UserRepository) AutoMigrate() error {
	return r.DB.AutoMigrate(&model.User{}, &model.UserDeptScope{})
}

func (r *UserRepository) Count() (int64, error) {
	var count int64
	err := r.DB.Model(&model.User{}).Count(&count).Error
	return count, err
}

func (r *UserRepository) Create(user *model.User) error {
	return r.DB.Create(user).Error
}

func (r *UserRepository) GetByUsername(username string) (*model.User, error) {
	var user model.User
	if err := r.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) GetByID(id int64) (*model.User, error) {
	var user model.User
	if err := r.DB.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) List() ([]model.User, error) {
	var users []model.User
	err := r.DB.Order("id ASC").Find(&users).Error
	return users, err
}

func (r *UserRepository) Update(user *model.User) error {
	return r.DB.Save(user).Error
}

func (r *UserRepository) SetScopes(userID int64, deptIDs []int) error {
	return r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", userID).Delete(&model.UserDeptScope{}).Error; err != nil {
			return err
		}
		if len(deptIDs) == 0 {
			return nil
		}
		scopes := make([]model.UserDeptScope, 0, len(deptIDs))
		seen := make(map[int]bool, len(deptIDs))
		for _, deptID := range deptIDs {
			if deptID <= 0 || seen[deptID] {
				continue
			}
			seen[deptID] = true
			scopes = append(scopes, model.UserDeptScope{UserID: userID, DeptID: deptID})
		}
		if len(scopes) == 0 {
			return nil
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&scopes).Error
	})
}

func (r *UserRepository) GetScopes(userID int64) ([]int, error) {
	var scopes []model.UserDeptScope
	if err := r.DB.Where("user_id = ?", userID).Order("dept_id ASC").Find(&scopes).Error; err != nil {
		return nil, err
	}
	deptIDs := make([]int, 0, len(scopes))
	for _, s := range scopes {
		deptIDs = append(deptIDs, s.DeptID)
	}
	return deptIDs, nil
}

func IsNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
