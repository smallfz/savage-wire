package client

import (
	"fmt"
)

type Result interface {
	LastInsertId() (int64, error)
	RowsAffected() (int64, error)
}

type result struct {
	lastInsertId *int64
	rowsAffected *int64
}

var _ Result = (*result)(nil)

func (rs *result) LastInsertId() (int64, error) {
	if rs.lastInsertId != nil {
		return *rs.lastInsertId, nil
	}
	return 0, fmt.Errorf("not available.")
}

func (rs *result) RowsAffected() (int64, error) {
	if rs.rowsAffected != nil {
		return *rs.rowsAffected, nil
	}
	return 0, nil
}
