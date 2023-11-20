package debugstat

type StatValue struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

type StatType struct {
	Type   string      `json:"type"`
	Values []StatValue `json:"values"`
}

type StatSummary struct {
	Stats []StatType `json:"stat_types"`
}
