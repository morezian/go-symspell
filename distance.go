package symspell

import (
	"errors"
	"github.com/hbollon/go-edlib"
)

func Distance(a, b string, algorithm edlib.Algorithm) (int, error) {
	switch algorithm {
	case edlib.OSADamerauLevenshtein:
		return edlib.OSADamerauLevenshteinDistance(a, b), nil
	case edlib.DamerauLevenshtein:
		return edlib.DamerauLevenshteinDistance(a, b), nil
	case edlib.Levenshtein:
		return edlib.LevenshteinDistance(a, b), nil
	}

	return -1, errors.New("invalid algorithm")
}
