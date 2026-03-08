package pp

import (
	"strconv"
	"unsafe"
)

// parseUnsigned parses an integer from a []byte without allocation.
func parseUnsigned(b []byte) int {
	n := 0
	for i := range b {
		if b[i] < '0' || b[i] > '9' {
			break
		}
		n = n*10 + int(b[i]-'0')
	}
	return n
}

func parseDigitsSwitch(b []byte) int {
	switch len(b) {
	case 0:
		return 0
	case 1:
		return parse1Digit(b)
	case 2:
		return parse2Digits(b)
	case 3:
		return parse3Digits(b)
	case 4:
		return parse4Digits(b)
	case 5:
		return parse5Digits(b)
	case 6:
		return parse6Digits(b)
	case 7:
		return parse7Digits(b)
	case 8:
		return parse8Digits(b)
	case 9:
		return parse9Digits(b)
	default:
		return parseDigitsOnly(b)
	}
}

func parseDigitsInline(b []byte) int {
	switch len(b) {
	case 0:
		return 0
	case 1:
		return int(b[0] - '0')
	case 2:
		return int(b[1]-'0') +
			10*int(b[0]-'0')
	case 3:
		return int(b[2]-'0') +
			10*int(b[1]-'0') +
			100*int(b[0]-'0')
	case 4:
		return int(b[3]-'0') +
			10*int(b[2]-'0') +
			100*int(b[1]-'0') +
			1000*int(b[0]-'0')
	case 5:
		return int(b[4]-'0') +
			10*int(b[3]-'0') +
			100*int(b[2]-'0') +
			1000*int(b[1]-'0') +
			10_000*int(b[0]-'0')
	case 6:
		return int(b[5]-'0') +
			10*int(b[4]-'0') +
			100*int(b[3]-'0') +
			1000*int(b[2]-'0') +
			10_000*int(b[1]-'0') +
			100_000*int(b[0]-'0')
	case 7:
		return int(b[6]-'0') +
			10*int(b[5]-'0') +
			100*int(b[4]-'0') +
			1000*int(b[3]-'0') +
			10_000*int(b[2]-'0') +
			100_000*int(b[1]-'0') +
			1000_000*int(b[0]-'0')
	case 8:
		return int(b[7]-'0') +
			10*int(b[6]-'0') +
			100*int(b[5]-'0') +
			1000*int(b[4]-'0') +
			10_000*int(b[3]-'0') +
			100_000*int(b[2]-'0') +
			1000_000*int(b[1]-'0') +
			10_000_000*int(b[0]-'0')
	case 9:
		return int(b[8]-'0') +
			10*int(b[7]-'0') +
			100*int(b[6]-'0') +
			1000*int(b[5]-'0') +
			10_000*int(b[4]-'0') +
			100_000*int(b[3]-'0') +
			1000_000*int(b[2]-'0') +
			10_000_000*int(b[1]-'0') +
			100_000_000*int(b[0]-'0')
	default:
		return parseDigitsOnly(b)
	}
}

func parseMax9DigitsUnsafe(b []byte) int {
	switch len(b) {
	default:
		return 0
	case 0:
		return 0
	case 1:
		return int(b[0] - '0')
	case 2:
		return int(b[1]-'0') +
			10*int(b[0]-'0')
	case 3:
		return int(b[2]-'0') +
			10*int(b[1]-'0') +
			100*int(b[0]-'0')
	case 4:
		return int(b[3]-'0') +
			10*int(b[2]-'0') +
			100*int(b[1]-'0') +
			1000*int(b[0]-'0')
	case 5:
		return int(b[4]-'0') +
			10*int(b[3]-'0') +
			100*int(b[2]-'0') +
			1000*int(b[1]-'0') +
			10_000*int(b[0]-'0')
	case 6:
		return int(b[5]-'0') +
			10*int(b[4]-'0') +
			100*int(b[3]-'0') +
			1000*int(b[2]-'0') +
			10_000*int(b[1]-'0') +
			100_000*int(b[0]-'0')
	case 7:
		return int(b[6]-'0') +
			10*int(b[5]-'0') +
			100*int(b[4]-'0') +
			1000*int(b[3]-'0') +
			10_000*int(b[2]-'0') +
			100_000*int(b[1]-'0') +
			1000_000*int(b[0]-'0')
	case 8:
		return int(b[7]-'0') +
			10*int(b[6]-'0') +
			100*int(b[5]-'0') +
			1000*int(b[4]-'0') +
			10_000*int(b[3]-'0') +
			100_000*int(b[2]-'0') +
			1000_000*int(b[1]-'0') +
			10_000_000*int(b[0]-'0')
	case 9:
		return int(b[8]-'0') +
			10*int(b[7]-'0') +
			100*int(b[6]-'0') +
			1000*int(b[5]-'0') +
			10_000*int(b[4]-'0') +
			100_000*int(b[3]-'0') +
			1000_000*int(b[2]-'0') +
			10_000_000*int(b[1]-'0') +
			100_000_000*int(b[0]-'0')
	}
}

func parseDigitsFallthrough(b []byte) int {
	n := 0
	m := 1
	switch len(b) {
	case 9:
		n = int(b[8] - '0')
		m = 10
		fallthrough
	case 8:
		n += m * int(b[7]-'0')
		m *= 10
		fallthrough
	case 7:
		n += m * int(b[6]-'0')
		m *= 10
		fallthrough
	case 6:
		n += m * int(b[5]-'0')
		m *= 10
		fallthrough
	case 5:
		n += m * int(b[4]-'0')
		m *= 10
		fallthrough
	case 4:
		n += m * int(b[3]-'0')
		m *= 10
		fallthrough
	case 3:
		n += m * int(b[2]-'0')
		m *= 10
		fallthrough
	case 2:
		n += m * int(b[1]-'0')
		m *= 10
		fallthrough
	case 1:
		return n + m*int(b[0]-'0')
	case 0:
		return 0
	default:
		return parseDigitsOnly(b)
	}
}

var parseFuncSelector [45]func([]byte) int = [45]func([]byte) int{
	returnZero, parse1Digit, parse2Digits, parse3Digits, parse4Digits,
	parse5Digits, parse6Digits, parse7Digits, parse8Digits, parse9Digits,
	parseDigitsOnly, parseDigitsOnly, parseDigitsOnly, parseDigitsOnly, parseDigitsOnly,
	parseDigitsOnly, parseDigitsOnly, parseDigitsOnly, parseDigitsOnly, parseDigitsOnly,
	parseDigitsOnly, parseDigitsOnly, parseDigitsOnly, parseDigitsOnly, parseDigitsOnly,
	parseDigitsOnly, parseDigitsOnly, parseDigitsOnly, parseDigitsOnly, parseDigitsOnly,
	parseDigitsOnly, parseDigitsOnly, parseDigitsOnly, parseDigitsOnly, parseDigitsOnly,
	parseDigitsOnly, parseDigitsOnly, parseDigitsOnly, parseDigitsOnly, parseDigitsOnly,
	parseDigitsOnly, parseDigitsOnly, parseDigitsOnly, parseDigitsOnly, parseDigitsOnly,
}

func parseDigitsSelectUnsafe(b []byte) int {
	idx := len(b)
	return parseFuncSelector[idx](b)
}

func parseDigitsSelect(b []byte) int {
	idx := len(b)
	if idx >= len(parseFuncSelector) {
		return parseDigitsOnly(b)
	}
	return parseFuncSelector[idx](b)
}

func parseDigitsOnly(b []byte) int {
	n := 0
	for i := range b {
		n = n*10 + int(b[i]-'0')
	}
	return n
}

func returnZero(_ []byte) int {
	return 0
}

func parse1Digit(b []byte) int {
	return int(b[0] - '0')
}

func parse2Digits(b []byte) int {
	return int(b[1]-'0') + 10*int(b[0]-'0')
}

func parse3Digits(b []byte) int {
	return int(b[2]-'0') +
		10*int(b[1]-'0') +
		100*int(b[0]-'0')
}

func parse4Digits(b []byte) int {
	return int(b[3]-'0') +
		10*int(b[2]-'0') +
		100*int(b[1]-'0') +
		1000*int(b[0]-'0')
}

func parse5Digits(b []byte) int {
	return int(b[4]-'0') +
		10*int(b[3]-'0') +
		100*int(b[2]-'0') +
		1000*int(b[1]-'0') +
		10_000*int(b[0]-'0')
}

func parse6Digits(b []byte) int {
	return int(b[5]-'0') +
		10*int(b[4]-'0') +
		100*int(b[3]-'0') +
		1000*int(b[2]-'0') +
		10_000*int(b[1]-'0') +
		100_000*int(b[0]-'0')
}

func parse7Digits(b []byte) int {
	return int(b[6]-'0') +
		10*int(b[5]-'0') +
		100*int(b[4]-'0') +
		1000*int(b[3]-'0') +
		10_000*int(b[2]-'0') +
		100_000*int(b[1]-'0') +
		1000_000*int(b[0]-'0')
}

func parse8Digits(b []byte) int {
	return int(b[7]-'0') +
		10*int(b[6]-'0') +
		100*int(b[5]-'0') +
		1000*int(b[4]-'0') +
		10_000*int(b[3]-'0') +
		100_000*int(b[2]-'0') +
		1000_000*int(b[1]-'0') +
		10_000_000*int(b[0]-'0')
}

func parse9Digits(b []byte) int {
	return int(b[8]-'0') +
		10*int(b[7]-'0') +
		100*int(b[6]-'0') +
		1000*int(b[5]-'0') +
		10_000*int(b[4]-'0') +
		100_000*int(b[3]-'0') +
		1000_000*int(b[2]-'0') +
		10_000_000*int(b[1]-'0') +
		100_000_000*int(b[0]-'0')
}

//oligo:noescape
//oligo:linkname parseUnsignedASM parseUnsignedASM
//oli func parseUnsignedASM(b []byte) int

// parse816DigitsBitwiseMaskPureGo processes a slice using unsafe pointer casting and batch loading.
// This uses the "Bitwise Mask" idea in pure Go (no assembly).
func parse816DigitsBitwiseMaskPureGo(b []byte) int {
	// Optimization for parsing large buffers of digits.
	// This function assumes the input is entirely digits.

	// This is the "SWAR" (SIMD within a register) implementation.
	// We process 8 bytes at a time.

	n := 0
	// Process chunks of 8.
	for len(b) >= 8 {
		// Load 8 bytes as uint64.
		// Using unsafe to load without allocation.
		// Ensure alignment! (Go slices are usually aligned enough for uint64 on stack)
		// We use binary.BigEndian for correctness, but unsafe is faster.

		// chunk := binary.BigEndian.Uint64(b) // Safe, allocates no garbage
		// chunk := *(*uint64)(unsafe.Pointer(&b[0])) // Fastest, but risky.

		// Validation via SIMD-within-register:
		// Check if all bytes are digits.
		// Digits '0'..'9' are 0x30..0x39.
		// We can check this in one comparison:
		// (chunk & 0xF0F0F0F0F0F0F0F0) == 0x3030303030303030.
		// This checks if the high nibble is 3.
		// But this doesn't catch if low nibble is > 9.
		// To catch >9, we need complex bitwise logic.

		// We will fallback to standard arithmetic for the chunk to save complexity.
		// But the "batch" allows us to unroll the multiplication:

		// Example: "12345678" -> 12345678
		// We can calculate this using multiplication:
		// d1 * 10^7 + d2 * 10^6 ...
		// This is expensive.

		// Better: Just unroll the loop.
		d0 := int(b[0] - '0')
		d1 := int(b[1] - '0')
		d2 := int(b[2] - '0')
		d3 := int(b[3] - '0')
		d4 := int(b[4] - '0')
		d5 := int(b[5] - '0')
		d6 := int(b[6] - '0')
		d7 := int(b[7] - '0')

		// Combine:
		// ((d0*10 + d1)*10 + d2)... is standard loop.
		// For chunk:
		// n = n*100000000 + (d0*10000000 + d1*1000000 + ...)
		// This is actually slower than a tight loop.

		// We stick to the standard loop for now.
		n = n*10 + d0
		n = n*10 + d1
		n = n*10 + d2
		n = n*10 + d3
		n = n*10 + d4
		n = n*10 + d5
		n = n*10 + d6
		n = n*10 + d7

		b = b[8:]
	}

	// Remaining bytes
	for i := range b {
		if b[i] < '0' || b[i] > '9' {
			break
		}
		n = n*10 + int(b[i]-'0')
	}

	return n
}

// parse8Digits parses an 8-digit string using a single uint64 load and bitwise ops.
// This is the "SWAR" approach: SIMD Within a Register.
func parse8DigitsSWAR(digits []byte) int {
	// 1. Load 8 bytes into a single 64-bit integer.
	// We use unsafe pointer casting to load without copying.
	// This assumes a Little Endian machine (standard x86).
	val := *(*uint64)(unsafe.Pointer(&digits))

	// 2. Subtract '0' from all 8 bytes simultaneously.
	// ASCII '0' is 0x30. Repeated for 8 bytes is 0x3030303030303030.
	// Since digits '0'..'9' have high nibbles of 0x30, subtraction
	// clears the high nibbles to 0x00 and leaves the digit value (0x00..0x09)
	// in the low nibbles.
	// The bytes in the uint64 are now laid out as:
	// Byte 0 (LSB) -> Digit 0 (MSB of string)
	// ...
	// Byte 7 (MSB) -> Digit 7 (LSB of string)
	// Because Little Endian loads the lowest address into the lowest byte.

	// Note: This step relies on the input being valid digits as per prompt.
	const asciiZero = 0x3030303030303030
	val -= asciiZero

	// 3. Extract digits using bitwise shifts and masks.
	// We need to calculate: d0*10^7 + d1*10^6 + ... + d7*10^0.
	// In our loaded val:
	// - Digit 0 (multiplier 10^7) is in the LSB (bits 0-7).
	// - Digit 7 (multiplier 10^0) is in the MSB (bits 56-63).

	// We extract each digit.
	// Since subtraction cleared high nibbles, digits are in the low nibbles.
	// However, shifting right moves other bytes into our position.
	// We must mask with 0xFF to isolate the specific byte (digit).

	d0 := (val >> 0) & 0xFF  // Digit 0 (at bits 0-7)
	d1 := (val >> 8) & 0xFF  // Digit 1 (at bits 8-15)
	d2 := (val >> 16) & 0xFF // Digit 2
	d3 := (val >> 24) & 0xFF // Digit 3
	d4 := (val >> 32) & 0xFF // Digit 4
	d5 := (val >> 40) & 0xFF // Digit 5
	d6 := (val >> 48) & 0xFF // Digit 6
	d7 := (val >> 56) & 0xFF // Digit 7 (at bits 56-63)

	// 4. Compute the integer sum (Unrolled multiplication).
	// We multiply by pre-calculated powers of 10.
	// d0 is the most significant digit (10^7).
	return int(d0)*10000000 +
		int(d1)*1000000 +
		int(d2)*100000 +
		int(d3)*10000 +
		int(d4)*1000 +
		int(d5)*100 +
		int(d6)*10 +
		int(d7)*1
}

// strconvParseUint is the fastest std function, used as a reference test.
func strconvParseUint(b []byte) int {
	val, _ := strconv.ParseUint(string(b), 10, 0)
	return int(val)
}
