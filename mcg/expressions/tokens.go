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

package expressions

import (
	"fmt"
	"math/big"
)

// Position represents a 1-indexed line and column position, and 0-indexed byte offset.
type Position struct {
	Line   int // 1-indexed line number
	Column int // 1-indexed column number
	Offset int // 0-indexed byte offset in the complete string
}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

// Span represents a source code range.
type Span struct {
	Start Position
	End   Position
}

// TokenKind represents the lexical category of a token.
type TokenKind int

// TokenKind constants define the lexical token categories recognized by the expression lexer.
const (
	TokenEOF TokenKind = iota

	// Literals & Identifiers
	TokenBool       // true, false
	TokenNumber     // integers, floats, and numeric limits
	TokenIdentifier // includes field paths like pub.field

	// Operators & Punctuation
	TokenPlus               // +
	TokenMinus              // -
	TokenStar               // *
	TokenSlash              // /
	TokenStarStar           // **
	TokenPercent            // %
	TokenEqualEqual         // ==
	TokenNotEqual           // !=
	TokenGreaterThan        // >
	TokenGreaterThanOrEqual // >=
	TokenLessThan           // <
	TokenLessThanOrEqual    // <=
	TokenAmpAmp             // &&
	TokenBarBar             // ||
	TokenCaret              // ^
	TokenExclamation        // !
	TokenLeftParen          // (
	TokenRightParen         // )
	TokenLeftBracket        // [
	TokenRightBracket       // ]
	TokenComma              // ,
)

// Token represents a single lexical token with source span information.
type Token struct {
	Kind  TokenKind
	Value any
	Span  Span
}

// String implements fmt.Stringer for TokenKind.
func (k TokenKind) String() string {
	switch k {
	case TokenEOF:
		return "EOF"
	case TokenBool:
		return "bool"
	case TokenNumber:
		return "number"
	case TokenIdentifier:
		return "identifier"
	case TokenPlus:
		return "+"
	case TokenMinus:
		return "-"
	case TokenStar:
		return "*"
	case TokenSlash:
		return "/"
	case TokenStarStar:
		return "**"
	case TokenPercent:
		return "%"
	case TokenEqualEqual:
		return "=="
	case TokenNotEqual:
		return "!="
	case TokenGreaterThan:
		return ">"
	case TokenGreaterThanOrEqual:
		return ">="
	case TokenLessThan:
		return "<"
	case TokenLessThanOrEqual:
		return "<="
	case TokenAmpAmp:
		return "&&"
	case TokenBarBar:
		return "||"
	case TokenCaret:
		return "^"
	case TokenExclamation:
		return "!"
	case TokenLeftParen:
		return "("
	case TokenRightParen:
		return ")"
	case TokenLeftBracket:
		return "["
	case TokenRightBracket:
		return "]"
	case TokenComma:
		return ","
	default:
		return fmt.Sprintf("unknown[%d]", k)
	}
}

// String implements fmt.Stringer for Token.
func (t Token) String() string {
	switch t.Kind {
	case TokenBool:
		if t.Value != nil {
			return fmt.Sprintf("%v", t.Value)
		}
	case TokenIdentifier:
		if t.Value != nil {
			return fmt.Sprintf("identifier(%q)", t.Value)
		}
	case TokenNumber:
		if t.Value != nil {
			if bi, ok := t.Value.(*big.Int); ok {
				return fmt.Sprintf("bigInt(%v)", bi)
			}
			return fmt.Sprintf("%T(%v)", t.Value, t.Value)
		}
	default:
		if t.Value != nil {
			return fmt.Sprintf("%s(%v)", t.Kind, t.Value)
		}
	}
	return t.Kind.String()
}

var (
	_ fmt.Stringer = Position{}
	_ fmt.Stringer = TokenKind(0)
	_ fmt.Stringer = Token{}
)
