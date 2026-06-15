package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

type ReqAuth struct {
	User   string `json:"user"`
	Pwd    string `json:"pwd"`
	DbName string `json:"db_name"`
}

type ReqBind struct {
	Id          int32 `json:"id"` /* prepared-statement id */
	ParamsCount int   `json:"params_count"`
	Params      []any `json:"params"`
}

type ReqQueryFetch struct {
	Id       int32  `json:"id"`
	CountMax int    `json:"count_max"`
	Encoding string `json:"encoding"`
}

type ReqId struct {
	Id int32 `json:"id"`
}

type AuthResult struct {
	UserId   int64  `json:"user_id"`
	VFSName  string `json:"vfs_name"`
	User     string `json:"user"`
	DbName   string `json:"db_name"`
	Super    bool   `json:"super"`
	Readonly bool   `json:"readonly"`
}

type Stmt struct {
	Id          int32    `json:"id"`
	ParamsCount int      `json:"params_count"`
	Cmds        []string `json:"cmds"`
}

type Result struct {
	LastInsertId int64 `json:"last_insert_id"`
	RowsAffected int64 `json:"rows_affected"`
}

type Column struct {
	Name string   `json:"column"`
	Type Datatype `json:"type"`
}

type Portal struct {
	Id           int32     `json:"id"`
	Columns      []*Column `json:"columns"`
	RowsAffected int64     `json:"rows_affected"`
	Offset       int64     `json:"-"`
}

type DataRows struct {
	Id          int32
	OffsetStart int64
	Rows        [][]any
}

func (rs *DataRows) WriteTo(w io.Writer) (int64, error) {
	if len(rs.Rows) == 0 {
		return 0, nil
	}

	colsCount := len(rs.Rows[0])

	/* header:
	   [ id(4) | offset(8) | rows-count(4) | columns-count-each(4) ]
	*/
	head := make([]byte, 4+8+4+4)
	binary.BigEndian.PutUint32(head[:4], uint32(rs.Id))
	binary.BigEndian.PutUint64(head[4:4+8], uint64(rs.OffsetStart))
	binary.BigEndian.PutUint32(head[12:12+4], uint32(len(rs.Rows)))
	binary.BigEndian.PutUint32(head[16:16+4], uint32(colsCount))

	w.Write(head)

	dataSize := len(head)
	buf := new(bytes.Buffer)
	for _, row := range rs.Rows {
		if len(row) != colsCount {
			return 0, fmt.Errorf(
				"found row not aligned to columns. expects %d values got %d.",
				colsCount,
				len(row),
			)
		}
		for _, value := range row {
			size, err := WriteValue(buf, value)
			if err != nil {
				return 0, err
			}
			dataSize += size
		}
	}

	if _, err := w.Write(buf.Bytes()); err != nil {
		return 0, err
	}

	return int64(dataSize), nil
}

func ReadDataRows(r io.Reader) (*DataRows, error) {
	head := make([]byte, 4+8+4+4)
	if _, err := io.ReadFull(r, head); err != nil {
		return nil, err
	}

	id := int32(binary.BigEndian.Uint32(head[:4]))
	offset := int64(binary.BigEndian.Uint64(head[4 : 4+8]))
	rowsCount := int(binary.BigEndian.Uint32(head[12 : 12+4]))
	colsCount := int(binary.BigEndian.Uint32(head[16 : 16+4]))

	if rowsCount <= 0 || colsCount <= 0 {
		return &DataRows{Id: id, OffsetStart: offset}, nil
	}

	vr := new(ValueReader)

	rows := make([][]any, rowsCount)
	for i := 0; i < rowsCount; i += 1 {
		row := make([]any, colsCount)
		for j, _ := range row {
			_, v, err := vr.ReadValue(r)
			if err != nil {
				return nil, err
			}
			row[j] = v
		}
		rows[i] = row
	}

	return &DataRows{Id: id, OffsetStart: offset, Rows: rows}, nil
}
