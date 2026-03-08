package pp

import (
	"log"
	"math/rand"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

type testCase struct {
	buf []byte
	val uint64
}

// Helper slice to access slices by length
var numbersByDigitsCount [19][]testCase

func init() {
	numbersByDigitsCount[0] = []testCase{{buf: nil, val: 0}}
	for i := range len(numbersByDigitsCount) - 1 {
		digitsCount := i + 1
		numbersByDigitsCount[digitsCount] = generateRandomASCIINumbers(digitsCount)
	}
}

// generateRandomASCIINumbers generates different byte slices representing numbers.
func generateRandomASCIINumbers(digitsCount int) []testCase {
	generator := rand.New(rand.NewSource(0)) // reproducible set of numbers
	const size = 50                          // size of the test set
	numbers := make([]testCase, 0, size)
	for range cap(numbers) {
		b := make([]byte, 0, digitsCount)
		for range digitsCount {
			asciiDigit := byte('0' + generator.Intn(10)) // random ASCII digit
			b = append(b, asciiDigit)
		}
		v, err := strconv.ParseUint(string(b), 10, 0)
		if err != nil {
			log.Fatalf("generateRandomASCIINumbers strconv.Atoi: %v", err)
		}
		numbers = append(numbers, testCase{buf: b, val: v})
	}
	return numbers
}

// globalSink prevents compiler from eliminating the function calls
var globalSink int

// bench runs a sub-benchmark for a specific parser function.
// this permit to guarantee all benchmark are implemented in the same way.
func bench(b *testing.B, fn func([]byte) int, minDigits, maxDigits int) {
	// zero maxDigits is a shortcut for max authorized value
	if maxDigits <= 0 || maxDigits >= len(numbersByDigitsCount) {
		maxDigits = len(numbersByDigitsCount) - 1
	}

	// 1. Get the pointer to the function value using reflect.
	// 2. Use runtime.FuncForPC to resolve the function's name from its program counter.
	// This is the robust way to get the name for any function value.
	f := runtime.FuncForPC(reflect.ValueOf(fn).Pointer())
	name := f.Name()

	// Clean up the name (remove package prefix or generic suffixes)
	// Example: "github.com/user/pkg.parse1Digit" -> "parse1Digit"
	idx := strings.LastIndex(name, ".")
	if idx != -1 {
		name = name[idx+1:]
	}

	for digitsCount := minDigits; digitsCount <= maxDigits; digitsCount++ {
		if digitsCount >= len(numbersByDigitsCount) {
			b.Errorf("digitsCount=%d is greater than len(numbersByDigitsCount)=%d", digitsCount, len(numbersByDigitsCount))
			return
		}
		numbers := numbersByDigitsCount[digitsCount]

		run := name + "-" + strconv.Itoa(digitsCount) + "-digits"
		b.Run(run, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				asciiDigits := numbers[i%len(numbers)].buf
				globalSink = fn(asciiDigits)
			}
		})
	}
}

// test mirrors the [bench] function
func test(t *testing.T, fn func([]byte) int, minDigits, maxDigits int) {
	if maxDigits <= 0 || maxDigits >= len(numbersByDigitsCount) {
		maxDigits = len(numbersByDigitsCount) - 1
	}

	f := runtime.FuncForPC(reflect.ValueOf(fn).Pointer())
	name := f.Name()

	idx := strings.LastIndex(name, ".")
	if idx != -1 {
		name = name[idx+1:]
	}

	for digitsCount := minDigits; digitsCount <= maxDigits; digitsCount++ {
		if digitsCount >= len(numbersByDigitsCount) {
			t.Errorf("%s() digitsCount=%d is greater than or equal to len(numbersByDigitsCount)=%d", name, digitsCount, len(numbersByDigitsCount))
			return
		}
		numbers := numbersByDigitsCount[digitsCount]

		run := name + "-" + strconv.Itoa(digitsCount) + "-digits"
		t.Run(run, func(t *testing.T) {
			t.Parallel()

			for _, tc := range numbers {
				got := fn(tc.buf)
				gotU64 := uint64(got)
				if gotU64 != tc.val {
					t.Errorf("got %d, want %d", got, tc.val)
				}
			}
		})
	}
}

var tests = []struct {
	fn     func([]byte) int // function pointer
	dm, dM int              // min/max digitsCount = number of digits (length in bytes of the []byte buffer passed to the parser)
}{
	// Fixed Length Parsers
	{dm: 1, dM: 1, fn: parse1Digit},
	{dm: 2, dM: 2, fn: parse2Digits},
	{dm: 3, dM: 3, fn: parse3Digits},
	{dm: 4, dM: 4, fn: parse4Digits},
	{dm: 5, dM: 5, fn: parse5Digits},
	{dm: 6, dM: 6, fn: parse6Digits},
	{dm: 7, dM: 7, fn: parse7Digits},
	{dm: 8, dM: 8, fn: parse8Digits},
	{dm: 9, dM: 9, fn: parse9Digits},
	// Generic Parsers
	{dm: 0, dM: 0, fn: parseDigitsSwitch},
	{dm: 0, dM: 0, fn: parseDigitsInline},
	{dm: 0, dM: 0, fn: parseDigitsFallthrough},
	{dm: 0, dM: 0, fn: parseDigitsOnly},
	{dm: 0, dM: 0, fn: parseUnsigned},
	{dm: 0, dM: 0, fn: parseDigitsSelect},
	{dm: 0, dM: 0, fn: strconvParseUint},
	// Parsers that panic: index out of range
	{dm: 0, dM: 9, fn: parseMax9DigitsUnsafe},
	{dm: 0, dM: len(parseFuncSelector), fn: parseDigitsSelectUnsafe},
	//TODO // Experimental parsers
	// {dm: 1, dM: 1, fn: parseUnsignedASM},
	// {dm: 8, dM: 8, fn: parse8DigitsSWAR},
	// {dm: 8, dM: 8, fn: parse816DigitsBitwiseMaskPureGo},
	// {dm: 16,dM:  16, fn: parse816DigitsBitwiseMaskPureGo},
}

func BenchmarkParsers(b *testing.B) {
	for _, tt := range tests {
		bench(b, tt.fn, tt.dm, tt.dM)
	}
}

func TestParsers(t *testing.T) {
	t.Parallel()
	for _, tt := range tests {
		test(t, tt.fn, tt.dm, tt.dM)
	}
}

func TestGenericParsers(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"5", 5},
		{"12", 12},
		{"123", 123},
		{"1234", 1234},
		{"12345", 12345},
		{"123456", 123456},
		{"1234567", 1234567},
		{"12345678", 12345678},
		{"123456789", 123456789},
		{"1234567890", 1234567890},
		{"987654321", 987654321},
	}

	// Functions that are safe and generic (variable length)
	parsers := []struct {
		name string
		fn   func([]byte) int
	}{
		{"parseUnsignedSafe", parseUnsigned},
		{"parseDigitsOnly", parseDigitsOnly},
		{"parseDigitsSwitch", parseDigitsSwitch},
		{"parseDigitsInline", parseDigitsInline},
		{"parseDigitsSelect", parseDigitsSelect},
		{"parseDigitsSelectSafe", parseDigitsSelect},
		{"parseDigitsFallthrough", parseDigitsFallthrough},
	}

	for _, p := range parsers {
		t.Run(p.name, func(t *testing.T) {
			for _, tc := range tests {
				b := []byte(tc.input)
				got := p.fn(b)
				if got != tc.want {
					t.Errorf("%s(%q) = %d; want %d", p.name, tc.input, got, tc.want)
				}
			}
		})
	}
}

func TestFixedLengthParsers(t *testing.T) {
	t.Run("parse1Digit", func(t *testing.T) {
		if parse1Digit([]byte("1")) != 1 {
			t.Error("parse1Digit failed")
		}
		if parse1Digit([]byte("5")) != 5 {
			t.Error("parse1Digit failed")
		}
	})

	t.Run("parse2Digits", func(t *testing.T) {
		if parse2Digits([]byte("12")) != 12 {
			t.Error("parse2Digits failed")
		}
	})

	t.Run("parse3Digits", func(t *testing.T) {
		if parse3Digits([]byte("123")) != 123 {
			t.Error("parse3Digits failed")
		}
	})

	t.Run("parse4Digits", func(t *testing.T) {
		if parse4Digits([]byte("1234")) != 1234 {
			t.Error("parse4Digits failed")
		}
	})

	t.Run("parse5Digits", func(t *testing.T) {
		if parse5Digits([]byte("12345")) != 12345 {
			t.Error("parse5Digits failed")
		}
	})

	t.Run("parse6Digits", func(t *testing.T) {
		if parse6Digits([]byte("123456")) != 123456 {
			t.Error("parse6Digits failed")
		}
	})

	t.Run("parse7Digits", func(t *testing.T) {
		if parse7Digits([]byte("1234567")) != 1234567 {
			t.Error("parse7Digits failed")
		}
	})

	t.Run("parse8Digits", func(t *testing.T) {
		if parse8Digits([]byte("12345678")) != 12345678 {
			t.Error("parse8Digits failed")
		}
	})

	t.Run("parse9Digits", func(t *testing.T) {
		if parse9Digits([]byte("123456789")) != 123456789 {
			t.Error("parse9Digits failed")
		}
	})
}

// TestBatchParsers tests the unsafe batch parsing logic.
func TestBatchParsers(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"8D", "12345678", 12345678},
		{"16D", "1234567812345678", 1234567812345678}, // Large number
	}

	t.Run("parse816DigitsBitwiseMaskPureGo", func(t *testing.T) {
		for _, tc := range tests {
			b := []byte(tc.input)
			// We use parseDigitsOnly as a reference implementation
			// to compare against the batch version.
			ref := parseDigitsOnly(b)
			got := parse816DigitsBitwiseMaskPureGo(b)
			if got != ref {
				t.Errorf("parseUnsignedBatch(%q) = %d; want %d", tc.input, got, ref)
			}
		}
	})
}

// TODO: TestUnsafeParsers tests parse8DigitsSWAR.
func DisableTestUnsafeParsers(t *testing.T) {
	t.Run("parse8DigitsSWAR", func(t *testing.T) {
		b := []byte("12345678")
		got := parse8DigitsSWAR(b)
		want := 12345678
		if got != want {
			t.Errorf("parse8DigitsSWAR = %d; want %d", got, want)
		}
		// Test another number
		b = []byte("87654321")
		got = parse8DigitsSWAR(b)
		want = 87654321
		if got != want {
			t.Errorf("parse8DigitsSWAR = %d; want %d", got, want)
		}
	})
}

// TestDigitsFallthrough tests the fallthrough implementation specifically.
func TestDigitsFallthrough(t *testing.T) {
	t.Run("parseDigitsFallthrough_BUG", func(t *testing.T) {
		got := parseDigitsFallthrough([]byte("12"))
		if got != 12 {
			t.Errorf("parseDigitsFallthrough('12') returned %d; expected 12", got)
		}
	})
}
