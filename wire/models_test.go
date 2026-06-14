package wire

import (
	"bytes"
	"io"
	"testing"
)

func TestRowsRW(t *testing.T) {

	drows := &DataRows{
		Id:          4,
		OffsetStart: 0,
		Rows: [][]any{
			[]any{int64(100), float64(107.9), nil, true, "great!"},
			[]any{int64(-234), nil, []byte("hello"), false, ""},
			[]any{int64(0), float64(-1.7), []byte(""), false, nil},
		},
	}

	buf := new(bytes.Buffer)
	if _, err := drows.WriteTo(buf); err != nil {
		t.Fatalf("drows.WriteTo: %v", err)
		return
	}

	r := bytes.NewReader(buf.Bytes())
	out, err := ReadDataRows(r)
	if err != nil {
		t.Fatalf("ReadDataRows: %v", err)
		return
	}

	buf1 := make([]byte, 1)
	if _, err := r.Read(buf1); err == nil {
		t.Fatalf("all bytes should be consumed.")
		return
	} else if err != io.EOF {
		t.Fatalf("expects EOF. got %v", err)
		return
	}

	if out.Id != drows.Id {
		t.Fatalf(
			"unexpected data rows id: expects %d but got %d.",
			drows.Id,
			out.Id,
		)
		return
	}
	if out.OffsetStart != drows.OffsetStart {
		t.Fatalf(
			"unexpected offset start: expects %d but got %d.",
			drows.OffsetStart,
			out.OffsetStart,
		)
		return
	}
	if len(drows.Rows) != len(out.Rows) {
		t.Fatalf(
			"expects %d rows. got %d.",
			len(drows.Rows),
			len(out.Rows),
		)
	}
}
