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

// TokenStream provides lookahead and navigation over a sequence of tokens.
type TokenStream struct {
	tokens []Token
	pos    int
}

// NewTokenStream returns a new TokenStream initialized with the given tokens.
func NewTokenStream(tokens []Token) *TokenStream {
	if len(tokens) == 0 {
		pos := Position{Line: 1, Column: 1, Offset: 0}
		return &TokenStream{
			tokens: []Token{{Kind: TokenEOF, Span: Span{Start: pos, End: pos}}},
			pos:    0,
		}
	}

	if tokens[len(tokens)-1].Kind != TokenEOF {
		lastPos := tokens[len(tokens)-1].Span.End
		tokens = append(tokens, Token{Kind: TokenEOF, Span: Span{Start: lastPos, End: lastPos}})
	}

	return &TokenStream{tokens: tokens, pos: 0}
}

// Peek returns the current token without advancing the stream.
func (s *TokenStream) Peek() Token {
	return s.Lookahead(0)
}

// Lookahead returns the token n positions ahead without advancing the stream.
// If the lookahead offset is out of bounds, Lookahead returns TokenEOF.
func (s *TokenStream) Lookahead(n int) Token {
	idx := s.pos + n
	if idx < 0 || idx >= len(s.tokens) {
		// `NewTokenStream` ensures that the last token is always EOF.
		return s.tokens[len(s.tokens)-1]
	}
	return s.tokens[idx]
}

// Next returns the current token and advances the stream.
func (s *TokenStream) Next() Token {
	tok := s.Peek()
	if s.pos < len(s.tokens) {
		s.pos++
	}
	return tok
}

// IsEOF reports whether the token stream has reached the end.
func (s *TokenStream) IsEOF() bool {
	return s.Peek().Kind == TokenEOF
}
