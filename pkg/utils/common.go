package utils

const (
	AFTER_MUTATE_CONF = "after-mutate-conf"
)

func Choose(ok bool, first, second string) string {
	if ok {
		return first
	}
	return second
}
