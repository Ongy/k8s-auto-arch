package util

func Intersect(left map[string]bool, right map[string]bool) map[string]bool {
	if left == nil {
		return right
	}
	s_intersection := map[string]bool{}
	if len(left) > len(right) {
		left, right = right, left // better to iterate over a shorter set
	}

	for k, _ := range left {
		if right[k] {
			s_intersection[k] = true
		}
	}

	return s_intersection
}

func Keys(dict map[string]bool) []string {
	ret := make([]string, len(dict))
	for key := range dict {
		ret = append(ret, key)
	}

	return ret
}
