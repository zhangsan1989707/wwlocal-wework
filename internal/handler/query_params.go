package handler

import (
	"fmt"
	"strconv"

	"github.com/labstack/echo/v4"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
)

func parsePagination(c echo.Context) (int, int, error) {
	page, err := parsePositiveIntQuery(c, "page", defaultPage, 0)
	if err != nil {
		return 0, 0, err
	}
	pageSize, err := parsePositiveIntQuery(c, "page_size", defaultPageSize, maxPageSize)
	if err != nil {
		return 0, 0, err
	}
	return page, pageSize, nil
}

func parsePositiveIntQuery(c echo.Context, name string, defaultValue int, maxValue int) (int, error) {
	raw := c.QueryParam(name)
	if raw == "" {
		return defaultValue, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("invalid %s", name)
	}
	if maxValue > 0 && value > maxValue {
		return 0, fmt.Errorf("%s must be <= %d", name, maxValue)
	}
	return value, nil
}

func parseOptionalIntQuery(c echo.Context, name string) (int, error) {
	raw := c.QueryParam(name)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return value, nil
}

func parseOptionalInt64Query(c echo.Context, name string) (int64, error) {
	raw := c.QueryParam(name)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid %s", name)
	}
	return value, nil
}
