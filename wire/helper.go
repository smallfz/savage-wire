package wire

import (
	"context"
	"net"
)

func GetRemoteAddr(x context.Context) net.Addr {
	if v := x.Value("remote-addr"); v != nil {
		if addr, ok := v.(net.Addr); ok {
			return addr
		}
	}
	return nil
}
