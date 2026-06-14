package client

import (
	"fmt"
	"savage-wire/log"
	"savage-wire/wire"
	"strings"
)

func readNextTagPacket(r wire.Reader, tagPrefix string) (*wire.Packet, error) {
	prefix := fmt.Sprintf("%s", tagPrefix)
	for {
		p, err := r.ReadPacket()
		if err != nil {
			return nil, err
		}
		if strings.Index(p.Tag, prefix) == 0 {
			return p, nil
		} else {
			log.Debug(fmt.Sprintf("tag %s: discard", p.Tag))
		}
	}
}
