package wire

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"io"
	"testing"
)

func TestPacketRw(t *testing.T) {
	MAX_FRAGMENT_SIZE = 128

	sizes := []int{
		MAX_FRAGMENT_SIZE*4 + 77,
		MAX_FRAGMENT_SIZE * 4,
		MAX_FRAGMENT_SIZE,
		MAX_FRAGMENT_SIZE - 1,
		MAX_FRAGMENT_SIZE + 32,
		MAX_FRAGMENT_SIZE * 1024,
		1024,
		1,
		0,
	}

	for _, size := range sizes {
		buf := new(bytes.Buffer)

		data := make([]byte, size)
		io.ReadFull(rand.Reader, data)

		if err := WritePacket(buf, "test", data); err != nil {
			t.Fatalf("%v", err)
			return
		}

		r := bytes.NewReader(buf.Bytes())
		p, err := ReadPacket(r)
		if err != nil {
			t.Fatalf("%v", err)
			return
		}

		if !bytes.Equal(p.Data, data) {
			t.Fatalf(
				"result(%d) not equal to initial data(%d).",
				len(p.Data),
				len(data),
			)
		}
	}
}

func TestFloat64RW(t *testing.T) {
	value := float64(107.8)
	buf := new(bytes.Buffer)
	if _, err := WriteValue(buf, value); err != nil {
		t.Fatalf("writing float64: %v", err)
		return
	}
	raw := buf.Bytes()
	if len(raw) != 9 {
		t.Fatalf("float64 expects 9 bytes. got %d", len(raw))
		return
	}
	if raw[0] != 'f' {
		t.Fatalf("float64 expects symbol 'f'. got %d.", raw[0])
		return
	}
	vf := float64(0)
	if _, err := binary.Decode(raw[1:], binary.BigEndian, &vf); err != nil {
		t.Fatalf("%v", err)
		return
	}
	if vf != value {
		t.Fatalf("float64 unexpected: input=%v, output=%v", value, vf)
	}
}

func TestValuesRW(t *testing.T) {
	rows := [][]any{
		[]any{
			int64(1), nil, int64(300),
			float64(107.8), float64(-54), float64(-0.5),
			[]byte("hello"), "hello",
			true, false,
		},
		[]any{
			[]byte("hi"), "there",
			true, int64(-1), nil, int64(300), float64(107.8),
			false, float64(-54), float64(-0.5),
		},
		[]any{
			[]byte("hi"), "there", true,
			int64(300), float64(-107.8), false,
			int64(-54), float64(0.5), nil,
		},
		[]any{
			nil, []byte("hi"), "there",
			true, int64(300), float64(107.8),
			false, int64(-54), float64(-0.5),
		},
		[]any{nil, nil, nil, nil},
		[]any{int64(0), int64(0), int64(0)},
		[]any{int64(1), int64(2), float64(3.3), float64(4.4), float64(5.1005)},
		[]any{"a", "bb", "ccc"},
		[]any{[]byte("hello world!")},
	}

	buf := new(bytes.Buffer)
	for _, row := range rows {
		buf.Reset()
		for _, value := range row {
			if _, err := WriteValue(buf, value); err != nil {
				t.Fatalf("WriteValue(%T, %v): %v", value, value, err)
				return
			}
		}
		raw := buf.Bytes()
		r := bytes.NewReader(raw)
		vr := new(ValueReader)
		for i, _ := range row {
			symbol, value, err := vr.ReadValue(r)
			if err != nil {
				t.Fatalf("ReadValue: %v", err)
				return
			}
			switch symbol {
			case 'N':
				if value != nil || row[i] != nil {
					t.Fatalf("expects nil: input=%T, output=%T", row[i], value)
					return
				}
			case 'i':
				v0, ok0 := row[i].(int64)
				v1, ok1 := value.(int64)
				if !ok0 || !ok1 || v0 != v1 {
					t.Fatalf("expects int64: input=%T, output=%T", row[i], value)
					return
				}
			case 'f':
				v0, ok0 := row[i].(float64)
				v1, ok1 := value.(float64)
				if !ok0 || !ok1 || v0 != v1 {
					t.Fatalf(
						"expects float64: input=%T,%v, output=%T,%v",
						row[i],
						row[i],
						value,
						value,
					)
					return
				}
			case 'b':
				v0, ok0 := row[i].(bool)
				v1, ok1 := value.(bool)
				if !ok0 || !ok1 || v0 != v1 {
					t.Fatalf("expects bool: input=%T, output=%T", row[i], value)
					return
				}
			case 'a':
				v0, ok0 := row[i].([]byte)
				v1, ok1 := value.([]byte)
				if !ok0 || !ok1 || !bytes.Equal(v0, v1) {
					t.Fatalf("expects bytes: input=%T, output=%T", row[i], value)
					return
				}
			case 's':
				v0, ok0 := row[i].(string)
				v1, ok1 := value.(string)
				if !ok0 || !ok1 || v0 != v1 {
					t.Fatalf("expects string: input=%T, output=%T", row[i], value)
					return
				}
			default:
				t.Fatalf("unexpected symbol: %d", symbol)
				return
			}
		}
	}
}
