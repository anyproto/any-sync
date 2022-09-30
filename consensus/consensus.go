package consensus

import "time"

type Log struct {
	Id      []byte   `bson:"_id"`
	Records []Record `bson:"records"`
}

type Record struct {
	Id      []byte    `bson:"id"`
	PrevId  []byte    `bson:"prevId"`
	Payload []byte    `bson:"payload"`
	Created time.Time `bson:"created"'`
}

func (l Log) CopyRecords() Log {
	l2 := l
	l2.Records = make([]Record, len(l.Records))
	copy(l2.Records, l.Records)
	return l2
}
