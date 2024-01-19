package utils

func Slice2Map(s []string) map[string]bool {
	m := map[string]bool{}
	for _, item := range s {
		m[item] = true
	}
	return m
}
