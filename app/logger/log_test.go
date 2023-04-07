package logger

import (
	"reflect"
	"testing"

	"go.uber.org/zap"
)

func Test_getLevel1(t *testing.T) {
	SetNamedLevels([]NamedLevel{
		{Name: "app", Level: "debug"},
		{Name: "app*", Level: "info"},
		{Name: "app.sub", Level: "warn"},
		{Name: "*", Level: "fatal"},
	})

	tests := []struct {
		name string
		want zap.AtomicLevel
	}{
		{
			name: "app",
			want: zap.NewAtomicLevelAt(zap.DebugLevel),
		},
		{
			name: "app.aaa",
			want: zap.NewAtomicLevelAt(zap.InfoLevel),
		},
		{
			name: "app.sub",
			want: zap.NewAtomicLevelAt(zap.InfoLevel),
		},
		{
			name: "random",
			want: zap.NewAtomicLevelAt(zap.FatalLevel),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getLevel(tt.name); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getLevel2(t *testing.T) {
	SetNamedLevels([]NamedLevel{
		{Name: "*", Level: "ERROR"},
		{Name: "app", Level: "info"},
		{Name: "app.sub", Level: "warn"},
		{Name: "*", Level: "fatal"},
	})

	tests := []struct {
		name string
		want zap.AtomicLevel
	}{
		{
			name: "app",
			want: zap.NewAtomicLevelAt(zap.ErrorLevel),
		},
		{
			name: "app.aaa",
			want: zap.NewAtomicLevelAt(zap.ErrorLevel),
		},
		{
			name: "app.sub",
			want: zap.NewAtomicLevelAt(zap.ErrorLevel),
		},
		{
			name: "random",
			want: zap.NewAtomicLevelAt(zap.ErrorLevel),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getLevel(tt.name); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getLevel3(t *testing.T) {
	SetNamedLevels([]NamedLevel{
		{Name: "app", Level: "info"},
		{Name: "*.sub", Level: "warn"},
		{Name: "*", Level: "fatal"},
	})

	tests := []struct {
		name string
		want zap.AtomicLevel
	}{
		{
			name: "app",
			want: zap.NewAtomicLevelAt(zap.InfoLevel),
		},
		{
			name: "app.sub",
			want: zap.NewAtomicLevelAt(zap.WarnLevel),
		},
		{
			name: "random",
			want: zap.NewAtomicLevelAt(zap.FatalLevel),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getLevel(tt.name); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getLevel4(t *testing.T) {
	SetNamedLevels([]NamedLevel{
		{Name: "*", Level: "invalid"},
		{Name: "app", Level: "info"},
		{Name: "b", Level: "invalid"},
	})

	tests := []struct {
		name string
		want zap.AtomicLevel
	}{
		{
			name: "app",
			want: zap.NewAtomicLevelAt(zap.InfoLevel),
		},
		{
			name: "app.sub",
			want: zap.NewAtomicLevelAt(logger.Level()),
		},
		{
			name: "b",
			want: zap.NewAtomicLevelAt(logger.Level()),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getLevel(tt.name); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getLevel() = %v, want %v", got, tt.want)
			}
		})
	}
}
