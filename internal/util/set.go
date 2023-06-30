package util

func Intersect[T any](left map[string]T, right map[string]T) map[string]T {
	if left == nil {
		return right
	}
	s_intersection := map[string]T{}
	if len(left) > len(right) {
		left, right = right, left // better to iterate over a shorter set
	}

	for k, _ := range left {
		if val, ok := right[k]; ok {
			s_intersection[k] = val
		}
	}

	return s_intersection
}

func Keys[T any](dict map[string]T) []string {
	ret := []string{}
	for key := range dict {
		ret = append(ret, key)
	}

	return ret
}
