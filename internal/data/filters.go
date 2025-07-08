package data

import (
	"strings"

	"github.com/giancarlosisasi/greenlight-api/internal/validator"
)

type Filters struct {
	Page         int
	PageSize     int
	Sort         string
	SortSafeList []string
}

func (f Filters) getSortColumn() string {
	for _, safeValue := range f.SortSafeList {
		// here we don't use contains because just returning
		// if the value is safe is better for performance
		// this has les iterations vs contains method
		if f.Sort == safeValue {
			return strings.TrimPrefix(f.Sort, "-")
		}
	}

	panic("unsafe sort parameter: " + f.Sort)
}

func (f Filters) getSortDirection() string {
	if strings.HasPrefix(f.Sort, "-") {
		return "DESC"
	}

	return "ASC"
}

func (f Filters) getLimit() int {
	return f.PageSize
}

func (f Filters) getOffSet() int {
	return (f.Page - 1) * f.PageSize
}

func ValidateFilters(v *validator.Validator, f Filters) {
	// Check that the page and page_size parameters contain sensible values.
	v.Check(f.Page > 0, "page", "must be greater than zero")
	v.Check(f.Page <= 10_000_000, "page", "must be a maximum of 10million")
	v.Check(f.PageSize > 0, "page_size", "must be greater than zero")
	v.Check(f.PageSize <= 100, "page_size", "must be a maximum of 100")

	// check that the sort parameter matches a value in the safe list
	v.Check(validator.PermittedValues(f.Sort, f.SortSafeList...), "sort", "invalid sort value")
}

type Metadata struct {
	CurrentPage int `json:"current_page,omitzero"`
	PageSize    int `json:"page_size,omitzero"`
	// FirstPage    int `json:"first_page,omitzero"`
	// LastPage     int `json:"last_page,omitzero"`
	TotalPages   int  `json:"total_pages,omitzero"`
	TotalRecords int  `json:"total_records,omitzero"`
	HasNext      bool `json:"has_next,omitzero"`
	HasPrev      bool `json:"has_prev,omitzero"`
}

func calculateMetadata(totalRecords int, page int, pageSize int) Metadata {
	if totalRecords == 0 {
		return Metadata{}
	}

	totalPages := (totalRecords + pageSize - 1) / pageSize

	return Metadata{
		CurrentPage: page,
		PageSize:    pageSize,
		// FirstPage:    1,
		// LastPage:     (totalRecords + pageSize - 1) / pageSize,
		TotalPages:   totalPages,
		TotalRecords: totalRecords,
		HasNext:      page < totalPages,
		HasPrev:      page > 1,
	}
}
