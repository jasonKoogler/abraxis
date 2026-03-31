package db

import "fmt"

func AddPagination(query string, args []interface{}, page, pageSize *int) (string, []interface{}) {
	if page != nil && pageSize != nil {
		offset := (*page - 1) * *pageSize
		args = append(args, *pageSize, offset)
		query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args))
	}
	return query, args
}
