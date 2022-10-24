package consensus

import "time"

type Log struct {
	Id      []byte   `bson:"_id"`
	Records []Record `bson:"records"`
	Err     error    `bson:"-"`
}

type Record struct {
	Id      []byte    `bson:"id"`
	PrevId  []byte    `bson:"prevId"`
	Payload []byte    `bson:"payload"`
	Created time.Time `bson:"created"'`
}
