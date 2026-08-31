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
