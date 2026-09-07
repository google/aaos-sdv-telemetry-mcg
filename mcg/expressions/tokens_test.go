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
	"testing"

	"sdv.googlesource.com/mcg/mcg/expressions"
)

func TestToken_String(t *testing.T) {
	tests := []struct {
		name string
		tok  expressions.Token
		want string
	}{
		{
			name: "token with nil value (operator)",
			tok:  expressions.Token{Kind: expressions.TokenPlus},
			want: "+",
		},
		{
			name: "token with nil value (punctuation)",
			tok:  expressions.Token{Kind: expressions.TokenRightParen},
			want: ")",
		},
		{
			name: "token with nil value (EOF)",
			tok:  expressions.Token{Kind: expressions.TokenEOF},
			want: "EOF",
		},
		{
			name: "token with big.Int number value",
			tok:  expressions.Token{Kind: expressions.TokenNumber, Value: big.NewInt(42)},
			want: "bigInt(42)",
		},
		{
			name: "token with float64 number value",
			tok:  expressions.Token{Kind: expressions.TokenNumber, Value: float64(3.14)},
			want: "float64(3.14)",
		},
		{
			name: "token with identifier value",
			tok:  expressions.Token{Kind: expressions.TokenIdentifier, Value: "foo.bar"},
			want: `identifier("foo.bar")`,
		},
		{
			name: "token with bool value true",
			tok:  expressions.Token{Kind: expressions.TokenBool, Value: true},
			want: "true",
		},
		{
			name: "token with bool value false",
			tok:  expressions.Token{Kind: expressions.TokenBool, Value: false},
			want: "false",
		},
		{
			name: "token with unknown kind",
			tok:  expressions.Token{Kind: expressions.TokenKind(999)},
			want: "unknown[999]",
		},
		{
			name: "token with unknown kind and value",
			tok:  expressions.Token{Kind: expressions.TokenKind(999), Value: "xyz"},
			want: "unknown[999](xyz)",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.tok.String()
			if got != tc.want {
				t.Errorf("tok.String() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTokenKind_String(t *testing.T) {
	tests := []struct {
		name string
		kind expressions.TokenKind
		want string
	}{
		{name: "EOF", kind: expressions.TokenEOF, want: "EOF"},
		{name: "number", kind: expressions.TokenNumber, want: "number"},
		{name: "identifier", kind: expressions.TokenIdentifier, want: "identifier"},
		{name: "bool", kind: expressions.TokenBool, want: "bool"},
		{name: "plus", kind: expressions.TokenPlus, want: "+"},
		{name: "starstar", kind: expressions.TokenStarStar, want: "**"},
		{name: "comma", kind: expressions.TokenComma, want: ","},
		{name: "unknown", kind: expressions.TokenKind(999), want: "unknown[999]"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.kind.String()
			if got != tc.want {
				t.Errorf("kind.String() = %q, want %q", got, tc.want)
			}
		})
	}
}
