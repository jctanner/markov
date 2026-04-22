package engine

import (
	"fmt"
	"strconv"
)

// FactStore wraps a map for use as a Grule DataContext fact.
// Exported methods are callable from GRL rule expressions.
type FactStore struct {
	data  map[string]any
	fired map[string]bool
}

func NewFactStore(data map[string]any) *FactStore {
	return &FactStore{
		data:  data,
		fired: make(map[string]bool),
	}
}

func (f *FactStore) Get(key string) interface{} {
	return f.data[key]
}

func (f *FactStore) GetStr(key string) string {
	v, ok := f.data[key]
	if !ok || v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func (f *FactStore) GetNum(key string) float64 {
	v, ok := f.data[key]
	if !ok || v == nil {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case string:
		if num, err := strconv.ParseFloat(n, 64); err == nil {
			return num
		}
		return 0
	default:
		return 0
	}
}

func (f *FactStore) IsTrue(key string) bool {
	v, ok := f.data[key]
	if !ok {
		return false
	}
	switch b := v.(type) {
	case bool:
		return b
	case nil:
		return false
	case string:
		return b != ""
	case int:
		return b != 0
	case int64:
		return b != 0
	case float64:
		return b != 0
	default:
		return true
	}
}

func (f *FactStore) IsNil(key string) bool {
	v, ok := f.data[key]
	return !ok || v == nil
}

func (f *FactStore) Set(key string, val interface{}) {
	f.data[key] = val
}

func (f *FactStore) SetStr(key string, val string) {
	f.data[key] = val
}

func (f *FactStore) SetBool(key string, val bool) {
	f.data[key] = val
}

func (f *FactStore) SetNum(key string, val float64) {
	f.data[key] = val
}

func (f *FactStore) MarkFired(ruleName string) {
	f.fired[ruleName] = true
}

func (f *FactStore) HasFired(ruleName string) bool {
	return f.fired[ruleName]
}
