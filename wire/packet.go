package wire

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"savage-wire/log"
	"strings"
	"sync"
)

var MAX_FRAGMENT_SIZE = 1024 * 1024 * 64

var BE = binary.BigEndian

/* package header

| 's' | 'd' | flag(1) | reserved(2) | tag-len(1) | data-len(4) |
| tag ..... | data ....... |

head[2]: [_ _ _ _ _ _ _ _]
                        ^------ fragment
                      ^-------- final
*/

type Packet struct {
	Flag byte
	Tag  string
	Data []byte
}

func (p *Packet) Fragment() bool {
	return (p.Flag & 0b01) == 0b01
}

func (p *Packet) SetFragment(v bool) {
	if v {
		p.Flag = p.Flag | 0b01
	} else {
		p.Flag = (p.Flag >> 1) << 1
	}
}

func (p *Packet) FinalFragment() bool {
	return (p.Flag & 0b10) == 0b10
}

func (p *Packet) SetFinalFragment(v bool) {
	if v {
		p.Flag = p.Flag | byte(0b10)
	} else {
		p.Flag = p.Flag & ^byte(0b10)
	}
}

func readRawPacket(r io.Reader) (*Packet, error) {
	head := make([]byte, 10)
	if _, err := io.ReadFull(r, head); err != nil {
		return nil, err
	}
	if head[0] != byte('s') || head[1] != byte('d') {
		return nil, fmt.Errorf("invalid savage-wire header sig.")
	}

	tagLen := int(head[5])
	dataLen := int(BE.Uint32(head[6:]))

	if dataLen > MAX_FRAGMENT_SIZE {
		return nil, fmt.Errorf("packet too large.")
	}

	tagb := make([]byte, tagLen)
	if _, err := io.ReadFull(r, tagb); err != nil {
		return nil, err
	}
	tag := string(tagb)

	p := &Packet{Flag: head[2], Tag: tag}

	if dataLen == 0 {
		return p, nil
	}

	dat := make([]byte, int(dataLen))
	if _, err := io.ReadFull(r, dat); err != nil {
		return p, err
	}
	p.Data = dat

	return p, nil
}

func ReadPacket(r io.Reader) (*Packet, error) {
	p, err := readRawPacket(r)
	if err != nil {
		return nil, err
	}
	if !p.Fragment() {
		return p, nil
	}
	for {
		pf, err := readRawPacket(r)
		if err != nil {
			return nil, err
		}
		if !pf.Fragment() {
			return nil, fmt.Errorf("expects a fragment packet.")
		}
		if len(pf.Tag) > 0 {
			return nil, fmt.Errorf("fragment following packet should omit tag.")
		}
		if len(pf.Data) > 0 {
			p.Data = append(p.Data, pf.Data...)
		}
		if pf.FinalFragment() || len(pf.Data) == 0 {
			break
		}
	}
	return p, nil
}

type Reader interface {
	ReadPacket() (*Packet, error)
}

type packetReader struct {
	base io.Reader
	lck  *sync.Mutex
}

func NewReader(r io.Reader) Reader {
	return &packetReader{base: r, lck: new(sync.Mutex)}
}

func (r *packetReader) ReadPacket() (*Packet, error) {
	r.lck.Lock()
	defer r.lck.Unlock()
	return ReadPacket(r.base)
}

func writeRawPacket(w io.Writer, p *Packet) error {
	head := make([]byte, 10)

	head[0] = byte('s')
	head[1] = byte('d')
	head[2] = p.Flag
	head[3] = 0 /* reserved */
	head[4] = 0 /* reserved */
	head[5] = byte(len(p.Tag))
	BE.PutUint32(head[6:6+4], uint32(len(p.Data)))

	if _, err := w.Write(head); err != nil {
		return err
	}

	if len(p.Tag) > 0 {
		if _, err := w.Write([]byte(p.Tag)); err != nil {
			return err
		}
	}

	if len(p.Data) > 0 {
		if _, err := w.Write(p.Data); err != nil {
			return err
		}
	}

	return nil
}

func WritePacket(w io.Writer, tag string, dat []byte) error {
	if len(tag) > 255 {
		return fmt.Errorf("tag too long.")
	}
	if len(dat) <= MAX_FRAGMENT_SIZE {
		p := &Packet{Tag: tag, Data: dat}
		return writeRawPacket(w, p)
	}

	fragmentSize := MAX_FRAGMENT_SIZE
	offset := 0

	p0 := &Packet{Tag: tag, Data: dat[:fragmentSize]}
	p0.SetFragment(true)
	if err := writeRawPacket(w, p0); err != nil {
		return err
	}

	offset += fragmentSize

	for {
		if offset >= len(dat) {
			break
		}
		final := false
		chunkSize := fragmentSize
		if offset+chunkSize >= len(dat) {
			chunkSize = len(dat) - offset
			final = true
		}
		pf := &Packet{Data: dat[offset : offset+chunkSize]}
		pf.SetFragment(true)
		pf.SetFinalFragment(final)
		if err := writeRawPacket(w, pf); err != nil {
			return err
		}
		offset += chunkSize
	}

	return nil
}

func WritePacketJSON(w io.Writer, tag string, payload any) error {
	if dat, err := json.Marshal(payload); err != nil {
		return err
	} else {
		return WritePacket(w, tag, dat)
	}
}

type Writer interface {
	WritePacket(string, []byte) error
	WritePacketJSON(string, any) error
}

type packetWriter struct {
	base io.Writer
	lck  *sync.Mutex
}

func NewWriter(w io.Writer) Writer {
	return &packetWriter{base: w, lck: new(sync.Mutex)}
}

func (w *packetWriter) WritePacket(tag string, dat []byte) error {
	w.lck.Lock()
	defer w.lck.Unlock()
	return WritePacket(w.base, tag, dat)
}

func (w *packetWriter) WritePacketJSON(tag string, payload any) error {
	w.lck.Lock()
	defer w.lck.Unlock()
	return WritePacketJSON(w.base, tag, payload)
}

type ReadWriter interface {
	Reader
	Writer
	io.Closer
}

type readWriter struct {
	r    Reader
	w    Writer
	base io.ReadWriter
}

func (rw *readWriter) Close() error {
	if rw.base != nil {
		if closer, ok := rw.base.(io.Closer); ok {
			return closer.Close()
		}
	}
	return nil
}

func (rw *readWriter) ReadPacket() (*Packet, error) {
	return rw.r.ReadPacket()
}

func (rw *readWriter) WritePacket(tag string, dat []byte) error {
	return rw.w.WritePacket(tag, dat)
}

func (rw *readWriter) WritePacketJSON(tag string, payload any) error {
	return rw.w.WritePacketJSON(tag, payload)
}

func NewReadWriter(rw io.ReadWriter) ReadWriter {
	return &readWriter{
		r:    NewReader(rw),
		w:    NewWriter(rw),
		base: rw,
	}
}

type readWriterAsync struct {
	chClose chan byte
	ch      chan *Packet
	w       Writer
	base    io.ReadWriter
}

func (rw *readWriterAsync) Close() error {
	if rw.base != nil {
		if closer, ok := rw.base.(io.Closer); ok {
			return closer.Close()
		}
	}
	select {
	case rw.chClose <- 1:
	default:
	}
	return nil
}

func (rw *readWriterAsync) ReadPacket() (*Packet, error) {
	select {
	case p, ok := <-rw.ch:
		if !ok {
			return nil, io.EOF
		}
		return p, nil
	}
}

func (rw *readWriterAsync) WritePacket(tag string, dat []byte) error {
	return rw.w.WritePacket(tag, dat)
}

func (rw *readWriterAsync) WritePacketJSON(tag string, payload any) error {
	return rw.w.WritePacketJSON(tag, payload)
}

func NewReadWriterAsync(rw io.ReadWriter) ReadWriter {
	chClose := make(chan byte, 1)
	ch := make(chan *Packet, 32)
	r := NewReader(rw)
	go func() {
		defer close(ch)
		defer log.Debug("ReadWriterAsync: Reader exit.")
		for {
			if p, err := r.ReadPacket(); err != nil {
				return
			} else {
				if strings.EqualFold(p.Tag, "ping") {
					log.Debug("ping received.")
				} else {
					select {
					case <-chClose:
					case ch <- p:
					}
				}
			}
		}
	}()
	return &readWriterAsync{
		chClose: chClose,
		ch:      ch,
		w:       NewWriter(rw),
		base:    rw,
	}
}
