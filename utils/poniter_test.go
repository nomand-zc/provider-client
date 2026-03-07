package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestToPtr_Int(t *testing.T) {
	p := ToPtr(42)
	require.NotNil(t, p)
	require.Equal(t, 42, *p)
}

func TestToPtr_IntZero(t *testing.T) {
	p := ToPtr(0)
	require.NotNil(t, p)
	require.Equal(t, 0, *p)
}

func TestToPtr_NegativeInt(t *testing.T) {
	p := ToPtr(-100)
	require.NotNil(t, p)
	require.Equal(t, -100, *p)
}

func TestToPtr_String(t *testing.T) {
	p := ToPtr("hello")
	require.NotNil(t, p)
	require.Equal(t, "hello", *p)
}

func TestToPtr_EmptyString(t *testing.T) {
	p := ToPtr("")
	require.NotNil(t, p)
	require.Equal(t, "", *p)
}

func TestToPtr_Bool_True(t *testing.T) {
	p := ToPtr(true)
	require.NotNil(t, p)
	require.Equal(t, true, *p)
}

func TestToPtr_Bool_False(t *testing.T) {
	p := ToPtr(false)
	require.NotNil(t, p)
	require.Equal(t, false, *p)
}

func TestToPtr_Float64(t *testing.T) {
	p := ToPtr(3.14)
	require.NotNil(t, p)
	require.InDelta(t, 3.14, *p, 1e-9)
}

func TestToPtr_Float64Zero(t *testing.T) {
	p := ToPtr(0.0)
	require.NotNil(t, p)
	require.Equal(t, 0.0, *p)
}

func TestToPtr_Struct(t *testing.T) {
	type person struct {
		Name string
		Age  int
	}
	v := person{Name: "Alice", Age: 30}
	p := ToPtr(v)
	require.NotNil(t, p)
	require.Equal(t, v, *p)

	// 修改原值不影响指针指向的值（值拷贝语义）
	v.Name = "Bob"
	require.Equal(t, "Alice", p.Name)
}

func TestToPtr_Slice(t *testing.T) {
	s := []int{1, 2, 3}
	p := ToPtr(s)
	require.NotNil(t, p)
	require.Equal(t, []int{1, 2, 3}, *p)
}

func TestToPtr_NilSlice(t *testing.T) {
	var s []int
	p := ToPtr(s)
	require.NotNil(t, p)
	require.Nil(t, *p)
}

func TestToPtr_Map(t *testing.T) {
	m := map[string]int{"a": 1}
	p := ToPtr(m)
	require.NotNil(t, p)
	require.Equal(t, map[string]int{"a": 1}, *p)
}

func TestToPtr_NilMap(t *testing.T) {
	var m map[string]int
	p := ToPtr(m)
	require.NotNil(t, p)
	require.Nil(t, *p)
}

func TestToPtr_Interface(t *testing.T) {
	var v any = "interface_value"
	p := ToPtr(v)
	require.NotNil(t, p)
	require.Equal(t, "interface_value", *p)
}

func TestToPtr_NilInterface(t *testing.T) {
	var v any
	p := ToPtr(v)
	require.NotNil(t, p)
	require.Nil(t, *p)
}

func TestToPtr_ReturnsDifferentPointers(t *testing.T) {
	// 每次调用应返回不同的指针地址
	p1 := ToPtr(1)
	p2 := ToPtr(1)
	require.NotSame(t, p1, p2)
	require.Equal(t, *p1, *p2)
}
