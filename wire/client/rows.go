package client

import (
	"database/sql/driver"
	"fmt"
	"github.com/smallfz/savage-wire/log"
	"github.com/smallfz/savage-wire/wire"
	"io"
)

type Rows interface {
	Close() error
	Columns() []string
	NextRow([]any) error
}

type rows struct {
	portal  *wire.Portal
	ch      <-chan []any
	chErr   <-chan error
	chClose chan<- byte
}

func (rs *rows) Close() error {
	rs.closeSelf()
	return nil
}

func (rs *rows) closeSelf() {
	select {
	case rs.chClose <- 1:
	default:
	}
}

func (rs *rows) Columns() []string {
	cols := make([]string, len(rs.portal.Columns))
	for i, _ := range cols {
		cols[i] = rs.portal.Columns[i].Name
	}
	return cols
}

func (rs *rows) Next(dest []driver.Value) error {
	row := make([]any, len(dest))
	for i, _ := range row {
		row[i] = dest[i]
	}
	if err := rs.NextRow(row); err != nil {
		return err
	} else {
		for i, v := range row {
			dest[i] = v
		}
		return nil
	}
}

func (rs *rows) NextRow(dest []any) error {
	select {
	case err := <-rs.chErr:
		return err
	default:
	}
	log.Debug("fetch row from ch...")
	vals, ok := <-rs.ch
	if !ok {
		log.Debug("ch closed.")
		return io.EOF
	}
	if len(dest) > len(vals) {
		go rs.closeSelf()
		return fmt.Errorf(
			"provided %d slots for only %d values.",
			len(dest),
			len(vals),
		)
	}
	for i, _ := range dest {
		dest[i] = vals[i]
	}
	return nil
}
