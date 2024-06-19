package utilities

import "testing"

type testCaseAbs struct {
	input, expected int
}

var testCasesAbs = []testCaseAbs{
	testCaseAbs{-4, 4},
	testCaseAbs{1, 1},
	testCaseAbs{-1, 1},
	testCaseAbs{-3, 3},
	testCaseAbs{-10, 10},
}

type testCaseMax struct {
	inputA, inputB, expected int
}

var testCasesMax = []testCaseMax{
	testCaseMax{-4, 3, 3},
	testCaseMax{4, 5, 5},
	testCaseMax{2, 1, 2},
	testCaseMax{10, 4, 10},
	testCaseMax{-1, 1, 1},
}

type testCaseMin struct {
	inputA, inputB, expected int
}

var testCasesMin = []testCaseMin{
	testCaseMin{-4, 3, -4},
	testCaseMin{4, 5, 4},
	testCaseMin{2, 1, 1},
	testCaseMin{10, 4, 4},
	testCaseMin{-1, 1, -1},
}

func TestAbs(t *testing.T) {
	for _, testCase := range testCasesAbs {
		actual := Abs(testCase.input)
		if actual != testCase.expected {
			t.Errorf("Abs doesn't match expected value for %d: got %d, want %d", testCase.input, actual, testCase.expected)
		}
	}
}

func TestMax(t *testing.T) {
	for _, testCase := range testCasesMax {
		actual := Max(testCase.inputA, testCase.inputB)
		if actual != testCase.expected {
			t.Errorf("Max doesn't match expected value for %d & %d: got %d, want %d", testCase.inputA, testCase.inputB, actual, testCase.expected)
		}
	}
}

func TestMin(t *testing.T) {
	for _, testCase := range testCasesMin {
		actual := Min(testCase.inputA, testCase.inputB)
		if actual != testCase.expected {
			t.Errorf("Min doesn't match expected value for %d & %d: got %d, want %d", testCase.inputA, testCase.inputB, actual, testCase.expected)
		}
	}
}

// Max(a, b int)

// Min(a, b int)
