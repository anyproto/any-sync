package debugstat

type statValue struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type statType struct {
	Type   string      `json:"type"`
	Values []statValue `json:"values"`
}

type statSummary struct {
	Stats []statType `json:"stat_types"`
}
