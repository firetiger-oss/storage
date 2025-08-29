package cache_test

import (
	"slices"
	"testing"

	"github.com/firetiger-oss/storage/cache"
)

func TestCache(t *testing.T) {
	c := cache.New[string, string](100)

	dataset := map[string]string{
		"foo": "bar",
		"baz": "qux",
	}

	for k := range dataset {
		v, err := c.Load(k, func() (string, error) {
			return dataset[k], nil
		})
		if err != nil {
			t.Fatal(err)
		}
		if v != dataset[k] {
			t.Errorf("c.Load(%q)=%q, want %q", k, v, dataset[k])
		}
	}
}

func TestSeqCache(t *testing.T) {
	c := cache.Seq[string, string](100)

	dataset := map[string][]string{
		"foo": {"bar", "baz"},
		"baz": {"A", "B", "C"},
	}

	for k := range dataset {
		var values []string
		for v, err := range c.Load(k, func(yield func(string, error) bool) {
			for _, v := range dataset[k] {
				if !yield(v, nil) {
					return
				}
			}
		}) {
			if err != nil {
				t.Fatal(err)
			}
			values = append(values, v)
		}
		slices.Sort(values)
		if !slices.Equal(values, dataset[k]) {
			t.Errorf("c.Load(%q)=%q, want %q", k, values, dataset[k])
		}
	}

}
