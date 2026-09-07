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
	"bufio"
	"fmt"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"
	"unicode"
)

// Lex converts an MCG expression from an io.Reader into a TokenStream.
func Lex(r io.Reader) (*TokenStream, error) {
	l := &lexer{
		reader: bufio.NewReader(r),
		pos:    Position{Line: 1, Column: 1, Offset: 0},
	}
	return l.lex()
}

type lexer struct {
	reader *bufio.Reader
	pos    Position
}

func (l *lexer) peek() (rune, error) {
	r, _, err := l.reader.ReadRune()
	if err != nil {
		return 0, err
	}
	_ = l.reader.UnreadRune()

	return r, nil
}

func (l *lexer) advance() (rune, error) {
	r, size, err := l.reader.ReadRune()
	if err != nil {
		return 0, err
	}
	l.pos.Offset += size
	if r == '\n' {
		l.pos.Line++
		l.pos.Column = 1
	} else {
		l.pos.Column++
	}
	return r, nil
}

// advanceIf peeks ahead one rune. If the provided matcher function returns true for that rune,
// it advances the lexer and returns true along with the peeked rune; otherwise, it returns false
// along with the peeked rune. Returns an error if peeking ahead fails (including at EOF).
func (l *lexer) advanceIf(matcher func(rune) bool) (bool, rune, error) {
	nr, err := l.peek()
	if err != nil {
		return false, 0, err
	}
	if !matcher(nr) {
		return false, nr, nil
	}

	_, _ = l.advance()
	return true, nr, nil
}

func (l *lexer) skipWhitespace() error {
	for {
		ok, _, err := l.advanceIf(unicode.IsSpace)
		if err != nil && err != io.EOF {
			return err
		}
		if !ok {
			return nil
		}
	}
}

// emit creates a basic Token with the given TokenKind and source Span.
func (l *lexer) emit(kind TokenKind, pos Position) Token {
	return l.emitVal(kind, nil, pos)
}

// emitVal creates a Token containing a Value payload and source Span.
func (l *lexer) emitVal(kind TokenKind, val any, pos Position) Token {
	return Token{
		Kind:  kind,
		Value: val,
		Span:  Span{Start: pos, End: l.pos},
	}
}

func isWordPart(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.'
}

func (l *lexer) lex() (*TokenStream, error) {
	var tokens []Token

	for {
		if err := l.skipWhitespace(); err != nil {
			return nil, err
		}

		r, err := l.peek()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		var tok Token
		if isWordPart(r) {
			// 1. Words: Numbers, Identifiers, Booleans, Keywords, and Dot-Prefixed Fields
			if tok, err = l.scanWord(); err != nil {
				return nil, err
			}
		} else {
			// 2. Operators & Punctuation
			if tok, err = l.scanOperator(); err != nil {
				return nil, err
			}
		}
		tokens = append(tokens, tok)
	}

	tokens = append(tokens, l.emit(TokenEOF, l.pos))
	return NewTokenStream(tokens), nil
}

// scanWord scans a contiguous sequence of word characters (identifiers, numbers, booleans, keywords, and dot-prefixed fields).
func (l *lexer) scanWord() (Token, error) {
	pos := l.pos
	var sb strings.Builder
	for {
		ok, r, err := l.advanceIf(isWordPart)
		if err != nil && err != io.EOF {
			return Token{}, err
		}
		if !ok {
			break
		}
		sb.WriteRune(r)
	}
	word := sb.String()

	if word == "." {
		return Token{}, fmt.Errorf("unexpected '.' at %v: expected field name or number", pos)
	}
	if word == "true" {
		return l.emitVal(TokenBool, true, pos), nil
	}
	if word == "false" {
		return l.emitVal(TokenBool, false, pos), nil
	}
	if n, err := parseNumber(word); err == nil {
		return l.emitVal(TokenNumber, n, pos), nil
	}
	return l.emitVal(TokenIdentifier, word, pos), nil
}

// scanOperator scans single- and two-character operators and punctuation.
func (l *lexer) scanOperator() (Token, error) {
	pos := l.pos
	r, err := l.advance()
	if err != nil {
		return Token{}, err
	}
	switch r {
	case '+':
		return l.emit(TokenPlus, pos), nil
	case '-':
		return l.emit(TokenMinus, pos), nil
	case '/':
		return l.emit(TokenSlash, pos), nil
	case '%':
		return l.emit(TokenPercent, pos), nil
	case '^':
		return l.emit(TokenCaret, pos), nil
	case '(':
		return l.emit(TokenLeftParen, pos), nil
	case ')':
		return l.emit(TokenRightParen, pos), nil
	case '[':
		return l.emit(TokenLeftBracket, pos), nil
	case ']':
		return l.emit(TokenRightBracket, pos), nil
	case ',':
		return l.emit(TokenComma, pos), nil
	case '*':
		ok, _, err := l.advanceIf(func(r rune) bool { return r == '*' })
		if err != nil && err != io.EOF {
			return Token{}, nil
		}
		if ok {
			return l.emit(TokenStarStar, pos), nil
		}
		return l.emit(TokenStar, pos), nil
	case '&':
		ok, nr, err := l.advanceIf(func(r rune) bool { return r == '&' })
		if err != nil && err != io.EOF {
			return Token{}, nil
		}
		if !ok {
			return Token{}, fmt.Errorf("unknown operator %q at %v", formatUnknownOperator(r, nr), pos)
		}
		return l.emit(TokenAmpAmp, pos), nil
	case '|':
		ok, nr, err := l.advanceIf(func(r rune) bool { return r == '|' })
		if err != nil && err != io.EOF {
			return Token{}, nil
		}
		if !ok {
			return Token{}, fmt.Errorf("unknown operator %q at %v", formatUnknownOperator(r, nr), pos)
		}
		return l.emit(TokenBarBar, pos), nil
	case '!':
		ok, _, err := l.advanceIf(func(r rune) bool { return r == '=' })
		if err != nil && err != io.EOF {
			return Token{}, nil
		}
		if ok {
			return l.emit(TokenNotEqual, pos), nil
		}
		return l.emit(TokenExclamation, pos), nil
	case '=':
		ok, nr, err := l.advanceIf(func(r rune) bool { return r == '=' })
		if err != nil && err != io.EOF {
			return Token{}, nil
		}
		if !ok {
			return Token{}, fmt.Errorf("unknown operator %q at %v", formatUnknownOperator(r, nr), pos)
		}
		return l.emit(TokenEqualEqual, pos), nil
	case '>':
		ok, _, err := l.advanceIf(func(r rune) bool { return r == '=' })
		if err != nil && err != io.EOF {
			return Token{}, nil
		}
		if ok {
			return l.emit(TokenGreaterThanOrEqual, pos), nil
		}
		return l.emit(TokenGreaterThan, pos), nil
	case '<':
		ok, _, err := l.advanceIf(func(r rune) bool { return r == '=' })
		if err != nil && err != io.EOF {
			return Token{}, nil
		}
		if ok {
			return l.emit(TokenLessThanOrEqual, pos), nil
		}
		return l.emit(TokenLessThan, pos), nil
	default:
		return Token{}, fmt.Errorf("unknown operator %q at %v", string(r), pos)
	}
}

func formatUnknownOperator(r, nr rune) string {
	// 0 is used as the placeholder for EOF - do not render it, because that
	// seems confusing.
	if nr == 0 {
		return string(r)
	}
	return string(r) + string(nr)
}

func parseNumber(s string) (any, error) {
	// First, try to parse the string as a big integer. We do not attempt to
	// parse it as int32/int64 here just yet, because we don't yet know if the
	// parser ends up negating the integer, which might lead to subtle
	// overflows.
	var bi big.Int
	if _, ok := bi.SetString(s, 10); ok {
		return &bi, nil
	}

	// Only attempt to parse as float if the number is not written in scientific
	// notation (e.g., "1e6"). We could add support for that later, but then
	// we'd also need to add support for more complex scientific notations, such
	// as "-2.25E+4" in the lexer/parser.
	//
	// Note that we do support parsing "infinity" and "nan" (and arbitrarily
	// cased variants, such as "InFiNiTy" for backwards compatibility) here.
	if !strings.ContainsAny(s, "eEpP") {
		if f64, err := strconv.ParseFloat(s, 64); err == nil {
			// Check whether converting to f32 doesn't lose any precision (the
			// condition for NaNs is needed because two NaN values always
			// compare as unequal).
			if f32 := float32(f64); float64(f32) == f64 || (math.IsNaN(f64) && math.IsNaN(float64(f32))) {
				return f32, nil
			}
			return f64, nil
		}
	}
	return nil, fmt.Errorf("invalid number format %q", s)
}
