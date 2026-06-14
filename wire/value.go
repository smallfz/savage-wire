package wire

import (
	"encoding/binary"
	"fmt"
	"io"
	"time"
)

type ValueReader struct {
	head []byte
}

func (vr *ValueReader) ReadString(r io.Reader) (string, error) {
	symbol, val, err := vr.ReadValue(r)
	if err != nil {
		return "", err
	}
	switch symbol {
	case 'a':
		v, ok := val.([]byte)
		if !ok {
			return "", fmt.Errorf("unexpected value with symbol 'a'.")
		}
		return string(v), nil
	case 's':
		v, ok := val.(string)
		if !ok {
			return "", fmt.Errorf("unexpected value with symbol 's'.")
		}
		return v, nil
	default:
		return "", fmt.Errorf("reading string but got symbol %c.", symbol)
	}
}

func (vr *ValueReader) ReadInt64(r io.Reader) (int64, error) {
	symbol, val, err := vr.ReadValue(r)
	if err != nil {
		return 0, err
	}
	switch symbol {
	case 'i':
		v, ok := val.(int64)
		if !ok {
			return 0, fmt.Errorf("unexpected value with symbol 'i'.")
		}
		return v, nil
	default:
		return 0, fmt.Errorf("reading string but got symbol %c.", symbol)
	}
}

func (vr *ValueReader) ReadValue(r io.Reader) (byte, any, error) {
	if len(vr.head) == 0 {
		vr.head = make([]byte, 1+8, 1+8)
	}
	h := vr.head
	if _, err := io.ReadFull(r, h[:1]); err != nil {
		return 0, nil, err
	}
	symbol := h[0]
	switch symbol {
	case 'N':
		return symbol, nil, nil
	case 'i':
		if _, err := io.ReadFull(r, h[1:1+8]); err != nil {
			return symbol, nil, err
		}
		v := BE.Uint64(h[1 : 1+8])
		return symbol, int64(v), nil
	case 'f':
		if _, err := io.ReadFull(r, h[1:1+8]); err != nil {
			return symbol, nil, err
		}
		vf := float64(0)
		if _, err := binary.Decode(h[1:1+8], BE, &vf); err != nil {
			return symbol, nil, err
		}
		return symbol, vf, nil
	case 'b':
		if _, err := io.ReadFull(r, h[1:2]); err != nil {
			return symbol, nil, err
		}
		return symbol, h[1] > 0, nil
	case 'a', 's':
		if _, err := io.ReadFull(r, h[1:1+4]); err != nil {
			return symbol, nil, err
		}
		size := int(BE.Uint32(h[1 : 1+4]))
		dat := make([]byte, size)
		if size > 0 {
			if _, err := io.ReadFull(r, dat); err != nil {
				return symbol, nil, err
			}
		}
		if symbol == 's' {
			return symbol, string(dat), nil
		}
		return symbol, dat, nil
	default:
		return symbol, nil, fmt.Errorf("unknown value symbol: %c", symbol)
	}
}

func WriteValue(w io.Writer, value any) (int, error) {
	chunk := make([]byte, 1+8)
	size := 0
	body := ([]byte)(nil)
	if value == nil {
		chunk[0] = 'N'
		size = 1
	} else {
		switch v := value.(type) {
		case int:
			chunk[0] = byte('i')
			BE.PutUint64(chunk[1:9], uint64(v))
			size = 9
		case int32:
			chunk[0] = byte('i')
			BE.PutUint64(chunk[1:9], uint64(v))
			size = 9
		case int64:
			chunk[0] = byte('i')
			BE.PutUint64(chunk[1:9], uint64(v))
			size = 9
		case float64:
			chunk[0] = byte('f')
			if _, err := binary.Encode(chunk[1:1+8], BE, v); err != nil {
				return 0, err
			}
			size = 9
		case bool:
			chunk[0] = byte('b')
			if v {
				chunk[1] = 1
			} else {
				chunk[1] = 0
			}
			size = 2
		case []byte:
			chunk[0] = 'a'
			BE.PutUint32(chunk[1:5], uint32(len(v)))
			size = 5
			body = v
		case string:
			chunk[0] = 's'
			BE.PutUint32(chunk[1:5], uint32(len(v)))
			size = 5
			body = []byte(v)
		case time.Time:
			chunk[0] = byte('i')
			BE.PutUint64(chunk[1:9], uint64(v.Unix()))
			size = 9
		default:
			return 0, fmt.Errorf(
				"type %T not serializable: %v",
				value,
				value,
			)
		}
	}
	if _, err := w.Write(chunk[:size]); err != nil {
		return 0, err
	}
	if len(body) > 0 {
		size += len(body)
		if _, err := w.Write(body); err != nil {
			return 0, err
		}
	}
	return size, nil
}

func WriteValues(w io.Writer, values ...any) (int, error) {
	if len(values) == 0 {
		return 0, nil
	}
	total := 0
	for _, value := range values {
		if size, err := WriteValue(w, value); err != nil {
			return total, err
		} else {
			total += size
		}
	}
	return total, nil
}
