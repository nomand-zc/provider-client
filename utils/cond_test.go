package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// ===== If 函数 =====

func TestIf_True(t *testing.T) {
	result := If(true, "yes", "no")
	require.Equal(t, "yes", result)
}

func TestIf_False(t *testing.T) {
	result := If(false, "yes", "no")
	require.Equal(t, "no", result)
}

func TestIf_IntType(t *testing.T) {
	require.Equal(t, 1, If(true, 1, 2))
	require.Equal(t, 2, If(false, 1, 2))
}

func TestIf_ZeroValues(t *testing.T) {
	// 零值也应该正确返回
	require.Equal(t, 0, If(true, 0, 99))
	require.Equal(t, "", If(false, "hello", ""))
}

func TestIf_NilValues(t *testing.T) {
	var p1 *int
	p2 := ToPtr(42)
	require.Nil(t, If(true, p1, p2))
	require.Equal(t, p2, If(false, p1, p2))
}

func TestIf_BothSidesEvaluated(t *testing.T) {
	// If 不是惰性求值，两侧都会被计算
	sideEffectA := 0
	sideEffectB := 0
	a := func() int { sideEffectA++; return 1 }()
	b := func() int { sideEffectB++; return 2 }()
	_ = If(true, a, b)
	require.Equal(t, 1, sideEffectA)
	require.Equal(t, 1, sideEffectB) // 两侧都已被求值
}

// ===== IfLazy 函数 =====

func TestIfLazy_True(t *testing.T) {
	called := false
	result := IfLazy(true,
		func() string { return "yes" },
		func() string { called = true; return "no" },
	)
	require.Equal(t, "yes", result)
	require.False(t, called, "onFalse 不应被调用")
}

func TestIfLazy_False(t *testing.T) {
	called := false
	result := IfLazy(false,
		func() string { called = true; return "yes" },
		func() string { return "no" },
	)
	require.Equal(t, "no", result)
	require.False(t, called, "onTrue 不应被调用")
}

func TestIfLazy_IntType(t *testing.T) {
	require.Equal(t, 10, IfLazy(true, func() int { return 10 }, func() int { return 20 }))
	require.Equal(t, 20, IfLazy(false, func() int { return 10 }, func() int { return 20 }))
}

func TestIfLazy_ZeroValue(t *testing.T) {
	result := IfLazy(true, func() int { return 0 }, func() int { return 99 })
	require.Equal(t, 0, result)
}

// ===== IfLazyL 函数 =====

func TestIfLazyL_True(t *testing.T) {
	result := IfLazyL(true, func() string { return "lazy_true" }, "eager_false")
	require.Equal(t, "lazy_true", result)
}

func TestIfLazyL_False(t *testing.T) {
	called := false
	result := IfLazyL(false, func() string { called = true; return "lazy_true" }, "eager_false")
	require.Equal(t, "eager_false", result)
	require.False(t, called, "onTrue lazy 不应被调用")
}

func TestIfLazyL_IntType(t *testing.T) {
	require.Equal(t, 100, IfLazyL(true, func() int { return 100 }, 200))
	require.Equal(t, 200, IfLazyL(false, func() int { return 100 }, 200))
}

func TestIfLazyL_ZeroValue(t *testing.T) {
	require.Equal(t, 0, IfLazyL(true, func() int { return 0 }, 42))
	require.Equal(t, 0, IfLazyL(false, func() int { return 99 }, 0))
}

// ===== IfLazyR 函数 =====

func TestIfLazyR_True(t *testing.T) {
	called := false
	result := IfLazyR(true, "eager_true", func() string { called = true; return "lazy_false" })
	require.Equal(t, "eager_true", result)
	require.False(t, called, "onFalse lazy 不应被调用")
}

func TestIfLazyR_False(t *testing.T) {
	result := IfLazyR(false, "eager_true", func() string { return "lazy_false" })
	require.Equal(t, "lazy_false", result)
}

func TestIfLazyR_IntType(t *testing.T) {
	require.Equal(t, 10, IfLazyR(true, 10, func() int { return 20 }))
	require.Equal(t, 20, IfLazyR(false, 10, func() int { return 20 }))
}

func TestIfLazyR_ZeroValue(t *testing.T) {
	require.Equal(t, 0, IfLazyR(true, 0, func() int { return 99 }))
	require.Equal(t, 0, IfLazyR(false, 42, func() int { return 0 }))
}

// ===== Lazy 类型 =====

func TestLazy_TypeAsFunction(t *testing.T) {
	var fn Lazy[string] = func() string { return "lazy_value" }
	require.Equal(t, "lazy_value", fn())
}

func TestLazy_UsedWithIfLazy(t *testing.T) {
	var onTrue Lazy[int] = func() int { return 1 }
	var onFalse Lazy[int] = func() int { return 2 }
	require.Equal(t, 1, IfLazy(true, onTrue, onFalse))
	require.Equal(t, 2, IfLazy(false, onTrue, onFalse))
}
