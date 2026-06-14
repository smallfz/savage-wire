package driver

import (
	"database/sql"
	"database/sql/driver"
	"github.com/smallfz/savage-wire/wire/client"
)

type drv struct {
}

var _ driver.Driver = (*drv)(nil)

func init() {
	sql.Register("savage", new(drv))
}

func (d *drv) Open(dsn string) (driver.Conn, error) {
	return client.OpenDSN(dsn)
}
