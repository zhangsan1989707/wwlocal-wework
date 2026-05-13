package handler

import (
	"log"
	"runtime/debug"
	"strconv"

	"wwlocal-wework/internal/repository"
	"wwlocal-wework/internal/service"
	"wwlocal-wework/pkg/response"

	"github.com/labstack/echo/v4"
)

type ContactHandler struct {
	contactSyncSvc *service.ContactSyncService
	contactRepo    *repository.ContactRepository
}

func NewContactHandler(contactSyncSvc *service.ContactSyncService, contactRepo *repository.ContactRepository) *ContactHandler {
	return &ContactHandler{contactSyncSvc: contactSyncSvc, contactRepo: contactRepo}
}

func (h *ContactHandler) List(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	name := c.QueryParam("name")
	mobile := c.QueryParam("mobile")

	contacts, total, err := h.contactRepo.QueryContacts(name, mobile, page, pageSize)
	if err != nil {
		return response.Error(c, 500, "query contacts failed")
	}

	return response.Success(c, map[string]interface{}{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"data":      contacts,
	})
}

func (h *ContactHandler) GetDepartments(c echo.Context) error {
	depts, err := h.contactRepo.GetAllDepartments()
	if err != nil {
		return response.Error(c, 500, "query departments failed")
	}
	return response.Success(c, depts)
}

func (h *ContactHandler) Sync(c echo.Context) error {
	if h.contactSyncSvc.IsRunning() {
		return response.Error(c, 409, "contact sync already in progress")
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("contact sync goroutine panic: %v\n%s", r, debug.Stack())
			}
		}()
		h.contactSyncSvc.SyncContactsFull()
	}()

	return response.Success(c, map[string]interface{}{
		"message": "contact sync started",
		"running": true,
	})
}

func (h *ContactHandler) SyncIncremental(c echo.Context) error {
	if h.contactSyncSvc.IsRunning() {
		return response.Error(c, 409, "contact sync already in progress")
	}

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("contact incremental sync goroutine panic: %v\n%s", r, debug.Stack())
			}
		}()
		h.contactSyncSvc.SyncContactsIncremental()
	}()

	return response.Success(c, map[string]interface{}{
		"message": "contact incremental sync started",
		"running": true,
	})
}

func (h *ContactHandler) Cancel(c echo.Context) error {
	if !h.contactSyncSvc.IsRunning() {
		return response.Error(c, 400, "no contact sync in progress")
	}
	h.contactSyncSvc.Cancel()
	return response.Success(c, map[string]interface{}{
		"message": "contact sync cancellation requested",
	})
}

func (h *ContactHandler) Status(c echo.Context) error {
	return response.Success(c, h.contactSyncSvc.GetStatus())
}

func (h *ContactHandler) GetDeptTree(c echo.Context) error {
	depts, err := h.contactRepo.GetAllDepartments()
	if err != nil {
		return response.Error(c, 500, "query departments failed")
	}

	deptIDs := make([]int, len(depts))
	for i, d := range depts {
		deptIDs[i] = d.ID
	}

	counts, err := h.contactRepo.GetMemberCountByDepartmentIDs(deptIDs)
	if err != nil {
		return response.Error(c, 500, "query member counts failed")
	}

	tree := repository.BuildDeptTree(depts, counts)

	totalContacts, err := h.contactRepo.GetTotalContacts()
	if err != nil {
		return response.Error(c, 500, "query total contacts failed")
	}

	return response.Success(c, map[string]interface{}{
		"tree":  tree,
		"total": totalContacts,
	})
}

func (h *ContactHandler) GetDeptMembers(c echo.Context) error {
	deptID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		return response.Error(c, 400, "invalid department id")
	}

	page, _ := strconv.Atoi(c.QueryParam("page"))
	if page <= 0 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.QueryParam("page_size"))
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	contacts, total, err := h.contactRepo.GetContactsByDepartmentID(deptID, page, pageSize)
	if err != nil {
		return response.Error(c, 500, "query department members failed")
	}

	return response.Success(c, map[string]interface{}{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"data":      contacts,
	})
}

func (h *ContactHandler) GetContact(c echo.Context) error {
	userID := c.Param("userId")
	if userID == "" {
		return response.Error(c, 400, "missing user id")
	}

	contact, err := h.contactRepo.GetContactByUserID(userID)
	if err != nil {
		return response.Error(c, 404, "contact not found")
	}

	return response.Success(c, contact)
}
