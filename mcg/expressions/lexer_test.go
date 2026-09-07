// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package expressions_test

import (
	"math/big"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"sdv.googlesource.com/mcg/mcg/expressions"
)

var equateBigInt = cmp.Comparer(func(x, y *big.Int) bool {
	if x == nil || y == nil {
		return x == y
	}
	return x.Cmp(y) == 0
})

func collectTokens(stream *expressions.TokenStream) []expressions.Token {
	var tokens []expressions.Token
	for !stream.IsEOF() {
		tokens = append(tokens, stream.Next())
	}
	return tokens
}

func TestLexer_TokensAndSpans(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  []expressions.Token
	}{
		{
			name:  "single_line",
			input: "pub.field + 42 - true && 3.14",
			want: []expressions.Token{
				{
					Kind:  expressions.TokenIdentifier,
					Value: "pub.field",
					Span: expressions.Span{
						Start: expressions.Position{Line: 1, Column: 1, Offset: 0},
						End:   expressions.Position{Line: 1, Column: 10, Offset: 9},
					},
				},
				{
					Kind:  expressions.TokenPlus,
					Value: nil,
					Span: expressions.Span{
						Start: expressions.Position{Line: 1, Column: 11, Offset: 10},
						End:   expressions.Position{Line: 1, Column: 12, Offset: 11},
					},
				},
				{
					Kind:  expressions.TokenNumber,
					Value: big.NewInt(42),
					Span: expressions.Span{
						Start: expressions.Position{Line: 1, Column: 13, Offset: 12},
						End:   expressions.Position{Line: 1, Column: 15, Offset: 14},
					},
				},
				{
					Kind:  expressions.TokenMinus,
					Value: nil,
					Span: expressions.Span{
						Start: expressions.Position{Line: 1, Column: 16, Offset: 15},
						End:   expressions.Position{Line: 1, Column: 17, Offset: 16},
					},
				},
				{
					Kind:  expressions.TokenBool,
					Value: true,
					Span: expressions.Span{
						Start: expressions.Position{Line: 1, Column: 18, Offset: 17},
						End:   expressions.Position{Line: 1, Column: 22, Offset: 21},
					},
				},
				{
					Kind:  expressions.TokenAmpAmp,
					Value: nil,
					Span: expressions.Span{
						Start: expressions.Position{Line: 1, Column: 23, Offset: 22},
						End:   expressions.Position{Line: 1, Column: 25, Offset: 24},
					},
				},
				{
					Kind:  expressions.TokenNumber,
					Value: float64(3.14),
					Span: expressions.Span{
						Start: expressions.Position{Line: 1, Column: 26, Offset: 25},
						End:   expressions.Position{Line: 1, Column: 30, Offset: 29},
					},
				},
			},
		},
		{
			name:  "multiline",
			input: "pub.field +\n  42 - true &&\n  3.14",
			want: []expressions.Token{
				{
					Kind:  expressions.TokenIdentifier,
					Value: "pub.field",
					Span: expressions.Span{
						Start: expressions.Position{Line: 1, Column: 1, Offset: 0},
						End:   expressions.Position{Line: 1, Column: 10, Offset: 9},
					},
				},
				{
					Kind:  expressions.TokenPlus,
					Value: nil,
					Span: expressions.Span{
						Start: expressions.Position{Line: 1, Column: 11, Offset: 10},
						End:   expressions.Position{Line: 1, Column: 12, Offset: 11},
					},
				},
				{
					Kind:  expressions.TokenNumber,
					Value: big.NewInt(42),
					Span: expressions.Span{
						Start: expressions.Position{Line: 2, Column: 3, Offset: 14},
						End:   expressions.Position{Line: 2, Column: 5, Offset: 16},
					},
				},
				{
					Kind:  expressions.TokenMinus,
					Value: nil,
					Span: expressions.Span{
						Start: expressions.Position{Line: 2, Column: 6, Offset: 17},
						End:   expressions.Position{Line: 2, Column: 7, Offset: 18},
					},
				},
				{
					Kind:  expressions.TokenBool,
					Value: true,
					Span: expressions.Span{
						Start: expressions.Position{Line: 2, Column: 8, Offset: 19},
						End:   expressions.Position{Line: 2, Column: 12, Offset: 23},
					},
				},
				{
					Kind:  expressions.TokenAmpAmp,
					Value: nil,
					Span: expressions.Span{
						Start: expressions.Position{Line: 2, Column: 13, Offset: 24},
						End:   expressions.Position{Line: 2, Column: 15, Offset: 26},
					},
				},
				{
					Kind:  expressions.TokenNumber,
					Value: float64(3.14),
					Span: expressions.Span{
						Start: expressions.Position{Line: 3, Column: 3, Offset: 29},
						End:   expressions.Position{Line: 3, Column: 7, Offset: 33},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stream, err := expressions.Lex(strings.NewReader(tc.input))
			if err != nil {
				t.Fatalf("unexpected lex error: %v", err)
			}

			got := collectTokens(stream)
			if diff := cmp.Diff(tc.want, got, equateBigInt); diff != "" {
				t.Errorf("Lex(%q) mismatch (-want +got):\n%s", tc.input, diff)
			}
		})
	}
}

func TestLexer_ReaderInterface(t *testing.T) {
	input := "10 + 20"
	stream, err := expressions.Lex(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected lex error: %v", err)
	}

	want := []expressions.Token{
		{Kind: expressions.TokenNumber, Value: big.NewInt(10)},
		{Kind: expressions.TokenPlus},
		{Kind: expressions.TokenNumber, Value: big.NewInt(20)},
	}

	got := collectTokens(stream)
	if diff := cmp.Diff(want, got, cmpopts.IgnoreFields(expressions.Token{}, "Span"), equateBigInt); diff != "" {
		t.Errorf("Lex(%q) mismatch (-want +got):\n%s", input, diff)
	}
}

func TestLexer_MinusTokensNotDistinguished(t *testing.T) {
	// Lexer should emit TokenMinus for all minus symbols regardless of position or context.
	input := "-5 - - 4.5 --3"
	stream, err := expressions.Lex(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected lex error: %v", err)
	}

	want := []expressions.Token{
		{Kind: expressions.TokenMinus},
		{Kind: expressions.TokenNumber, Value: big.NewInt(5)},
		{Kind: expressions.TokenMinus},
		{Kind: expressions.TokenMinus},
		{Kind: expressions.TokenNumber, Value: float32(4.5)},
		{Kind: expressions.TokenMinus},
		{Kind: expressions.TokenMinus},
		{Kind: expressions.TokenNumber, Value: big.NewInt(3)},
	}

	got := collectTokens(stream)
	if diff := cmp.Diff(want, got, cmpopts.IgnoreFields(expressions.Token{}, "Span"), equateBigInt); diff != "" {
		t.Errorf("Lex(%q) mismatch (-want +got):\n%s", input, diff)
	}
}

func TestTokenStream_Navigation(t *testing.T) {
	input := "10 + 20"
	stream, err := expressions.Lex(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected lex error: %v", err)
	}

	if stream.IsEOF() {
		t.Fatalf("expected stream not to be at EOF initially")
	}

	ignoreSpan := cmpopts.IgnoreFields(expressions.Token{}, "Span")

	if diff := cmp.Diff(expressions.Token{Kind: expressions.TokenPlus}, stream.Lookahead(1), ignoreSpan, equateBigInt); diff != "" {
		t.Errorf("unexpected lookahead(1) token (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(expressions.Token{Kind: expressions.TokenNumber, Value: big.NewInt(20)}, stream.Lookahead(2), ignoreSpan, equateBigInt); diff != "" {
		t.Errorf("unexpected lookahead(2) token (-want +got):\n%s", diff)
	}

	tok1 := stream.Peek()
	if diff := cmp.Diff(expressions.Token{Kind: expressions.TokenNumber, Value: big.NewInt(10)}, tok1, ignoreSpan, equateBigInt); diff != "" {
		t.Errorf("unexpected first peek token (-want +got):\n%s", diff)
	}

	next1 := stream.Next()
	if diff := cmp.Diff(expressions.Token{Kind: expressions.TokenNumber, Value: big.NewInt(10)}, next1, ignoreSpan, equateBigInt); diff != "" {
		t.Errorf("unexpected first next token (-want +got):\n%s", diff)
	}

	tok2 := stream.Next()
	if diff := cmp.Diff(expressions.Token{Kind: expressions.TokenPlus}, tok2, ignoreSpan, equateBigInt); diff != "" {
		t.Errorf("unexpected second next token (-want +got):\n%s", diff)
	}

	tok3 := stream.Next()
	if diff := cmp.Diff(expressions.Token{Kind: expressions.TokenNumber, Value: big.NewInt(20)}, tok3, ignoreSpan, equateBigInt); diff != "" {
		t.Errorf("unexpected third next token (-want +got):\n%s", diff)
	}

	eofTok := stream.Peek()
	if diff := cmp.Diff(expressions.Token{Kind: expressions.TokenEOF}, eofTok, ignoreSpan, equateBigInt); diff != "" {
		t.Errorf("expected TokenEOF on peek after stream consumed (-want +got):\n%s", diff)
	}
	if !stream.IsEOF() {
		t.Errorf("expected IsEOF() to be true")
	}
}

func TestLexer_TrailingWhitespaceInEOFSpan(t *testing.T) {
	testCases := []struct {
		name    string
		input   string
		wantPos expressions.Position
	}{
		{
			name:    "single_line_trailing_spaces",
			input:   "42   ",
			wantPos: expressions.Position{Line: 1, Column: 6, Offset: 5},
		},
		{
			name:    "multi_line_trailing_whitespace",
			input:   "42   \n  ",
			wantPos: expressions.Position{Line: 2, Column: 3, Offset: 8},
		},
		{
			name:    "empty_input",
			input:   "",
			wantPos: expressions.Position{Line: 1, Column: 1, Offset: 0},
		},
		{
			name:    "whitespace_only_input",
			input:   "   \n\t ",
			wantPos: expressions.Position{Line: 2, Column: 3, Offset: 6},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stream, err := expressions.Lex(strings.NewReader(tc.input))
			if err != nil {
				t.Fatalf("unexpected lex error: %v", err)
			}

			// Consume all non-EOF tokens
			for !stream.IsEOF() {
				stream.Next()
			}

			eof := stream.Peek()
			if eof.Kind != expressions.TokenEOF {
				t.Fatalf("expected TokenEOF, got %v", eof.Kind)
			}
			if eof.Span.Start != tc.wantPos || eof.Span.End != tc.wantPos {
				t.Errorf("EOF span mismatch: want %+v, got %+v", tc.wantPos, eof.Span)
			}
		})
	}
}

func TestLexer_Operators(t *testing.T) {
	input := "+ - * / ** % == != > >= < <= && || ^ ! ( ) [ ] ,"
	stream, err := expressions.Lex(strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected lex error: %v", err)
	}

	want := []expressions.Token{
		{Kind: expressions.TokenPlus},
		{Kind: expressions.TokenMinus},
		{Kind: expressions.TokenStar},
		{Kind: expressions.TokenSlash},
		{Kind: expressions.TokenStarStar},
		{Kind: expressions.TokenPercent},
		{Kind: expressions.TokenEqualEqual},
		{Kind: expressions.TokenNotEqual},
		{Kind: expressions.TokenGreaterThan},
		{Kind: expressions.TokenGreaterThanOrEqual},
		{Kind: expressions.TokenLessThan},
		{Kind: expressions.TokenLessThanOrEqual},
		{Kind: expressions.TokenAmpAmp},
		{Kind: expressions.TokenBarBar},
		{Kind: expressions.TokenCaret},
		{Kind: expressions.TokenExclamation},
		{Kind: expressions.TokenLeftParen},
		{Kind: expressions.TokenRightParen},
		{Kind: expressions.TokenLeftBracket},
		{Kind: expressions.TokenRightBracket},
		{Kind: expressions.TokenComma},
	}

	got := collectTokens(stream)
	if diff := cmp.Diff(want, got, cmpopts.IgnoreFields(expressions.Token{}, "Span")); diff != "" {
		t.Errorf("Lex(%q) mismatch (-want +got):\n%s", input, diff)
	}
}

func TestLexer_InvalidTokens(t *testing.T) {
	testCases := []struct {
		input string
	}{
		{"@"},
		{"&"},
		{"|"},
		{"="},
		{"$foo"},
		{"."},
		{"a[5]. abc"},
		{"a[5]."},
		{". "},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			_, err := expressions.Lex(strings.NewReader(tc.input))
			if err == nil {
				t.Fatalf("expected lex error for %q, got nil", tc.input)
			}
		})
	}
}

func TestLexer_FieldAccessAndNumberDisambiguation(t *testing.T) {
	testCases := []struct {
		name  string
		input string
		want  []expressions.Token
	}{
		{
			name:  "bracket field access with letter identifier",
			input: "foo[3].bar",
			want: []expressions.Token{
				{Kind: expressions.TokenIdentifier, Value: "foo"},
				{Kind: expressions.TokenLeftBracket},
				{Kind: expressions.TokenNumber, Value: big.NewInt(3)},
				{Kind: expressions.TokenRightBracket},
				{Kind: expressions.TokenIdentifier, Value: ".bar"},
			},
		},
		{
			name:  "bracket field access with digit starting identifier",
			input: "foo[4].123abc",
			want: []expressions.Token{
				{Kind: expressions.TokenIdentifier, Value: "foo"},
				{Kind: expressions.TokenLeftBracket},
				{Kind: expressions.TokenNumber, Value: big.NewInt(4)},
				{Kind: expressions.TokenRightBracket},
				{Kind: expressions.TokenIdentifier, Value: ".123abc"},
			},
		},
		{
			name:  "leading dot float",
			input: ".5 + .123",
			want: []expressions.Token{
				{Kind: expressions.TokenNumber, Value: float32(0.5)},
				{Kind: expressions.TokenPlus},
				{Kind: expressions.TokenNumber, Value: float64(0.123)},
			},
		},
		{
			name:  "leading dot field access with exponent like ident",
			input: ".5e2",
			want: []expressions.Token{
				{Kind: expressions.TokenIdentifier, Value: ".5e2"},
			},
		},
		{
			name:  "identifier starting with digits",
			input: "1e5 + 1.5e3",
			want: []expressions.Token{
				{Kind: expressions.TokenIdentifier, Value: "1e5"},
				{Kind: expressions.TokenPlus},
				{Kind: expressions.TokenIdentifier, Value: "1.5e3"},
			},
		},
		{
			name:  "negative identifier starting with digits",
			input: "-1e5",
			want: []expressions.Token{
				{Kind: expressions.TokenMinus},
				{Kind: expressions.TokenIdentifier, Value: "1e5"},
			},
		},
		{
			name:  "negative hex float integer as identifier",
			input: "-0x1p5",
			want: []expressions.Token{
				{Kind: expressions.TokenMinus},
				{Kind: expressions.TokenIdentifier, Value: "0x1p5"},
			},
		},
		{
			name:  "negative hex float negative exponent as subtraction",
			input: "-0x1p-5",
			want: []expressions.Token{
				{Kind: expressions.TokenMinus},
				{Kind: expressions.TokenIdentifier, Value: "0x1p"},
				{Kind: expressions.TokenMinus},
				{Kind: expressions.TokenNumber, Value: big.NewInt(5)},
			},
		},
		{
			name:  "bracket field access with infinity",
			input: "a[1].infinity",
			want: []expressions.Token{
				{Kind: expressions.TokenIdentifier, Value: "a"},
				{Kind: expressions.TokenLeftBracket},
				{Kind: expressions.TokenNumber, Value: big.NewInt(1)},
				{Kind: expressions.TokenRightBracket},
				{Kind: expressions.TokenIdentifier, Value: ".infinity"},
			},
		},
		{
			name:  "bracket field access with inf",
			input: "a[1].inf",
			want: []expressions.Token{
				{Kind: expressions.TokenIdentifier, Value: "a"},
				{Kind: expressions.TokenLeftBracket},
				{Kind: expressions.TokenNumber, Value: big.NewInt(1)},
				{Kind: expressions.TokenRightBracket},
				{Kind: expressions.TokenIdentifier, Value: ".inf"},
			},
		},
		{
			name:  "bracket field access with nan",
			input: "a[1].nan",
			want: []expressions.Token{
				{Kind: expressions.TokenIdentifier, Value: "a"},
				{Kind: expressions.TokenLeftBracket},
				{Kind: expressions.TokenNumber, Value: big.NewInt(1)},
				{Kind: expressions.TokenRightBracket},
				{Kind: expressions.TokenIdentifier, Value: ".nan"},
			},
		},
		{
			name:  "bracket field access with true",
			input: "a[1].true",
			want: []expressions.Token{
				{Kind: expressions.TokenIdentifier, Value: "a"},
				{Kind: expressions.TokenLeftBracket},
				{Kind: expressions.TokenNumber, Value: big.NewInt(1)},
				{Kind: expressions.TokenRightBracket},
				{Kind: expressions.TokenIdentifier, Value: ".true"},
			},
		},
		{
			name:  "bracket field access with false",
			input: "a[1].false",
			want: []expressions.Token{
				{Kind: expressions.TokenIdentifier, Value: "a"},
				{Kind: expressions.TokenLeftBracket},
				{Kind: expressions.TokenNumber, Value: big.NewInt(1)},
				{Kind: expressions.TokenRightBracket},
				{Kind: expressions.TokenIdentifier, Value: ".false"},
			},
		},
		{
			name:  "leading dot infinity",
			input: ".infinity",
			want: []expressions.Token{
				{Kind: expressions.TokenIdentifier, Value: ".infinity"},
			},
		},
		{
			name:  "leading dot nan",
			input: ".nan",
			want: []expressions.Token{
				{Kind: expressions.TokenIdentifier, Value: ".nan"},
			},
		},
		{
			name:  "leading dot true",
			input: ".true",
			want: []expressions.Token{
				{Kind: expressions.TokenIdentifier, Value: ".true"},
			},
		},
		{
			name:  "leading dot false",
			input: ".false",
			want: []expressions.Token{
				{Kind: expressions.TokenIdentifier, Value: ".false"},
			},
		},
		{
			name:  "unbracketed field path with infinity",
			input: "foo.infinity",
			want: []expressions.Token{
				{Kind: expressions.TokenIdentifier, Value: "foo.infinity"},
			},
		},
		{
			name:  "unbracketed field path with true and nan",
			input: "foo.true.nan",
			want: []expressions.Token{
				{Kind: expressions.TokenIdentifier, Value: "foo.true.nan"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stream, err := expressions.Lex(strings.NewReader(tc.input))
			if err != nil {
				t.Fatalf("unexpected lex error: %v", err)
			}
			got := collectTokens(stream)
			if diff := cmp.Diff(tc.want, got, cmpopts.IgnoreFields(expressions.Token{}, "Span"), equateBigInt); diff != "" {
				t.Errorf("Lex(%q) mismatch (-want +got):\n%s", tc.input, diff)
			}
		})
	}
}

func TestLexer_InvalidOperators(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErrText string
	}{
		{
			name:        "single ampersand at EOF",
			input:       "&",
			wantErrText: `unknown operator "&"`,
		},
		{
			name:        "single pipe at EOF",
			input:       "|",
			wantErrText: `unknown operator "|"`,
		},
		{
			name:        "single equals at EOF",
			input:       "=",
			wantErrText: `unknown operator "="`,
		},
		{
			name:        "unknown character at EOF",
			input:       "@",
			wantErrText: `unknown operator "@"`,
		},
		{
			name:        "single ampersand followed by equals",
			input:       "&=",
			wantErrText: `unknown operator "&="`,
		},
		{
			name:        "single ampersand followed by identifier",
			input:       "&x",
			wantErrText: `unknown operator "&x"`,
		},
		{
			name:        "single ampersand followed by whitespace",
			input:       "& ",
			wantErrText: `unknown operator "& "`,
		},
		{
			name:        "single pipe followed by equals",
			input:       "|=",
			wantErrText: `unknown operator "|="`,
		},
		{
			name:        "single pipe followed by identifier",
			input:       "|x",
			wantErrText: `unknown operator "|x"`,
		},
		{
			name:        "single pipe followed by whitespace",
			input:       "| ",
			wantErrText: `unknown operator "| "`,
		},
		{
			name:        "single equals followed by greater than",
			input:       "=>",
			wantErrText: `unknown operator "=>"`,
		},
		{
			name:        "single equals followed by identifier",
			input:       "=x",
			wantErrText: `unknown operator "=x"`,
		},
		{
			name:        "single equals followed by whitespace",
			input:       "= ",
			wantErrText: `unknown operator "= "`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := expressions.Lex(strings.NewReader(tc.input))
			if err == nil {
				t.Fatalf("expected error for %q, got nil", tc.input)
			}
			if !strings.Contains(err.Error(), tc.wantErrText) {
				t.Errorf("error %q does not contain expected %q", err.Error(), tc.wantErrText)
			}
			if strings.Contains(err.Error(), "\x00") {
				t.Errorf("error %q unexpectedly contains a null byte (0 byte)", err.Error())
			}
		})
	}
}
