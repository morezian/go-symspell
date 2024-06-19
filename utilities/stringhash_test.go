package utilities

import (
	"math"
	"testing"
)

var compactMask uint = (math.MaxUint >> (3 + 5)) << 2

type testCase struct {
	source string
	hash   uint
}

var testStrings []testCase = []testCase{
	{"test", 137050753280518951},
	{"test2", 253748902539114519},
	{"tester", 115586007494858103},
	{"ardvark", 242448674802442711},
	{"Test", 32906162972958471},
	{"Test2", 235707456239602295},
	{"blarg", 226522123152123963},
	{"日本語", 100391766260962863},
}

func TestHashing(t *testing.T) {
	for _, test := range testStrings {
		if h := GetStringHash(test.source, compactMask); test.hash != h {
			t.Errorf("hash doesn't match expected hash for '%s': got %d, want %d", test.source, h, test.hash)
		}
	}
}
