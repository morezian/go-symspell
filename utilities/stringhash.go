package utilities

const (
	offsetBasis uint = 14695981039346656037
	prime       uint = 1099511628211
)

func GetStringHash(s string, compactMask uint) int {
	length := len(s)
	lenMask := length
	if lenMask > 3 {
		lenMask = 3
	}

	var hash uint = offsetBasis

	for _, c := range s {
		hash ^= uint(c)
		hash *= prime
	}

	hash &= compactMask
	hash |= uint(lenMask)
	return int(hash)
}
