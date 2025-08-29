package sequtil

import "iter"

// All consumes an iterator of [T, error], producing a []T, error. It
// stops reading on the first error it encounters.
func All[T any](seq iter.Seq2[T, error]) ([]T, error) {
	var slice []T
	for elem, err := range seq {
		if err != nil {
			return slice, err
		}
		slice = append(slice, elem)
	}
	return slice, nil
}

func Limit[T any](seq iter.Seq2[T, error], limit int) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		remain := limit
		for v, err := range seq {
			if err != nil {
				yield(v, err)
				return
			}
			if !yield(v, nil) {
				return
			}
			if remain--; remain == 0 {
				return
			}
		}
	}
}

func Transform[T any](seq iter.Seq2[T, error], transform func(T) (T, error)) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		for v, err := range seq {
			if err == nil {
				v, err = transform(v)
			}
			if !yield(v, err) {
				return
			}
		}
	}
}
