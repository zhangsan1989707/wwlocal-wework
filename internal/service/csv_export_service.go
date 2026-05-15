package service

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"log"
	"strings"

	"wwlocal-wework/internal/model"
)

type CSVExportService struct{}

func NewCSVExportService() *CSVExportService {
	return &CSVExportService{}
}

type UserChange struct {
	UserID     string
	Name       string
	Department []int
	Position   string
	Mobile     string
	Email      string
	Telephone  string
	Status     int
}

func (s *CSVExportService) GenerateMemberCSV(changes []UserChange) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	header := []string{
		"userid", "name", "department", "position",
		"mobile", "email", "telephone", "enable",
	}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("write CSV header failed: %w", err)
	}

	for _, c := range changes {
		deptStr := ""
		if len(c.Department) > 0 {
			deptStr = intsToSemicolonString(c.Department)
		}

		enable := 1
		if c.Status == 5 {
			enable = 0
		}

		row := []string{
			c.UserID,
			c.Name,
			deptStr,
			c.Position,
			c.Mobile,
			c.Email,
			c.Telephone,
			fmt.Sprintf("%d", enable),
		}
		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("write CSV row failed: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush CSV failed: %w", err)
	}

	return buf.Bytes(), nil
}

func (s *CSVExportService) GenerateIncrementalCSV(newUsers, updatedUsers []UserChange) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	header := []string{
		"userid", "name", "department", "position",
		"mobile", "email", "telephone", "enable",
	}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("write CSV header failed: %w", err)
	}

	for _, c := range newUsers {
		if err := writer.Write(s.userChangeToRow(c)); err != nil {
			return nil, fmt.Errorf("write CSV row failed: %w", err)
		}
	}

	for _, c := range updatedUsers {
		if err := writer.Write(s.userChangeToRow(c)); err != nil {
			return nil, fmt.Errorf("write CSV row failed: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush CSV failed: %w", err)
	}

	return buf.Bytes(), nil
}

func (s *CSVExportService) userChangeToRow(c UserChange) []string {
	deptStr := ""
	if len(c.Department) > 0 {
		deptStr = intsToSemicolonString(c.Department)
	}

	enable := 1
	if c.Status == 5 {
		enable = 0
	}

	return []string{
		c.UserID,
		c.Name,
		deptStr,
		c.Position,
		c.Mobile,
		c.Email,
		c.Telephone,
		fmt.Sprintf("%d", enable),
	}
}

type DepartmentChange struct {
	PartyID  int
	Name     string
	ParentID int
	Order    int
}

func (s *CSVExportService) GenerateDepartmentCSV(changes []DepartmentChange) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	header := []string{"partyid", "name", "parentid", "order"}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("write CSV header failed: %w", err)
	}

	for _, c := range changes {
		row := []string{
			fmt.Sprintf("%d", c.PartyID),
			c.Name,
			fmt.Sprintf("%d", c.ParentID),
			fmt.Sprintf("%d", c.Order),
		}
		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("write CSV row failed: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush CSV failed: %w", err)
	}

	return buf.Bytes(), nil
}

func (s *CSVExportService) GenerateDepartmentPathCSV(changes []DepartmentChange) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	header := []string{"name", "path"}
	if err := writer.Write(header); err != nil {
		return nil, fmt.Errorf("write CSV header failed: %w", err)
	}

	for _, c := range changes {
		row := []string{
			c.Name,
			fmt.Sprintf("%d", c.PartyID),
		}
		if err := writer.Write(row); err != nil {
			return nil, fmt.Errorf("write CSV row failed: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("flush CSV failed: %w", err)
	}

	return buf.Bytes(), nil
}

func ContactToUserChange(c model.Contact) UserChange {
	var depts []int
	if c.Department != "" {
		parts := strings.Split(strings.Trim(c.Department, "[]"), ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			p = strings.Trim(p, " ")
			if id := parseDeptID(p); id > 0 {
				depts = append(depts, id)
			}
		}
	}

	return UserChange{
		UserID:     c.UserID,
		Name:       c.Name,
		Department: depts,
		Position:   c.Position,
		Mobile:     c.Mobile,
		Email:      c.Email,
		Status:     c.Status,
	}
}

func ContactDetailToUserChange(d model.ContactDetail) UserChange {
	return UserChange{
		UserID:     d.UserID,
		Name:       d.Name,
		Department: d.Department,
		Position:   d.Position,
		Mobile:     d.Mobile,
		Email:      d.Email,
		Telephone:  d.Telephone,
		Status:     d.Status,
	}
}

func parseDeptID(s string) int {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return 0
	}
	var id int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			id = id*10 + int(c-'0')
		} else {
			return 0
		}
	}
	return id
}

func intsToSemicolonString(nums []int) string {
	var parts []string
	for _, n := range nums {
		parts = append(parts, fmt.Sprintf("%d", n))
	}
	return strings.Join(parts, ";")
}

func (s *CSVExportService) ParseMemberCSV(data []byte) ([]UserChange, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse CSV failed: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV has no data rows")
	}

	var changes []UserChange
	for i, record := range records[1:] {
		if len(record) < 8 {
			log.Printf("CSVExport: row %d has insufficient columns, skipping", i+2)
			continue
		}

		var depts []int
		if record[2] != "" {
			parts := strings.Split(record[2], ";")
			for _, p := range parts {
				if id := parseDeptID(strings.TrimSpace(p)); id > 0 {
					depts = append(depts, id)
				}
			}
		}

		enable := 1
		if record[7] != "" {
			fmt.Sscanf(record[7], "%d", &enable)
		}

		changes = append(changes, UserChange{
			UserID:     record[0],
			Name:       record[1],
			Department: depts,
			Position:   record[3],
			Mobile:     record[4],
			Email:      record[5],
			Telephone:  record[6],
			Status:     1 - enable + 4,
		})
	}

	return changes, nil
}

func (s *CSVExportService) ParseDepartmentCSV(data []byte) ([]DepartmentChange, error) {
	reader := csv.NewReader(bytes.NewReader(data))
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("parse CSV failed: %w", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV has no data rows")
	}

	var changes []DepartmentChange
	for i, record := range records[1:] {
		if len(record) < 4 {
			log.Printf("CSVExport: row %d has insufficient columns, skipping", i+2)
			continue
		}

		partyID := parseDeptID(record[0])
		parentID := parseDeptID(record[2])
		order := 0
		fmt.Sscanf(record[3], "%d", &order)

		changes = append(changes, DepartmentChange{
			PartyID:  partyID,
			Name:     record[1],
			ParentID: parentID,
			Order:    order,
		})
	}

	return changes, nil
}
