package romanizer

import (
	"testing"
)

func TestRomanize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: "",
		},
		{
			name:     "already latin",
			input:    "Hello World",
			expected: "",
		},
		{
			name:     "latin with punctuation",
			input:    "Hello, World!",
			expected: "",
		},
		{
			name:     "japanese",
			input:    "こんにちは",
			expected: "konnichiha",
		},
		{
			name:     "korean",
			input:    "안녕하세요",
			expected: "annyeonghaseyo",
		},
		{
			name:     "chinese",
			input:    "你好",
			expected: "Ni Hao",
		},
		{
			name:     "mixed latin and japanese",
			input:    "Hello 世界",
			expected: "Hello Shi Jie",
		},
		{
			name:     "leading and trailing whitespace",
			input:    "   こんにちは   ",
			expected: "konnichiha",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Romanize(tt.input)
			if result != tt.expected {
				t.Errorf("Romanize(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
