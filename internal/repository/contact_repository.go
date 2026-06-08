package repository

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"wwlocal-wework/internal/model"
)

type ContactRepository struct {
	DB *gorm.DB
}

func NewContactRepository(db *gorm.DB) *ContactRepository {
	return &ContactRepository{DB: db}
}

func (r *ContactRepository) AutoMigrate() error {
	return r.DB.AutoMigrate(&model.Contact{}, &model.Department{}, &model.ContactDepartment{})
}

func (r *ContactRepository) BatchUpsertContacts(contacts []model.Contact) error {
	if len(contacts) == 0 {
		return nil
	}
	for i := 0; i < len(contacts); i += 100 {
		end := i + 100
		if end > len(contacts) {
			end = len(contacts)
		}
		batch := contacts[i:end]
		if err := r.DB.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"name":       gorm.Expr("VALUES(name)"),
				"mobile":     gorm.Expr("VALUES(mobile)"),
				"gender":     gorm.Expr("VALUES(gender)"),
				"email":      gorm.Expr("VALUES(email)"),
				"position":   gorm.Expr("VALUES(position)"),
				"department": gorm.Expr("VALUES(department)"),
				"positions":  gorm.Expr("VALUES(positions)"),
				"avatar":     gorm.Expr("VALUES(avatar)"),
				"status":     gorm.Expr("VALUES(status)"),
				"raw_json":   gorm.Expr("VALUES(raw_json)"),
				"synced_at":  gorm.Expr("VALUES(synced_at)"),
			}),
		}).CreateInBatches(batch, 100).Error; err != nil {
			return err
		}
		// 同时更新 contact_departments 中间表
		for _, contact := range batch {
			// 先删除旧的关联
			r.DB.Where("user_id = ?", contact.UserID).Delete(&model.ContactDepartment{})
			// 解析 department 字段为 int 数组
			var deptIDs []int
			if err := json.Unmarshal([]byte(contact.Department), &deptIDs); err == nil {
				var deptLinks []model.ContactDepartment
				for _, deptID := range deptIDs {
					deptLinks = append(deptLinks, model.ContactDepartment{
						UserID:     contact.UserID,
						Department: deptID,
					})
				}
				if len(deptLinks) > 0 {
					if err := r.DB.Clauses(clause.OnConflict{
						Columns:   []clause.Column{{Name: "user_id"}, {Name: "department"}},
						DoNothing: true,
					}).CreateInBatches(deptLinks, 100).Error; err != nil {
						slog.Info(fmt.Sprintf("update contact_departments for %s failed: %v", contact.UserID, err))
					}
				}
			}
		}
	}
	return nil
}

// BatchUpdateBasicInfo 只更新已有用户的基础字段（姓名、部门、同步时间），不覆盖 mobile/email/avatar 等详情
func (r *ContactRepository) BatchUpdateBasicInfo(contacts []model.Contact) error {
	if len(contacts) == 0 {
		return nil
	}
	for i := 0; i < len(contacts); i += 100 {
		end := i + 100
		if end > len(contacts) {
			end = len(contacts)
		}
		batch := contacts[i:end]

		sql := "INSERT INTO contacts (user_id, name, department, synced_at) VALUES "
		var args []interface{}
		for j, c := range batch {
			if j > 0 {
				sql += ","
			}
			sql += "(?,?,?,?)"
			args = append(args, c.UserID, c.Name, c.Department, c.SyncedAt)
		}
		sql += " ON DUPLICATE KEY UPDATE name=VALUES(name), department=VALUES(department), synced_at=VALUES(synced_at)"
		if err := r.DB.Exec(sql, args...).Error; err != nil {
			return err
		}
	}
	return nil
}

func (r *ContactRepository) BatchUpsertDepts(depts []model.Department) error {
	if len(depts) == 0 {
		return nil
	}
	return r.DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"name":      gorm.Expr("VALUES(name)"),
			"parent_id": gorm.Expr("VALUES(parent_id)"),
			"order_num": gorm.Expr("VALUES(order_num)"),
			"type":      gorm.Expr("VALUES(type)"),
			"synced_at": gorm.Expr("VALUES(synced_at)"),
		}),
	}).CreateInBatches(depts, 100).Error
}

func (r *ContactRepository) GetAllUserIDs() (map[string]bool, error) {
	var ids []string
	if err := r.DB.Model(&model.Contact{}).Pluck("user_id", &ids).Error; err != nil {
		return nil, err
	}
	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set, nil
}

func (r *ContactRepository) QueryContacts(name, mobile string, page, pageSize int) ([]model.Contact, int64, error) {
	query := r.DB.Model(&model.Contact{})
	if name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	if mobile != "" {
		query = query.Where("mobile = ?", mobile)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var contacts []model.Contact
	offset := (page - 1) * pageSize
	if err := query.Order("name ASC").Offset(offset).Limit(pageSize).Find(&contacts).Error; err != nil {
		return nil, 0, err
	}
	return contacts, total, nil
}

func (r *ContactRepository) GetContactByMobile(mobile string) (*model.Contact, error) {
	var c model.Contact
	if err := r.DB.Where("mobile = ?", mobile).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ContactRepository) GetAllDepartments() ([]model.Department, error) {
	var depts []model.Department
	if err := r.DB.Order("id ASC").Find(&depts).Error; err != nil {
		return nil, err
	}
	return depts, nil
}

func (r *ContactRepository) ExpandDepartmentIDs(rootIDs []int) ([]int, error) {
	if len(rootIDs) == 0 {
		return []int{}, nil
	}
	depts, err := r.GetAllDepartments()
	if err != nil {
		return nil, err
	}
	return ExpandDepartmentIDsFromList(depts, rootIDs), nil
}

func ExpandDepartmentIDsFromList(depts []model.Department, rootIDs []int) []int {
	children := make(map[int][]int, len(depts))
	for _, d := range depts {
		children[d.ParentID] = append(children[d.ParentID], d.ID)
	}
	seen := make(map[int]bool)
	var walk func(int)
	walk = func(id int) {
		if id <= 0 || seen[id] {
			return
		}
		seen[id] = true
		for _, child := range children[id] {
			walk(child)
		}
	}
	for _, id := range rootIDs {
		walk(id)
	}
	result := make([]int, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	sort.Ints(result)
	return result
}

func (r *ContactRepository) GetScopedContactCount(deptIDs []int, unrestricted bool) (int64, error) {
	var count int64
	if unrestricted {
		err := r.DB.Model(&model.Contact{}).
			Where("status = 1 AND mobile IS NOT NULL AND mobile != ''").
			Count(&count).Error
		return count, err
	}
	if len(deptIDs) == 0 {
		return 0, nil
	}
	err := r.DB.Raw(`
		SELECT COUNT(DISTINCT c.mobile)
		FROM contacts c
		INNER JOIN contact_departments cd ON c.user_id = cd.user_id
		WHERE c.status = 1 AND c.mobile IS NOT NULL AND c.mobile != ''
		  AND cd.department IN ?
	`, deptIDs).Scan(&count).Error
	return count, err
}

func (r *ContactRepository) IsIdentifierInScope(identifier string, deptIDs []int, unrestricted bool) (bool, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return false, nil
	}
	if unrestricted {
		return true, nil
	}
	if len(deptIDs) == 0 {
		return false, nil
	}

	var count int64
	err := r.DB.Raw(`
		SELECT COUNT(DISTINCT c.user_id)
		FROM contacts c
		INNER JOIN contact_departments cd ON c.user_id = cd.user_id
		WHERE cd.department IN ?
		  AND (c.user_id = ? OR c.mobile = ?)
	`, deptIDs, identifier, identifier).Scan(&count).Error
	return count > 0, err
}

func (r *ContactRepository) MarkSyncedAt(userIDs []string) error {
	if len(userIDs) == 0 {
		return nil
	}
	return r.DB.Model(&model.Contact{}).Where("user_id IN ?", userIDs).Update("synced_at", time.Now()).Error
}

func (r *ContactRepository) GetContactsByDepartmentID(deptID int, page, pageSize int) ([]model.Contact, int64, error) {
	query := r.DB.Model(&model.Contact{}).
		Joins("INNER JOIN contact_departments cd ON contacts.user_id = cd.user_id").
		Where("cd.department = ?", deptID)

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var contacts []model.Contact
	offset := (page - 1) * pageSize
	if err := query.Order("contacts.name ASC").Offset(offset).Limit(pageSize).Find(&contacts).Error; err != nil {
		return nil, 0, err
	}
	return contacts, total, nil
}

func (r *ContactRepository) GetNamesByUserIDs(userIDs []string) (map[string]string, error) {
	ids := uniqueStrings(userIDs)
	if len(ids) == 0 {
		return nil, nil
	}
	type result struct {
		UserID string
		Mobile string
		Name   string
	}
	var rows []result
	if err := r.DB.Model(&model.Contact{}).Select("user_id, mobile, name").Where("user_id IN ? OR mobile IN ?", ids, ids).Find(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[string]string, len(rows))
	for _, row := range rows {
		if row.Name == "" {
			continue
		}
		if row.UserID != "" {
			m[row.UserID] = row.Name
		}
		if row.Mobile != "" {
			m[row.Mobile] = row.Name
		}
	}
	return m, nil
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		result = append(result, value)
	}
	return result
}

func (r *ContactRepository) GetContactByUserID(userID string) (*model.Contact, error) {
	var c model.Contact
	if err := r.DB.Where("user_id = ?", userID).First(&c).Error; err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *ContactRepository) GetTotalContacts() (int64, error) {
	var count int64
	err := r.DB.Model(&model.Contact{}).Count(&count).Error
	return count, err
}

func (r *ContactRepository) GetLastSyncTime() (*time.Time, error) {
	var result *time.Time
	err := r.DB.Model(&model.Contact{}).Select("MAX(synced_at)").Scan(&result).Error
	return result, err
}

func (r *ContactRepository) GetCountByDeptID(deptID int) (int64, error) {
	var count int64
	if deptID > 0 {
		err := r.DB.Model(&model.ContactDepartment{}).Where("department = ?", deptID).Count(&count).Error
		return count, err
	}
	err := r.DB.Model(&model.Contact{}).Count(&count).Error
	return count, err
}

type DeptMemberCount struct {
	DeptID int
	Count  int64
}

func (r *ContactRepository) GetDeptMemberCounts() ([]DeptMemberCount, error) {
	var results []DeptMemberCount
	err := r.DB.Raw(`
		SELECT d.id AS dept_id, COUNT(cd.user_id) AS count
		FROM departments d
		LEFT JOIN contact_departments cd ON d.id = cd.department
		INNER JOIN contacts c ON cd.user_id = c.user_id AND c.status = 1
		GROUP BY d.id
	`).Scan(&results).Error
	return results, err
}

func (r *ContactRepository) GetMemberCountByDepartmentIDs(deptIDs []int) (map[int]int, error) {
	counts := make(map[int]int, len(deptIDs))
	for _, id := range deptIDs {
		counts[id] = 0
	}
	if len(deptIDs) == 0 {
		return counts, nil
	}

	placeholders := make([]string, len(deptIDs))
	args := make([]interface{}, len(deptIDs))
	for i, id := range deptIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	var results []struct {
		DeptID int
		Count  int64
	}
	sql := fmt.Sprintf(`
		SELECT cd.department AS dept_id, COUNT(cd.user_id) AS count
		FROM contact_departments cd
		INNER JOIN contacts c ON cd.user_id = c.user_id
		WHERE cd.department IN (%s) AND c.status = 1
		GROUP BY cd.department
	`, strings.Join(placeholders, ","))
	err := r.DB.Raw(sql, args...).Scan(&results).Error
	if err != nil {
		return nil, err
	}
	for _, r := range results {
		counts[r.DeptID] = int(r.Count)
	}
	return counts, nil
}

func BuildDeptTree(depts []model.Department, memberCounts map[int]int) []model.DeptTreeNode {
	// 用指针构建完整树
	type ptrNode struct {
		node     *model.DeptTreeNode
		children []*ptrNode
	}
	nodeMap := make(map[int]*ptrNode, len(depts))
	var roots []*ptrNode

	for _, d := range depts {
		pn := &ptrNode{
			node: &model.DeptTreeNode{
				ID:          d.ID,
				Name:        d.Name,
				ParentID:    d.ParentID,
				Order:       d.Order,
				Type:        d.Type,
				MemberCount: memberCounts[d.ID],
			},
		}
		nodeMap[d.ID] = pn
	}

	for _, pn := range nodeMap {
		if pn.node.ParentID == 0 {
			roots = append(roots, pn)
		} else if parent, ok := nodeMap[pn.node.ParentID]; ok {
			parent.children = append(parent.children, pn)
		} else {
			roots = append(roots, pn)
		}
	}

	// 指针树 → 值树（递归转换）
	var toValue func([]*ptrNode) []model.DeptTreeNode
	toValue = func(children []*ptrNode) []model.DeptTreeNode {
		if len(children) == 0 {
			return []model.DeptTreeNode{}
		}
		sort.Slice(children, func(i, j int) bool {
			return children[i].node.Order < children[j].node.Order
		})
		result := make([]model.DeptTreeNode, len(children))
		for i, c := range children {
			result[i] = *c.node
			result[i].Children = toValue(c.children)
		}
		return result
	}

	result := toValue(roots)
	return result
}

// SimpleUserToContact 将 SimpleUser 转为 Contact（仅基础字段）
func SimpleUserToContact(u model.SimpleUser) model.Contact {
	deptBytes, _ := json.Marshal(u.Department)
	return model.Contact{
		UserID:     u.UserID,
		Name:       u.Name,
		Department: string(deptBytes),
		SyncedAt:   time.Now(),
	}
}

// DetailToContact 将 ContactDetail 转为 Contact（含完整字段）
func DetailToContact(d model.ContactDetail, rawJSON string) model.Contact {
	deptBytes, _ := json.Marshal(d.Department)
	posBytes, _ := json.Marshal(d.Positions)
	gender, _ := strconv.Atoi(d.Gender)
	return model.Contact{
		UserID:     d.UserID,
		Name:       d.Name,
		Mobile:     d.Mobile,
		Gender:     gender,
		Email:      d.Email,
		Position:   d.Position,
		Department: string(deptBytes),
		Positions:  string(posBytes),
		Avatar:     d.Avatar,
		Status:     d.Status,
		RawJSON:    rawJSON,
		SyncedAt:   time.Now(),
	}
}

// DeptItemToDepartment 将 DepartmentItem 转为 Department
func DeptItemToDepartment(d model.DepartmentItem) model.Department {
	return model.Department{
		ID:       d.ID,
		Name:     d.Name,
		ParentID: d.ParentID,
		Order:    d.Order,
		Type:     d.Type,
		SyncedAt: time.Now(),
	}
}

// FilterNewUserIDs 返回 inAPI 中不在 existing 里的 userid
func FilterNewUserIDs(inAPI []string, existing map[string]bool) []string {
	var news []string
	for _, id := range inAPI {
		if !existing[id] {
			news = append(news, id)
		}
	}
	return news
}

// FilterMissingUserIDs 返回 existing 中不在 inAPI 里的 userid
func FilterMissingUserIDs(inAPI []string, existing map[string]bool) []string {
	apiSet := make(map[string]bool, len(inAPI))
	for _, id := range inAPI {
		apiSet[id] = true
	}
	var missing []string
	for id := range existing {
		if !apiSet[id] {
			missing = append(missing, id)
		}
	}
	return missing
}

// ExtractMobileFromRawJSON 从 API 原始 JSON 中提取 mobile 字段
func ExtractMobileFromRawJSON(rawJSON string) string {
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(rawJSON), &data); err != nil {
		return ""
	}
	if m, ok := data["mobile"].(string); ok {
		return strings.TrimSpace(m)
	}
	return ""
}
