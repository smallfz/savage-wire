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

type Conn interface {
	PrepareStmt(context.Context, string) (Stmt, error)
	ExecWithArgs(context.Context, string, []any) (Result, error)
	BeginTx() (*Tx, error)
	Close() error
}

type conn struct {
	rw         wire.ReadWriter
	reqAuth    *wire.ReqAuth
	authResult *wire.AuthResult
}

var _ driver.Conn = (*conn)(nil)
var _ Conn = (*conn)(nil)

func (c *conn) handshake(req *wire.ReqAuth) error {
	if err := c.rw.WritePacketJSON("auth", req); err != nil {
		return err
	}

	p, err := readNextTagPacket(c.rw, "auth")
	if err != nil {
		return err
	}
	switch p.Tag {
	case "auth:error":
		return fmt.Errorf("%s", string(p.Data))
	case "auth:result":
		rs := new(wire.AuthResult)
		if err := json.Unmarshal(p.Data, rs); err != nil {
			return err
		}
		c.authResult = rs
	default:
		return fmt.Errorf("unexpected reply: tag=%s", p.Tag)
	}

	p, err = readNextTagPacket(c.rw, "open")
	if err != nil {
		return err
	}
	switch p.Tag {
	case "open:error":
		return fmt.Errorf("%s", string(p.Data))
	case "open:ok":
		log.Debug("auth OK. target db opened.")
		return nil
	default:
		return fmt.Errorf("unexpected reply: tag=%s", p.Tag)
	}
}

func (c *conn) Prepare(q string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), q)
}

func (c *conn) PrepareContext(x context.Context, q string) (driver.Stmt, error) {
	return c.prepare(x, q)
}

func (c *conn) PrepareStmt(x context.Context, q string) (Stmt, error) {
	return c.prepare(x, q)
}

func (c *conn) prepare(x context.Context, q string) (*stmt, error) {
	if err := c.rw.WritePacket("prepare", []byte(q)); err != nil {
		return nil, err
	}
	p, err := readNextTagPacket(c.rw, "prepare")
	if err != nil {
		return nil, err
	}
	switch p.Tag {
	case "prepare:error":
		return nil, fmt.Errorf("%s", string(p.Data))
	case "prepare:statement":
		t := new(wire.Stmt)
		if err := json.Unmarshal(p.Data, t); err != nil {
			return nil, err
		}
		return &stmt{rw: c.rw, base: t}, nil
	default:
		return nil, fmt.Errorf("unexpected reply: tag=%s", p.Tag)
	}
}

func (c *conn) ExecContext(x context.Context, q string, args []driver.NamedValue) (driver.Result, error) {
	argValues := make([]any, len(args))
	for i, _ := range argValues {
		argValues[i] = args[i].Value
	}
	return c.ExecWithArgs(x, q, argValues)
}

func (c *conn) ExecWithArgs(x context.Context, q string, args []any) (Result, error) {
	log.Debug("exec:", "q", q, "args", len(args))
	buf := new(bytes.Buffer)
	parts := make([]any, 2+len(args))
	parts[0] = q
	parts[1] = int64(len(args))
	for i, v := range args {
		parts[i+2] = v
	}
	if _, err := wire.WriteValues(buf, parts...); err != nil {
		return nil, err
	}

	if err := c.rw.WritePacket("exec", buf.Bytes()); err != nil {
		return nil, err
	}
	p, err := readNextTagPacket(c.rw, "exec")
	if err != nil {
		return nil, err
	}
	switch p.Tag {
	case "exec:error":
		return nil, fmt.Errorf("%s", string(p.Data))
	case "exec:result":
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

func (c *conn) Begin() (driver.Tx, error) {
	return c.BeginTx()
}

func (c *conn) BeginTx() (*Tx, error) {
	if err := c.rw.WritePacket("begin", nil); err != nil {
		return nil, err
	}
	p, err := readNextTagPacket(c.rw, "begin")
	if err != nil {
		return nil, err
	}
	switch p.Tag {
	case "begin:error":
		return nil, fmt.Errorf("%s", string(p.Data))
	case "begin:ok":
		return &Tx{rw: c.rw}, nil
	default:
		return nil, fmt.Errorf("unexpected reply: tag=%s", p.Tag)
	}
}

func (c *conn) Close() error {
	if err := c.rw.WritePacket("bye", nil); err != nil {
	}
	return c.rw.Close()
}
