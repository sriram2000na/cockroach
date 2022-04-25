// Copyright 2016 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

package util

import (
	"bytes"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetSingleRune(t *testing.T) {
	tests := []struct {
		s        string
		expected rune
		err      bool
	}{
		{"a", 'a', false},
		{"", 0, false},
		{"🐛"[:1], 0, true},
		{"aa", 'a', true},
	}
	for _, tc := range tests {
		t.Run(tc.s, func(t *testing.T) {
			got, err := GetSingleRune(tc.s)
			if (err != nil) != tc.err {
				t.Fatalf("got unexpected err: %v", err)
			}
			if tc.expected != got {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func TestToLowerSingleByte(t *testing.T) {
	testCases := []struct {
		from     byte
		expected byte
	}{
		{'a', 'a'},
		{'A', 'a'},
		{'c', 'c'},
		{'C', 'c'},
		{'Z', 'z'},
		{'1', '1'},
		{'\n', '\n'},
	}

	for _, tc := range testCases {
		t.Run(string(tc.from), func(t *testing.T) {
			ret := ToLowerSingleByte(tc.from)
			require.Equal(t, tc.expected, ret)
		})
	}
}

func TestTruncateString(t *testing.T) {
	testCases := []struct {
		s string
		// res stores the expected results for maxRunes=0,1,2,3,etc.
		res []string
	}{
		{"", []string{"", ""}},
		{"abcd", []string{"", "a", "ab", "abc", "abcd", "abcd", "abcd"}},
		{"🐛🏠", []string{"", "🐛", "🐛🏠", "🐛🏠", "🐛🏠"}},
		{"a🐛b🏠c", []string{"", "a", "a🐛", "a🐛b", "a🐛b🏠", "a🐛b🏠c", "a🐛b🏠c"}},
		{
			// Test with an invalid UTF-8 sequence.
			"\xf0\x90\x28\xbc",
			[]string{"", "\xf0", "\xf0\x90", "\xf0\x90\x28", "\xf0\x90\x28\xbc", "\xf0\x90\x28\xbc"},
		},
	}

	for _, tc := range testCases {
		for i := range tc.res {
			if r := TruncateString(tc.s, i); r != tc.res[i] {
				t.Errorf("TruncateString(\"%q\", %d) = \"%q\"; expected \"%q\"", tc.s, i, r, tc.res[i])
			}
		}
	}
}

func TestRemoveTrailingSpaces(t *testing.T) {
	for _, tc := range []struct{ input, expected string }{
		{
			input:    "",
			expected: "\n",
		},
		{
			input:    "line 1  \nline 2   \nline 3 \n\n",
			expected: "line 1\nline 2\nline 3\n",
		},
		{
			input:    " line 1  \nline 2   \nline 3  ",
			expected: " line 1\nline 2\nline 3\n",
		},
		{
			input:    "line 1\n\n  \nline 2   \nline 3",
			expected: "line 1\n\n\nline 2\nline 3\n",
		},
	} {
		output := RemoveTrailingSpaces(tc.input)
		if output != tc.expected {
			t.Errorf("expected:\n%s\ngot:\n%s", tc.expected, output)
		}
	}
}

func TestStringListBuilder(t *testing.T) {
	var buf bytes.Buffer
	var b StringListBuilder
	expect := func(exp string) {
		t.Helper()
		if buf.String() != exp {
			t.Errorf("expected `%s`, got `%s`", exp, buf.String())
		}
		buf.Reset()
	}

	b = MakeStringListBuilder("(", ",", ")")
	b.Finish(&buf)
	expect("")

	b = MakeStringListBuilder("(", ",", ")")
	b.Add(&buf, "one")
	b.Finish(&buf)
	expect("(one)")

	b = MakeStringListBuilder("[", ", ", "]")
	b.Add(&buf, "one")
	b.Addf(&buf, "%s", "two")
	b.Finish(&buf)
	expect("[one, two]")
}

func TestCollapseRepeatedChar(t *testing.T) {
	type StringTest struct {
		ToTest   string
		DupeChar rune
		Expected string
	}

	tests := []StringTest{
		{"%test%", '%', "%test%"},
		{"%test%%%%", '%', "%test%"},
		{"%%%test%%%%", '%', "%test%"},
		{"%%%test1%%%test2%%%test3%%%", '%', "%test1%test2%test3%"},
		{"I work on ddddddifferent characters", 'd', "I work on different characters"},
		{"%%%%%%%%tèʂt%", '%', "%tèʂt%"},
		{"%a%b%%c%%d", '%', "%a%b%c%d"},
		{"🐛🐛", '🐛', "🐛"},
	}

	for _, test := range tests {
		result := CollapseRepeatedChar(test.ToTest, test.DupeChar)

		if result != test.Expected {
			t.Errorf("expected %s but got %s", test.Expected, result)
		}
	}
}

func BenchmarkCollapseRepeatedChar(b *testing.B) {
	for _, runFn := range []func(*testing.B){
		runBenchmarkNoDupe,
		runBenchmarkSingleDupe,
		runBenchmarkMultipleDupe,
		runBenchmarkSpacedDupe,
	} {
		fnName := runtime.FuncForPC(reflect.ValueOf(runFn).Pointer()).Name()
		fnName = strings.TrimPrefix(fnName, "github.com/cockroachdb/cockroach/pkg/util.runBenchmark")
		b.Run(fnName, func(b *testing.B) {
			for _, count := range []int{1, 10, 100, 1000} {
				b.Run(fmt.Sprintf("count=%d", count), func(b *testing.B) {
					runFn(b)
				})
			}
		})
	}
}

func runBenchmarkNoDupe(b *testing.B) {
	toTest := "%test%"

	for n := 0; n < b.N; n++ {
		CollapseRepeatedChar(toTest, '%')
	}
}

func runBenchmarkSingleDupe(b *testing.B) {
	toTest := "%test%%%%"

	for n := 0; n < b.N; n++ {
		CollapseRepeatedChar(toTest, '%')
	}
}

func runBenchmarkMultipleDupe(b *testing.B) {
	toTest := "%%%%%test%%%%"

	for n := 0; n < b.N; n++ {
		CollapseRepeatedChar(toTest, '%')
	}
}

func runBenchmarkSpacedDupe(b *testing.B) {
	toTest := "%%%spaced%%%dupe%%%"

	for n := 0; n < b.N; n++ {
		CollapseRepeatedChar(toTest, '%')
	}
}
