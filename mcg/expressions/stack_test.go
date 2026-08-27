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
	"testing"
)

func TestStack_Int(t *testing.T) {
	var s stack[int]
	if !s.isEmpty() {
		t.Errorf("new stack should be empty")
	}

	s.push(10)
	if s.isEmpty() {
		t.Errorf("stack after push should not be empty")
	}
	if got := s.peek(); got != 10 {
		t.Errorf("peek() = %v, want %v", got, 10)
	}

	s.push(20)
	if got := s.peek(); got != 20 {
		t.Errorf("peek() = %v, want %v", got, 20)
	}

	if got := s.pop(); got != 20 {
		t.Errorf("pop() = %v, want %v", got, 20)
	}
	if got := s.pop(); got != 10 {
		t.Errorf("pop() = %v, want %v", got, 10)
	}
	if !s.isEmpty() {
		t.Errorf("stack after popping all items should be empty")
	}
}

func TestStack_Operator(t *testing.T) {
	var s stack[Operator]
	if !s.isEmpty() {
		t.Errorf("new stack should be empty")
	}

	s.push(OperatorAdd)
	s.push(OperatorMultiply)

	if got := s.peek(); got != OperatorMultiply {
		t.Errorf("peek() = %v, want %v", got, OperatorMultiply)
	}
	if got := s.pop(); got != OperatorMultiply {
		t.Errorf("pop() = %v, want %v", got, OperatorMultiply)
	}
	if got := s.pop(); got != OperatorAdd {
		t.Errorf("pop() = %v, want %v", got, OperatorAdd)
	}
	if !s.isEmpty() {
		t.Errorf("stack after popping all items should be empty")
	}
}

func TestStack_Operand(t *testing.T) {
	var s stack[operand]
	if !s.isEmpty() {
		t.Errorf("new stack should be empty")
	}

	op1 := operand{index: 10, isComparison: false, parenthesized: false}
	op2 := operand{index: 20, isComparison: true, parenthesized: true}

	s.push(op1)
	if s.isEmpty() {
		t.Errorf("stack after push should not be empty")
	}
	if got := s.peek(); got != op1 {
		t.Errorf("peek() = %+v, want %+v", got, op1)
	}

	s.push(op2)
	if got := s.peek(); got != op2 {
		t.Errorf("peek() = %+v, want %+v", got, op2)
	}

	if got := s.pop(); got != op2 {
		t.Errorf("pop() = %+v, want %+v", got, op2)
	}
	if got := s.pop(); got != op1 {
		t.Errorf("pop() = %+v, want %+v", got, op1)
	}
	if !s.isEmpty() {
		t.Errorf("stack after popping all items should be empty")
	}
}

func TestStack_PointersAndZeroing(t *testing.T) {
	var s stack[*int]
	val1 := 10
	val2 := 20

	s.push(&val1)
	s.push(&val2)

	if got := s.pop(); got != &val2 {
		t.Errorf("pop() = %v, want %v", got, &val2)
	}

	// Verify that the underlying backing slice at index 1 is cleared to nil.
	underlying := ([]*int)(s)
	if underlying[:2][1] != nil {
		t.Errorf("expected underlying element at index 1 to be cleared to nil, got %v", underlying[:2][1])
	}
}

func TestStack_PeekDepth(t *testing.T) {
	t.Run("empty_stack", func(t *testing.T) {
		var s stack[int]
		for _, depth := range []int{-1, 0, 1, 10} {
			val, ok := s.peekDepth(depth)
			if ok {
				t.Errorf("peekDepth(%d) on empty stack should return ok=false, got val=%v, ok=true", depth, val)
			}
			if val != 0 {
				t.Errorf("peekDepth(%d) on empty stack should return zero value, got %v", depth, val)
			}
		}
	})

	t.Run("elements_accessible_by_depth", func(t *testing.T) {
		var s stack[string]
		s.push("first")
		s.push("second")
		s.push("third")

		testCases := []struct {
			depth   int
			wantVal string
			wantOk  bool
		}{
			{depth: 0, wantVal: "third", wantOk: true},
			{depth: 1, wantVal: "second", wantOk: true},
			{depth: 2, wantVal: "first", wantOk: true},
			{depth: 3, wantVal: "", wantOk: false},
			{depth: 4, wantVal: "", wantOk: false},
			{depth: -1, wantVal: "", wantOk: false},
			{depth: -10, wantVal: "", wantOk: false},
		}

		for _, tc := range testCases {
			gotVal, gotOk := s.peekDepth(tc.depth)
			if gotOk != tc.wantOk {
				t.Errorf("peekDepth(%d) ok = %v, want %v", tc.depth, gotOk, tc.wantOk)
			}
			if gotVal != tc.wantVal {
				t.Errorf("peekDepth(%d) val = %q, want %q", tc.depth, gotVal, tc.wantVal)
			}
		}
	})

	t.Run("peek_depth_after_pop", func(t *testing.T) {
		var s stack[int]
		s.push(100)
		s.push(200)

		val, ok := s.peekDepth(0)
		if !ok || val != 200 {
			t.Fatalf("peekDepth(0) before pop = (%v, %v), want (200, true)", val, ok)
		}

		s.pop()

		val, ok = s.peekDepth(0)
		if !ok || val != 100 {
			t.Errorf("peekDepth(0) after pop = (%v, %v), want (100, true)", val, ok)
		}

		val, ok = s.peekDepth(1)
		if ok {
			t.Errorf("peekDepth(1) after pop should be out of bounds, got (%v, %v)", val, ok)
		}
	})

	t.Run("operator_stack_parser_use_case", func(t *testing.T) {
		// Simulates how parse.go inspects the operator preceding OperatorLeftParen
		var opStack stack[Operator]
		opStack.push(OperatorContains)
		opStack.push(OperatorLeftParen)

		// Top should be LeftParen (depth 0)
		top, ok := opStack.peekDepth(0)
		if !ok || top != OperatorLeftParen {
			t.Errorf("peekDepth(0) = (%v, %v), want (%v, true)", top, ok, OperatorLeftParen)
		}

		// Preceding operator should be OperatorContains (depth 1)
		parentOp, ok := opStack.peekDepth(1)
		if !ok || parentOp != OperatorContains {
			t.Errorf("peekDepth(1) = (%v, %v), want (%v, true)", parentOp, ok, OperatorContains)
		}

		// Nothing deeper than that
		_, ok = opStack.peekDepth(2)
		if ok {
			t.Errorf("peekDepth(2) should be out of bounds, got ok=true")
		}
	})
}
