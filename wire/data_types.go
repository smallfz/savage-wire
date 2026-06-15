package wire

type Datatype uint32

const (
	INTEGER Datatype = 1
	FLOAT   Datatype = 2
	TEXT    Datatype = 3
	BLOB    Datatype = 4
	NULL    Datatype = 5
)
