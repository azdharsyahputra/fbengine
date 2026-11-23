// Ported from Player12 (Go-only support types replacing std::optional and std::pair) (GPL-3.0)

package engine

// Optional mirrors std::optional<T> as a value type (copyable with the
// engine). Zero value = empty (std::nullopt).
//
//	opt.has_value()   -> opt.HasValue
//	*opt / opt.value() -> opt.Value
//	opt = v           -> opt = hSome(v)
//	opt.reset()       -> opt = Optional[T]{}
//	opt.value_or(x)   -> opt.ValueOr(x)
type Optional[T any] struct {
	Value    T
	HasValue bool
}

// hSome wraps a value in a present Optional.
func hSome[T any](value T) Optional[T] { return Optional[T]{Value: value, HasValue: true} }

// ValueOr returns the value when present, else fallback.
func (o Optional[T]) ValueOr(fallback T) T {
	if o.HasValue {
		return o.Value
	}
	return fallback
}

// Pair mirrors std::pair<A, B>.
type Pair[A any, B any] struct {
	First  A
	Second B
}
