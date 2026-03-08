package kafka

import (
	"testing"
)

// ---------------------------------------------------------------------------
// truncateBytes
// ---------------------------------------------------------------------------

func TestTruncateBytes_ShortInput(t *testing.T) {
	result := truncateBytes([]byte("hello"), 10)
	if result != "hello" {
		t.Errorf("truncateBytes short input = %q, want %q", result, "hello")
	}
}

func TestTruncateBytes_ExactLength(t *testing.T) {
	result := truncateBytes([]byte("12345"), 5)
	if result != "12345" {
		t.Errorf("truncateBytes exact length = %q, want %q", result, "12345")
	}
}

func TestTruncateBytes_Truncated(t *testing.T) {
	result := truncateBytes([]byte("hello world"), 5)
	expected := "hello...(truncated)"
	if result != expected {
		t.Errorf("truncateBytes truncated = %q, want %q", result, expected)
	}
}

func TestTruncateBytes_EmptyInput(t *testing.T) {
	result := truncateBytes([]byte{}, 10)
	if result != "" {
		t.Errorf("truncateBytes empty = %q, want empty", result)
	}
}

func TestTruncateBytes_ZeroMaxLen(t *testing.T) {
	result := truncateBytes([]byte("hello"), 0)
	expected := "...(truncated)"
	if result != expected {
		t.Errorf("truncateBytes zero maxLen = %q, want %q", result, expected)
	}
}

func TestTruncateBytes_EmptyInputZeroMaxLen(t *testing.T) {
	result := truncateBytes([]byte{}, 0)
	if result != "" {
		t.Errorf("truncateBytes empty+zero = %q, want empty", result)
	}
}

func TestTruncateBytes_SingleByte(t *testing.T) {
	result := truncateBytes([]byte("a"), 1)
	if result != "a" {
		t.Errorf("truncateBytes single byte = %q, want %q", result, "a")
	}
}

func TestTruncateBytes_SingleByteOverLimit(t *testing.T) {
	result := truncateBytes([]byte("ab"), 1)
	expected := "a...(truncated)"
	if result != expected {
		t.Errorf("truncateBytes over by one = %q, want %q", result, expected)
	}
}

func TestTruncateBytes_LargeInput(t *testing.T) {
	// 1000 bytes truncated to 256
	input := make([]byte, 1000)
	for i := range input {
		input[i] = 'x'
	}
	result := truncateBytes(input, 256)
	if len(result) != 256+len("...(truncated)") {
		t.Errorf("truncateBytes large input length = %d, want %d", len(result), 256+len("...(truncated)"))
	}
}

func TestTruncateBytes_NilInput(t *testing.T) {
	result := truncateBytes(nil, 10)
	if result != "" {
		t.Errorf("truncateBytes nil = %q, want empty", result)
	}
}

func TestTruncateBytes_BinaryData(t *testing.T) {
	input := []byte{0x00, 0xFF, 0x01, 0xFE, 0x02}
	result := truncateBytes(input, 3)
	// Should truncate at 3 bytes and append suffix
	if len(result) < 3 {
		t.Errorf("truncateBytes binary data too short: %d", len(result))
	}
}
