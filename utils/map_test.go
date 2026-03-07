package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// ===== 基本使用 =====

func TestGetMapValue_StringValue(t *testing.T) {
	m := map[string]any{"name": "Alice"}
	val, ok := GetMapValue[string, string](m, "name")
	require.True(t, ok)
	require.Equal(t, "Alice", val)
}

func TestGetMapValue_IntValue(t *testing.T) {
	m := map[string]any{"age": 30}
	val, ok := GetMapValue[string, int](m, "age")
	require.True(t, ok)
	require.Equal(t, 30, val)
}

func TestGetMapValue_BoolValue(t *testing.T) {
	m := map[string]any{"active": true}
	val, ok := GetMapValue[string, bool](m, "active")
	require.True(t, ok)
	require.Equal(t, true, val)
}

func TestGetMapValue_Float64Value(t *testing.T) {
	m := map[string]any{"score": 99.5}
	val, ok := GetMapValue[string, float64](m, "score")
	require.True(t, ok)
	require.InDelta(t, 99.5, val, 1e-9)
}

// ===== key 不存在 =====

func TestGetMapValue_KeyNotFound(t *testing.T) {
	m := map[string]any{"name": "Alice"}
	val, ok := GetMapValue[string, string](m, "missing")
	require.False(t, ok)
	require.Equal(t, "", val) // 零值
}

func TestGetMapValue_KeyNotFound_IntZero(t *testing.T) {
	m := map[string]any{}
	val, ok := GetMapValue[string, int](m, "count")
	require.False(t, ok)
	require.Equal(t, 0, val)
}

// ===== nil map =====

func TestGetMapValue_NilMap(t *testing.T) {
	val, ok := GetMapValue[string, string](nil, "key")
	require.False(t, ok)
	require.Equal(t, "", val)
}

func TestGetMapValue_NilMap_IntZero(t *testing.T) {
	val, ok := GetMapValue[string, int](nil, "key")
	require.False(t, ok)
	require.Equal(t, 0, val)
}

// ===== value 为 nil =====

func TestGetMapValue_NilValue(t *testing.T) {
	m := map[string]any{"key": nil}
	val, ok := GetMapValue[string, string](m, "key")
	require.False(t, ok)
	require.Equal(t, "", val)
}

func TestGetMapValue_NilValue_IntZero(t *testing.T) {
	m := map[string]any{"key": nil}
	val, ok := GetMapValue[string, int](m, "key")
	require.False(t, ok)
	require.Equal(t, 0, val)
}

// ===== 类型不匹配 =====

func TestGetMapValue_TypeMismatch_StringExpected_GotInt(t *testing.T) {
	m := map[string]any{"key": 42}
	val, ok := GetMapValue[string, string](m, "key")
	require.False(t, ok)
	require.Equal(t, "", val)
}

func TestGetMapValue_TypeMismatch_IntExpected_GotString(t *testing.T) {
	m := map[string]any{"key": "not_int"}
	val, ok := GetMapValue[string, int](m, "key")
	require.False(t, ok)
	require.Equal(t, 0, val)
}

func TestGetMapValue_TypeMismatch_BoolExpected_GotString(t *testing.T) {
	m := map[string]any{"active": "true"} // 字符串 "true" 不是 bool
	val, ok := GetMapValue[string, bool](m, "active")
	require.False(t, ok)
	require.Equal(t, false, val)
}

// ===== int 类型 key =====

func TestGetMapValue_IntKey(t *testing.T) {
	m := map[int]any{1: "one", 2: "two"}
	val, ok := GetMapValue[int, string](m, 1)
	require.True(t, ok)
	require.Equal(t, "one", val)
}

func TestGetMapValue_IntKey_NotFound(t *testing.T) {
	m := map[int]any{1: "one"}
	val, ok := GetMapValue[int, string](m, 99)
	require.False(t, ok)
	require.Equal(t, "", val)
}

// ===== 复杂值类型 =====

func TestGetMapValue_SliceValue(t *testing.T) {
	m := map[string]any{"tags": []string{"go", "test"}}
	val, ok := GetMapValue[string, []string](m, "tags")
	require.True(t, ok)
	require.Equal(t, []string{"go", "test"}, val)
}

func TestGetMapValue_MapValue(t *testing.T) {
	inner := map[string]int{"a": 1, "b": 2}
	m := map[string]any{"inner": inner}
	val, ok := GetMapValue[string, map[string]int](m, "inner")
	require.True(t, ok)
	require.Equal(t, inner, val)
}

func TestGetMapValue_StructValue(t *testing.T) {
	type point struct {
		X, Y int
	}
	m := map[string]any{"origin": point{0, 0}}
	val, ok := GetMapValue[string, point](m, "origin")
	require.True(t, ok)
	require.Equal(t, point{0, 0}, val)
}

func TestGetMapValue_PointerValue(t *testing.T) {
	num := 42
	m := map[string]any{"ptr": &num}
	val, ok := GetMapValue[string, *int](m, "ptr")
	require.True(t, ok)
	require.Equal(t, &num, val)
	require.Equal(t, 42, *val)
}

// ===== 零值存在的情况 =====

func TestGetMapValue_ZeroInt_IsValid(t *testing.T) {
	m := map[string]any{"count": 0}
	val, ok := GetMapValue[string, int](m, "count")
	require.True(t, ok)
	require.Equal(t, 0, val)
}

func TestGetMapValue_EmptyString_IsValid(t *testing.T) {
	m := map[string]any{"name": ""}
	val, ok := GetMapValue[string, string](m, "name")
	require.True(t, ok)
	require.Equal(t, "", val)
}

func TestGetMapValue_False_IsValid(t *testing.T) {
	m := map[string]any{"flag": false}
	val, ok := GetMapValue[string, bool](m, "flag")
	require.True(t, ok)
	require.Equal(t, false, val)
}

// ===== 空 map（非 nil）=====

func TestGetMapValue_EmptyMap(t *testing.T) {
	m := map[string]any{}
	val, ok := GetMapValue[string, string](m, "key")
	require.False(t, ok)
	require.Equal(t, "", val)
}

// ===== any 类型值 =====

func TestGetMapValue_AnyValue(t *testing.T) {
	m := map[string]any{"data": "anything"}
	val, ok := GetMapValue[string, any](m, "data")
	require.True(t, ok)
	require.Equal(t, "anything", val)
}

func TestGetMapValue_InterfaceValue(t *testing.T) {
	m := map[string]any{"num": 123}
	// 用 any 类型取值
	val, ok := GetMapValue[string, any](m, "num")
	require.True(t, ok)
	require.Equal(t, 123, val)
}
