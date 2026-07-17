package pagination

type Page[T any] struct {
	Items   []T    `json:"items"`
	Cursor  string `json:"cursor,omitempty"`
	HasMore bool   `json:"has_more"`
}
