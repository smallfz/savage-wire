package client

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"github.com/smallfz/savage-wire/log"
	"github.com/smallfz/savage-wire/wire"
)

type Stmt interface {
	Close() error
	NumInput() int
	ExecWithArgs(context.Context, []any) (Result, error)
	QueryWithArgs(context.Context, []any) (Rows, error)
}

type stmt struct {
	rw   wire.ReadWriter
	base *wire.Stmt
}

func (s *stmt) Exec(args []driver.Value) (driver.Result, error) {
	namedArgs := make([]driver.NamedValue, len(args))
	for i, arg := range args {
		namedArgs[i] = driver.NamedValue{Ordinal: i + 1, Value: arg}
	}
	return s.ExecContext(context.Background(), namedArgs)
}

func (s *stmt) Query(args []driver.Value) (driver.Rows, error) {
	namedArgs := make([]driver.NamedValue, len(args))
	for i, arg := range args {
		namedArgs[i] = driver.NamedValue{Ordinal: i + 1, Value: arg}
	}
	return s.QueryContext(context.Background(), namedArgs)
}

func (s *stmt) Close() error {
	req := &wire.ReqId{Id: s.base.Id}
	if err := s.rw.WritePacketJSON("stmt-close", req); err != nil {
		return err
	}
	p, err := readNextTagPacket(s.rw, "stmt-close")
	if err != nil {
		return err
	}
	switch p.Tag {
	case "stmt-close:error":
		return fmt.Errorf("%s", string(p.Data))
	case "stmt-close:ok":
		return nil
	default:
		return fmt.Errorf("unexpected reply: tag=%s", p.Tag)
	}
}

func (s *stmt) NumInput() int {
	return s.base.ParamsCount
}

func (s *stmt) toBindArgs(namedArgs []driver.NamedValue) ([]any, error) {
	args := make([]any, s.NumInput())
	for i, _ := range args {
		found := false
		for _, narg := range namedArgs {
			if len(narg.Name) > 0 {
				return nil, fmt.Errorf("named arg not supported.")
			}
			if narg.Ordinal == i+1 {
				found = true
				args[i] = narg.Value
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("arg $%d missing.", i+1)
		}
	}
	return args, nil
}

func (s *stmt) bindArgs(x context.Context, args []any) error {
	buf := new(bytes.Buffer)
	parts := make([]any, 2+len(args))
	parts[0] = int64(s.base.Id)
	parts[1] = int64(len(args))
	for i, v := range args {
		parts[i+2] = v
	}
	if _, err := wire.WriteValues(buf, parts...); err != nil {
		return err
	}
	// req := &wire.ReqBind{Id: s.base.Id}
	// req.Params = make([]any, len(args))
	// for i, arg := range args {
	// 	req.Params[i] = arg
	// }
	if err := s.rw.WritePacket("bind", buf.Bytes()); err != nil {
		return err
	}
	p, err := readNextTagPacket(s.rw, "bind")
	if err != nil {
		return err
	}
	switch p.Tag {
	case "bind:error":
		return fmt.Errorf("%s", string(p.Data))
	case "bind:ok":
		return nil
	default:
		return fmt.Errorf("unexpected reply: tag=%s", p.Tag)
	}
}

func (s *stmt) ExecContext(x context.Context, args []driver.NamedValue) (driver.Result, error) {
	argsOut, err := s.toBindArgs(args)
	if err != nil {
		return nil, err
	}
	return s.ExecWithArgs(x, argsOut)
}

func (s *stmt) ExecWithArgs(x context.Context, args []any) (Result, error) {
	if err := s.bindArgs(x, args); err != nil {
		return nil, err
	}

	req := &wire.ReqId{Id: s.base.Id}
	if err := s.rw.WritePacketJSON("stmt-exec", req); err != nil {
		return nil, err
	}
	p, err := readNextTagPacket(s.rw, "stmt-exec")
	if err != nil {
		return nil, err
	}
	switch p.Tag {
	case "stmt-exec:error":
		return nil, fmt.Errorf("%s", string(p.Data))
	case "stmt-exec:result":
		rsRaw := new(wire.Result)
		if err := json.Unmarshal(p.Data, rsRaw); err != nil {
			return nil, err
		}
		rs := &result{}
		if rsRaw.LastInsertId > 0 {
			rs.lastInsertId = &rsRaw.LastInsertId
		}
		if rsRaw.RowsAffected >= 0 {
			rs.rowsAffected = &rsRaw.RowsAffected
		}
		return rs, nil
	default:
		return nil, fmt.Errorf("unexpected reply: tag=%s", p.Tag)
	}
}

func (s *stmt) QueryContext(x context.Context, args []driver.NamedValue) (driver.Rows, error) {
	argsOut, err := s.toBindArgs(args)
	if err != nil {
		return nil, err
	}
	return s.query(x, argsOut)
}

func (s *stmt) QueryWithArgs(x context.Context, args []any) (Rows, error) {
	return s.query(x, args)
}

func (s *stmt) query(x context.Context, args []any) (*rows, error) {
	if err := s.bindArgs(x, args); err != nil {
		return nil, err
	}

	req := &wire.ReqQueryFetch{Id: s.base.Id}
	if err := s.rw.WritePacketJSON("stmt-query", req); err != nil {
		return nil, err
	}
	p, err := readNextTagPacket(s.rw, "stmt-query")
	if err != nil {
		return nil, err
	}

	portal := (*wire.Portal)(nil)

	switch p.Tag {
	case "stmt-query:error":
		return nil, fmt.Errorf("%s", string(p.Data))
	case "stmt-query:portal":
		ptl := new(wire.Portal)
		if err := json.Unmarshal(p.Data, ptl); err != nil {
			return nil, err
		}
		portal = ptl
		log.Debug("portal received.", "cols", len(ptl.Columns))
	default:
		return nil, fmt.Errorf("unexpected reply: tag=%s", p.Tag)
	}

	// fetch rows
	chClose := make(chan byte, 1)
	ch := make(chan []any, 1)
	chErr := make(chan error, 1)

	go func() {
		defer close(ch)
		for {
			p, err := readNextTagPacket(s.rw, "fetch")
			if err != nil {
				chErr <- err
				return
			}
			// log.Debug(fmt.Sprintf("fetch loop: tag=%s", p.Tag))
			switch p.Tag {
			case "fetch:end":
				return
			case "fetch:error":
				chErr <- fmt.Errorf("%s", string(p.Data))
				return
			case "fetch:data-rows":
				r := bytes.NewReader(p.Data)
				drows, err := wire.ReadDataRows(r)
				if err != nil {
					chErr <- err
					return
				}
				log.Debug(fmt.Sprintf(
					"data rows received. %d rows.",
					len(drows.Rows),
				))
				if len(drows.Rows) > 0 {
					for _, vals := range drows.Rows {
						// log.Info(fmt.Sprintf(
						// 	"deliver row: %d values.",
						// 	len(vals),
						// ))
						select {
						case ch <- vals:
						case <-chClose:
							return
						case <-x.Done():
						}
					}
				}
			}
		}
	}()

	return &rows{portal: portal, ch: ch, chErr: chErr, chClose: chClose}, nil
}
