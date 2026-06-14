package client

import (
	"fmt"
	"savage-wire/wire"
)

type Tx struct {
	rw wire.ReadWriter
}

func (tx *Tx) Commit() error {
	if err := tx.rw.WritePacket("commit", nil); err != nil {
		return err
	}
	p, err := readNextTagPacket(tx.rw, "commit")
	if err != nil {
		return err
	}
	switch p.Tag {
	case "commit:error":
		return fmt.Errorf("%s", string(p.Data))
	case "commit:ok":
		return nil
	default:
		return fmt.Errorf("unexpected reply: tag=%s", p.Tag)
	}
}

func (tx *Tx) Rollback() error {
	if err := tx.rw.WritePacket("rollback", nil); err != nil {
		return err
	}
	p, err := readNextTagPacket(tx.rw, "rollback")
	if err != nil {
		return err
	}
	switch p.Tag {
	case "rollback:error":
		return fmt.Errorf("%s", string(p.Data))
	case "rollback:ok":
		return nil
	default:
		return fmt.Errorf("unexpected reply: tag=%s", p.Tag)
	}
}
