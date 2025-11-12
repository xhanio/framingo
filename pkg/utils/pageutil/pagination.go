package pageutil

// PaginationParams contains pagination and sorting parameters
type PaginationParams struct {
	Page     int    `json:"page"`                // Page number (1-based). If 0, returns all items (unpaginated)
	PageSize int    `json:"page_size"`           // Items per page. If 0, uses default (20)
	SortBy   string `json:"sort_by,omitempty"`   // Field to sort by
	SortDesc bool   `json:"sort_desc,omitempty"` // Sort direction (false=ASC, true=DESC)
}

// Pagination response structure
type Pagination[T any] struct {
	Items      []T `json:"items"`
	TotalCount int `json:"total_count"`
	Page       int `json:"page"`        // Current page (0 for unpaginated)
	PageSize   int `json:"page_size"`   // Items per page (0 for unpaginated)
	TotalPages int `json:"total_pages"` // Total number of pages
}

// IsPaginated returns true if pagination is requested
func (p *PaginationParams) IsPaginated() bool {
	return p.Page > 0
}

// Offset returns the database offset for pagination
func (p *PaginationParams) Offset() int {
	if !p.IsPaginated() {
		return 0
	}
	return (p.Page - 1) * p.PageSize
}

// Limit returns the database limit for pagination
func (p *PaginationParams) Limit() int {
	if !p.IsPaginated() {
		return 0 // No limit
	}
	return p.PageSize
}

// NewPagination creates a pagination response
func NewPagination[T any](items []T, total int, params PaginationParams) *Pagination[T] {
	totalPages := 0
	if params.IsPaginated() && params.PageSize > 0 {
		totalPages = (total + params.PageSize - 1) / params.PageSize
	}
	return &Pagination[T]{
		Items:      items,
		TotalCount: total,
		Page:       params.Page,
		PageSize:   params.PageSize,
		TotalPages: totalPages,
	}
}
