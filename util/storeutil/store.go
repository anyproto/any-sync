package storeutil

import "github.com/anyproto/any-store/anyenc"

func NewStringArrayValue(strings []string, arena *anyenc.Arena) *anyenc.Value {
	val := arena.NewArray()
	for idx, str := range strings {
		val.SetArrayItem(idx, arena.NewString(str))
	}
	return val
}

func StringsFromArrayValue(val *anyenc.Value, key string) (res []string) {
	vals := val.GetArray(key)
	res = make([]string, 0, len(vals))
	for _, item := range vals {
		res = append(res, item.GetString())
	}
	return res
}
