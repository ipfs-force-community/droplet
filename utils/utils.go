package utils

func ToMap[K comparable, V any](list []V, f func(v V) K) map[K]V {
	m := make(map[K]V)
	for _, v := range list {
		k := f(v)
		m[k] = v
	}
	return m
}
