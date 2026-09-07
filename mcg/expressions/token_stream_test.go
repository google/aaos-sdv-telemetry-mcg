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
	"testing"

	"sdv.googlesource.com/mcg/mcg/expressions"
)

func TestTokenStream_Empty(t *testing.T) {
	for _, stream := range []*expressions.TokenStream{
		expressions.NewTokenStream(nil),
		expressions.NewTokenStream([]expressions.Token{}),
	} {
		if !stream.IsEOF() {
			t.Errorf("expected IsEOF() to be true for empty stream")
		}

		tok := stream.Peek()
		if tok.Kind != expressions.TokenEOF {
			t.Errorf("expected TokenEOF on Peek(), got %v", tok.Kind)
		}
		expectedPos := expressions.Position{Line: 1, Column: 1, Offset: 0}
		if tok.Span.Start != expectedPos || tok.Span.End != expectedPos {
			t.Errorf("expected EOF span %+v, got %+v", expectedPos, tok.Span)
		}

		if la := stream.Lookahead(0); la.Kind != expressions.TokenEOF {
			t.Errorf("expected TokenEOF on Lookahead(0), got %v", la.Kind)
		}
		if la := stream.Lookahead(-1); la.Kind != expressions.TokenEOF {
			t.Errorf("expected TokenEOF on Lookahead(-1), got %v", la.Kind)
		}
		if la := stream.Lookahead(5); la.Kind != expressions.TokenEOF {
			t.Errorf("expected TokenEOF on Lookahead(5), got %v", la.Kind)
		}
		if la := stream.Lookahead(-5); la.Kind != expressions.TokenEOF {
			t.Errorf("expected TokenEOF on Lookahead(-5), got %v", la.Kind)
		}

		next := stream.Next()
		if next.Kind != expressions.TokenEOF {
			t.Errorf("expected TokenEOF on Next(), got %v", next.Kind)
		}
		if !stream.IsEOF() {
			t.Errorf("expected IsEOF() to remain true after Next()")
		}
	}
}

func TestTokenStream_SequentialNavigation(t *testing.T) {
	tok1 := expressions.Token{
		Kind:  expressions.TokenNumber,
		Value: int32(42),
		Span: expressions.Span{
			Start: expressions.Position{Line: 1, Column: 1, Offset: 0},
			End:   expressions.Position{Line: 1, Column: 3, Offset: 2},
		},
	}
	tok2 := expressions.Token{
		Kind:  expressions.TokenPlus,
		Value: nil,
		Span: expressions.Span{
			Start: expressions.Position{Line: 1, Column: 4, Offset: 3},
			End:   expressions.Position{Line: 1, Column: 5, Offset: 4},
		},
	}
	tok3 := expressions.Token{
		Kind:  expressions.TokenBool,
		Value: true,
		Span: expressions.Span{
			Start: expressions.Position{Line: 1, Column: 6, Offset: 5},
			End:   expressions.Position{Line: 1, Column: 10, Offset: 9},
		},
	}

	stream := expressions.NewTokenStream([]expressions.Token{tok1, tok2, tok3})

	if stream.IsEOF() {
		t.Fatalf("expected stream not to be at EOF initially")
	}

	// 1. Check Peek & Lookahead without advancing
	if p := stream.Peek(); p != tok1 {
		t.Errorf("expected peek to be tok1 (%+v), got %+v", tok1, p)
	}
	if la0 := stream.Lookahead(0); la0 != tok1 {
		t.Errorf("expected lookahead(0) to be tok1 (%+v), got %+v", tok1, la0)
	}
	if la1 := stream.Lookahead(1); la1 != tok2 {
		t.Errorf("expected lookahead(1) to be tok2 (%+v), got %+v", tok2, la1)
	}
	if la2 := stream.Lookahead(2); la2 != tok3 {
		t.Errorf("expected lookahead(2) to be tok3 (%+v), got %+v", tok3, la2)
	}

	// Lookahead beyond stream length should return TokenEOF with last token's end span
	la3 := stream.Lookahead(3)
	if la3.Kind != expressions.TokenEOF {
		t.Errorf("expected lookahead(3) to be TokenEOF, got %v", la3.Kind)
	}
	if la3.Span.Start != tok3.Span.End || la3.Span.End != tok3.Span.End {
		t.Errorf("expected EOF span at end of tok3 (%+v), got %+v", tok3.Span.End, la3.Span)
	}

	la10 := stream.Lookahead(10)
	if la10.Kind != expressions.TokenEOF {
		t.Errorf("expected lookahead(10) to be TokenEOF, got %v", la10.Kind)
	}

	// 2. Consume tokens one by one with Next()
	if n1 := stream.Next(); n1 != tok1 {
		t.Errorf("expected first Next() to be tok1 (%+v), got %+v", tok1, n1)
	}
	if laNeg1 := stream.Lookahead(-1); laNeg1 != tok1 {
		t.Errorf("expected Lookahead(-1) after Next() to be tok1 (%+v), got %+v", tok1, laNeg1)
	}
	if laNeg5 := stream.Lookahead(-5); laNeg5.Kind != expressions.TokenEOF {
		t.Errorf("expected Lookahead(-5) after Next() to be TokenEOF, got %v", laNeg5.Kind)
	}
	if stream.IsEOF() {
		t.Errorf("expected stream not to be EOF after first Next()")
	}

	if n2 := stream.Next(); n2 != tok2 {
		t.Errorf("expected second Next() to be tok2 (%+v), got %+v", tok2, n2)
	}
	if stream.IsEOF() {
		t.Errorf("expected stream not to be EOF after second Next()")
	}

	if n3 := stream.Next(); n3 != tok3 {
		t.Errorf("expected third Next() to be tok3 (%+v), got %+v", tok3, n3)
	}
	if !stream.IsEOF() {
		t.Errorf("expected stream to be EOF after third Next()")
	}

	// 3. Post-EOF behavior
	eofTok := stream.Peek()
	if eofTok.Kind != expressions.TokenEOF {
		t.Errorf("expected TokenEOF on peek at EOF, got %v", eofTok.Kind)
	}
	if eofTok.Span.Start != tok3.Span.End {
		t.Errorf("expected EOF span start %+v, got %+v", tok3.Span.End, eofTok.Span.Start)
	}

	// Multiple Next() calls at EOF should be idempotent
	for i := 0; i < 3; i++ {
		nextEOF := stream.Next()
		if nextEOF.Kind != expressions.TokenEOF {
			t.Errorf("expected TokenEOF on Next() %d at EOF, got %v", i, nextEOF.Kind)
		}
		if !stream.IsEOF() {
			t.Errorf("expected stream to remain at EOF")
		}
	}
}

func TestTokenStream_WithPreExistingEOF(t *testing.T) {
	tok := expressions.Token{
		Kind: expressions.TokenNumber,
		Span: expressions.Span{
			Start: expressions.Position{Line: 1, Column: 1, Offset: 0},
			End:   expressions.Position{Line: 1, Column: 3, Offset: 2},
		},
	}
	eofPos := expressions.Position{Line: 1, Column: 10, Offset: 9}
	eof := expressions.Token{
		Kind: expressions.TokenEOF,
		Span: expressions.Span{Start: eofPos, End: eofPos},
	}

	stream := expressions.NewTokenStream([]expressions.Token{tok, eof})

	// Verify Lookahead(-1) at pos == 0 safely returns TokenEOF
	if la := stream.Lookahead(-1); la.Kind != expressions.TokenEOF {
		t.Errorf("expected TokenEOF on Lookahead(-1) at start, got %v", la.Kind)
	}

	// Consume tok
	if n := stream.Next(); n != tok {
		t.Errorf("expected tok, got %+v", n)
	}

	// Now at EOF
	if !stream.IsEOF() {
		t.Errorf("expected IsEOF() to be true")
	}
	if p := stream.Peek(); p != eof {
		t.Errorf("expected original EOF token to be preserved, got %+v", p)
	}

	// Peek backwards from EOF
	if laBack := stream.Lookahead(-1); laBack != tok {
		t.Errorf("expected Lookahead(-1) from EOF to return tok, got %+v", laBack)
	}
	if laBackOOB := stream.Lookahead(-2); laBackOOB.Kind != expressions.TokenEOF {
		t.Errorf("expected Lookahead(-2) from EOF to return TokenEOF, got %v", laBackOOB.Kind)
	}

	// Verify duplicate EOF wasn't appended (calling Next() stays at eof)
	nextEOF := stream.Next()
	if nextEOF != eof {
		t.Errorf("expected eof, got %+v", nextEOF)
	}
	if !stream.IsEOF() {
		t.Errorf("expected stream to remain at EOF")
	}
}
