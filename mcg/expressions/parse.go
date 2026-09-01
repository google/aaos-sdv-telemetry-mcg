// Copyright 2024 Google LLC
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

// The expressions package implements a Shunting Yard parser algorithm with a tokenizer to parse
// user-supplied expressions into a MetricsConfig expression node tree.
//
// The primary entry point is `NewParserShunt().CompileAll(..)`.
package expressions

import (
	"fmt"
	"log"
	"math"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	mcgerrors "sdv.googlesource.com/mcg/mcg/errors"
	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

type Text struct {
	Uncompiled string
}

type operand struct {
	index         uint32
	isComparison  bool
	parenthesized bool
}

type stack[T any] []T

func (s *stack[T]) push(item T) {
	*s = append(*s, item)
}

func (s *stack[T]) pop() T {
	index := len(*s) - 1
	top := (*s)[index]
	var zero T
	(*s)[index] = zero
	*s = (*s)[:index]
	return top
}

func (s stack[T]) peek() T {
	return s[len(s)-1]
}

func (s stack[T]) isEmpty() bool {
	return len(s) == 0
}

type ParserShunt struct {
	nodes                              []*pb.Node
	nodeByWire                         map[string]uint32
	disallowComparisonOperatorChaining bool
}

// NewParserShunt returns a new ParserShunt initialized with the given comparison chaining configuration.
func NewParserShunt(disallowComparisonOperatorChaining bool) *ParserShunt {
	return &ParserShunt{
		nodeByWire:                         make(map[string]uint32),
		disallowComparisonOperatorChaining: disallowComparisonOperatorChaining,
	}
}

// CompileAll compiles every expression in a session into MetricsConfig expression nodes.
//
// The returned map is a mapping from the uint32 key in the argument map to the position of that expression's root node in the returned node list.
func (p *ParserShunt) CompileAll(uncompiled map[uint32]Text) (map[uint32]uint32, []*pb.Node, error) {
	sessionIdxs := make([]uint32, 0, len(uncompiled))
	for sessionIdx := range uncompiled {
		sessionIdxs = append(sessionIdxs, sessionIdx)
	}
	sort.Slice(sessionIdxs, func(i, j int) bool { return sessionIdxs[i] < sessionIdxs[j] })
	idxMap := make(map[uint32]uint32)
	for _, sessionIdx := range sessionIdxs {
		n, err := p.compileOne(uncompiled[sessionIdx].Uncompiled)
		if err != nil {
			return nil, nil, err
		}
		idxMap[sessionIdx] = n
	}
	return idxMap, p.nodes, nil
}

// Push a deduplicated node proto into the parsing session and return the index.
func (p *ParserShunt) pushNode(n *pb.Node) uint32 {
	by, err := proto.MarshalOptions{Deterministic: true}.Marshal(n)
	if err != nil {
		panic(err)
	}
	if idx, ok := p.nodeByWire[string(by)]; ok {
		return idx
	}
	// No dupe found, insert
	i := len(p.nodes)
	if i >= 0xFFFF_FFFF {
		panic("uint32 overflow")
	}
	p.nodes = append(p.nodes, n)
	p.nodeByWire[string(by)] = uint32(i)
	return uint32(i)
}

// token is one of the following:
//
// bool, int32, int64, float32, float64, Operator, []string, *pb.Node
type token any

// Complete compilation flow for a single expression. Returns the root node index.
func (p *ParserShunt) compileOne(source string) (uint32, error) {
	tokens, err := p.tokenize(source)
	if err != nil {
		return 0, mcgerrors.InvalidExpressionError(source, err)
	}

	operandStack := make(stack[operand], 0, 8)
	operatorStack := make(stack[Operator], 0, 8)
	for i, tok := range tokens {
		switch ttok := tok.(type) {
		case []string:
			if len(ttok) == 0 {
				break
			}
			pub, fields := ttok[0], ttok[1:]
			fieldLeafNodeBuilder := pb.FieldLeafNode_builder{
				SourceName: pub,
				FieldNames: fields,
			}
			if pub == "" {
				if i == 0 {
					return 0, mcgerrors.InvalidExpressionError(source, fmt.Errorf("postfix field access must follow an expression"))
				}
				if isOperand(tokens[i-1]) {
					// Postfix field access acts as an operator with the same precedence as Subscript.
					// We must collapse the operator stack for any operators with higher or equal precedence.
					if err := p.collapseOperators(precedence[OperatorSubscript], &operandStack, &operatorStack); err != nil {
						return 0, mcgerrors.InvalidExpressionError(source, err)
					}

					if operandStack.isEmpty() {
						return 0, mcgerrors.InvalidExpressionError(source, fmt.Errorf("empty operand stack for postfix field access"))
					}
					lhs := operandStack.pop()
					fieldLeafNodeBuilder.ExpressionNodeIndex = proto.Uint32(lhs.index)
				}
			}
			p.pushNodeToOperandStack(pb.Node_builder{FieldLeafNode: fieldLeafNodeBuilder.Build()}.Build(), &operandStack, false)
		case bool:
			p.pushNodeToOperandStack(pb.Node_builder{
				ConstantLeafNode: pb.ConstantLeafNode_builder{
					BoolValue: &ttok,
				}.Build(),
			}.Build(), &operandStack, false)
		case int32:
			p.pushNodeToOperandStack(pb.Node_builder{
				ConstantLeafNode: pb.ConstantLeafNode_builder{
					Int32Value: &ttok,
				}.Build(),
			}.Build(), &operandStack, false)
		case int64:
			p.pushNodeToOperandStack(pb.Node_builder{
				ConstantLeafNode: pb.ConstantLeafNode_builder{
					Int64Value: &ttok,
				}.Build(),
			}.Build(), &operandStack, false)
		case float32:
			p.pushNodeToOperandStack(pb.Node_builder{
				ConstantLeafNode: pb.ConstantLeafNode_builder{
					FloatValue: &ttok,
				}.Build(),
			}.Build(), &operandStack, false)
		case float64:
			p.pushNodeToOperandStack(pb.Node_builder{
				ConstantLeafNode: pb.ConstantLeafNode_builder{
					DoubleValue: &ttok,
				}.Build(),
			}.Build(), &operandStack, false)
		case *pb.Node:
			p.pushNodeToOperandStack(ttok, &operandStack, false)
		case Operator:
			switch ttok {
			case OperatorAllEq, OperatorContains, OperatorDoesNotContain, OperatorFloor, OperatorRound, OperatorCeil, OperatorAbsolute, OperatorLength:
				if i+1 >= len(tokens) {
					return 0, mcgerrors.InvalidExpressionError(source, fmt.Errorf("Missing operand for unary operator: %v", ttok))
				}
				if nextOp, ok := tokens[i+1].(Operator); !ok || nextOp != OperatorLeftParen {
					return 0, mcgerrors.InvalidExpressionError(source, fmt.Errorf("%v requires parentheses", ttok))
				}
				operatorStack.push(ttok)
			case OperatorUnaryMinus, OperatorNot:
				operatorStack.push(ttok)
			case OperatorLeftParen:
				operatorStack.push(ttok)
			case OperatorRightParen:
				if err := p.handleRightParen(&operandStack, &operatorStack); err != nil {
					return 0, mcgerrors.InvalidExpressionError(source, err)
				}
			case OperatorLeftSquareBracket:
				if err := p.collapseOperators(precedence[OperatorSubscript], &operandStack, &operatorStack); err != nil {
					return 0, mcgerrors.InvalidExpressionError(source, err)
				}
				operatorStack.push(OperatorSubscript)
				operatorStack.push(OperatorLeftSquareBracket)
			case OperatorRightSquareBracket:
				if err := p.handleRightSquareBracket(&operandStack, &operatorStack); err != nil {
					return 0, mcgerrors.InvalidExpressionError(source, err)
				}
			default:
				if err := p.collapseOperators(precedence[ttok], &operandStack, &operatorStack); err != nil {
					return 0, mcgerrors.InvalidExpressionError(source, err)
				}
				operatorStack.push(ttok)
			}
		}
	}
	for !operatorStack.isEmpty() {
		if slices.Contains(operatorStack, OperatorLeftParen) {
			return 0, mcgerrors.InvalidExpressionError(source, fmt.Errorf("Found opening parenthesis without matching closing parenthesis"))
		}
		if slices.Contains(operatorStack, OperatorLeftSquareBracket) {
			return 0, mcgerrors.InvalidExpressionError(source, fmt.Errorf("Found opening square bracket without matching closing square bracket"))
		}

		lastOp := operatorStack.pop()

		n, err := p.buildCombinationNode(lastOp, &operandStack)
		if err != nil {
			return 0, mcgerrors.InvalidExpressionError(source, err)
		}
		p.pushNodeToOperandStack(n, &operandStack, isComparisonOperator(lastOp))
	}
	if len(operandStack) > 1 {
		return 0, mcgerrors.InvalidExpressionError(source, fmt.Errorf("Found operand(s) but no operator"))
	} else if operandStack.isEmpty() {
		return 0, mcgerrors.InvalidExpressionError(source, fmt.Errorf("Found no valid operands"))
	}
	return operandStack.peek().index, nil
}

// isFunctionLike returns true if op uses parentheses like a function call.
func isFunctionLike(op Operator) bool {
	switch op {
	case OperatorAllEq, OperatorContains, OperatorDoesNotContain, OperatorFloor, OperatorRound, OperatorCeil, OperatorAbsolute, OperatorLength:
		return true
	default:
		return false
	}
}

// isUnaryOperator returns true if op is a unary operator.
func isUnaryOperator(op Operator) bool {
	switch op {
	case OperatorFloor, OperatorRound, OperatorCeil, OperatorNot, OperatorAbsolute, OperatorUnaryMinus, OperatorLength:
		return true
	default:
		return false
	}
}

// isComparisonOperator returns true if op is a binary comparison operator.
func isComparisonOperator(op Operator) bool {
	switch op {
	case OperatorEq, OperatorNotEq, OperatorLt, OperatorLtEq, OperatorGt, OperatorGtEq:
		return true
	default:
		return false
	}
}

func isOperand(tok token) bool {
	switch t := tok.(type) {
	case bool, int32, int64, float32, float64, []string, *pb.Node:
		return true
	case Operator:
		return t == OperatorRightParen || t == OperatorRightSquareBracket
	}
	return false
}

func (p *ParserShunt) collapseOperators(targetPrecedence int8, operandStack *stack[operand], operatorStack *stack[Operator]) error {
	for !operatorStack.isEmpty() {
		lastOp := operatorStack.peek()
		if precedence[lastOp] < targetPrecedence {
			break
		}
		operatorStack.pop()
		n, err := p.buildCombinationNode(lastOp, operandStack)
		if err != nil {
			return err
		}
		p.pushNodeToOperandStack(n, operandStack, isComparisonOperator(lastOp))
	}
	return nil
}

// Processes the stack when token ')' is encountered. Builds nodes until a
// matching '(' is found. Returns errors for any mismatched or redundant parenthesis.
func (p *ParserShunt) handleRightParen(operandStack *stack[operand], operatorStack *stack[Operator]) error {
	foundOperator := false
	for !operatorStack.isEmpty() {
		op := operatorStack.pop()

		if op == OperatorLeftParen {
			if !foundOperator {
				if operatorStack.isEmpty() || (!isFunctionLike(operatorStack.peek()) && !isUnaryOperator(operatorStack.peek())) {
					return fmt.Errorf("Found redundant parenthesis")
				}
			}
			if !operandStack.isEmpty() {
				top := operandStack.pop()
				top.parenthesized = true
				operandStack.push(top)
			}
			if !operatorStack.isEmpty() && isFunctionLike(operatorStack.peek()) {
				fnOp := operatorStack.pop()
				n, err := p.buildCombinationNode(fnOp, operandStack)
				if err != nil {
					return err
				}
				p.pushNodeToOperandStack(n, operandStack, isComparisonOperator(fnOp))
			}
			return nil
		}
		if op == OperatorLeftSquareBracket {
			return fmt.Errorf("Found closing parenthesis without matching opening parenthesis")
		}

		foundOperator = true
		n, err := p.buildCombinationNode(op, operandStack)
		if err != nil {
			return err
		}
		p.pushNodeToOperandStack(n, operandStack, isComparisonOperator(op))
	}

	return fmt.Errorf("Found closing parenthesis without matching opening parenthesis")
}

func (p *ParserShunt) handleRightSquareBracket(operandStack *stack[operand], operatorStack *stack[Operator]) error {
	for !operatorStack.isEmpty() {
		op := operatorStack.pop()

		if op == OperatorLeftSquareBracket {
			if operatorStack.isEmpty() || operatorStack.peek() != OperatorSubscript {
				return fmt.Errorf("Found square bracket without matching subscript operator")
			}
			return nil
		}
		if op == OperatorLeftParen {
			return fmt.Errorf("Found closing square bracket without matching opening square bracket")
		}

		n, err := p.buildCombinationNode(op, operandStack)
		if err != nil {
			return err
		}
		p.pushNodeToOperandStack(n, operandStack, isComparisonOperator(op))
	}

	return fmt.Errorf("Found closing square bracket without matching opening square bracket")
}

func (p *ParserShunt) getOperatorToProto(op Operator) *pb.CombinationNode {
	return operatorToProto[op]
}

// Pop the correct number of operands off the stack and create a Node.
func (p *ParserShunt) buildCombinationNode(op Operator, operandStack *stack[operand]) (*pb.Node, error) {
	var left, right operand
	var rightIndex *uint32

	if isUnaryOperator(op) {
		if operandStack.isEmpty() {
			return nil, fmt.Errorf("Missing operand for unary operator: %v", op)
		}
		left = operandStack.pop()
	} else { // Binary operator
		if len(*operandStack) < 2 {
			return nil, fmt.Errorf("Missing operand(s) for binary operator: %v", op)
		}
		right = operandStack.pop()
		left = operandStack.pop()
		rightIndex = proto.Uint32(right.index)

		if isComparisonOperator(op) && ((left.isComparison && !left.parenthesized) || (right.isComparison && !right.parenthesized)) {
			if p.disallowComparisonOperatorChaining {
				return nil, fmt.Errorf("comparison operator chaining is disallowed; use parentheses or logical operators (e.g. &&) to clarify intent")
			}
			log.Printf("Warning: comparison operator chaining detected; this will be disallowed in a future release (opt in now using query parameter disallow_comparison_operator_chaining=true)")
		}
	}

	n := pb.Node_builder{
		CombinationNode: pb.CombinationNode_builder{
			LeftIndex:  proto.Uint32(left.index),
			RightIndex: rightIndex,
		}.Build(),
	}.Build()

	operator := p.getOperatorToProto(op)
	switch operator.WhichOperator() {
	// go/keep-sorted start
	case pb.CombinationNode_ArithmeticOperator_case:
		n.GetCombinationNode().SetArithmeticOperator(operator.GetArithmeticOperator())
	case pb.CombinationNode_ListOperator_case:
		n.GetCombinationNode().SetListOperator(operator.GetListOperator())
	case pb.CombinationNode_LogicalOperator_case:
		n.GetCombinationNode().SetLogicalOperator(operator.GetLogicalOperator())
	case pb.CombinationNode_RelationalOperator_case:
		n.GetCombinationNode().SetRelationalOperator(operator.GetRelationalOperator())
	case pb.CombinationNode_RoundingOperator_case:
		n.GetCombinationNode().SetRoundingOperator(operator.GetRoundingOperator())
		// go/keep-sorted end
	}
	return n, nil
}

func (p *ParserShunt) pushNodeToOperandStack(nod *pb.Node, operandStack *stack[operand], isComparison bool) {
	id := p.pushNode(nod)
	operandStack.push(operand{
		index:         id,
		isComparison:  isComparison,
		parenthesized: false,
	})
}

func (p *ParserShunt) tokenize(source string) ([]token, error) {
	var tokens []token
	i := 0

	for {
		if i >= len(source) {
			return tokens, nil
		}
		c, adv := utf8.DecodeRuneInString(source[i:])
		if ('0' <= c && c <= '9') || c == '.' || c == '_' || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z') {
			l := peekFunctionLen(source[i:])
			if l > 0 {
				fun, err := parseFunction(source[i : i+l])
				if err != nil {
					return nil, err
				}
				tokens = append(tokens, fun)
			} else {
				l = peekValueLen(source[i:], adv)
				if l == 0 {
					panic("peekValueLen disagrees with character matching")
				}
				val, err := parseValue(source[i : i+l])
				if err != nil {
					return nil, err
				}
				tokens = append(tokens, val)
			}
			adv = l
		} else if c == '-' {
			lenNumber := peekNumberLen(source[i+1:])
			if isLastTokenOperator(tokens) {
				if lenNumber > 0 {
					val, err := parseValue(source[i : i+1+lenNumber])
					if err != nil {
						return nil, err
					}
					tokens = append(tokens, val)
					adv = 1 + lenNumber
				} else if lenInf := peekInfLen(source[i+1:]); lenInf > 0 {
					val, err := parseValue(source[i : i+1+lenInf])
					if err != nil {
						return nil, err
					}
					tokens = append(tokens, val)
					adv = 1 + lenInf
				} else {
					tokens = append(tokens, OperatorUnaryMinus)
				}
			} else {
				tokens = append(tokens, OperatorSubtract)
			}
		} else if c == '!' {
			if strings.HasPrefix(source[i:], "!=") {
				tokens = append(tokens, OperatorNotEq)
				adv = 2
			} else {
				tokens = append(tokens, OperatorNot)
				adv = 1
			}
		} else if c == '(' {
			tokens = append(tokens, OperatorLeftParen)
		} else if c == ')' {
			tokens = append(tokens, OperatorRightParen)
		} else if c == '[' {
			tokens = append(tokens, OperatorLeftSquareBracket)
		} else if c == ']' {
			tokens = append(tokens, OperatorRightSquareBracket)
		} else if c == ',' {
			// skip commas
		} else if unicode.IsSpace(c) {
			// skip whitespace
		} else {
			l := peekOperator(source[i:])
			val, err := parseOperator(source[i : i+l])
			if err != nil {
				return nil, err
			}
			tokens = append(tokens, val)
			adv = l
		}
		i += adv
	}
}

// Returns whether the last token represents an operator. Assumes that left
// parentheses follow operators and right parentheses follow expressions that
// evaluate to operands.
func isLastTokenOperator(tok []token) bool {
	if len(tok) == 0 {
		return true
	}
	op, ok := tok[len(tok)-1].(Operator)
	if ok && (op == OperatorRightParen || op == OperatorRightSquareBracket) {
		return false
	}
	return ok
}

var (
	rgxInf                = regexp.MustCompile(`^(?i:infinity|inf)(?:[^0-9a-zA-Z_]|$)`)
	rgxNumber             = regexp.MustCompile("^[0-9.]+")
	rgxNumberOrIdentifier = regexp.MustCompile("^[0-9a-zA-Z._]+")
	rgxEndOperator        = regexp.MustCompile("[0-9a-zA-Z._)(" + rgxFragSpaces + rgxUnaryMinus + "\\[\\]]")
	// All space characters in Latin-1 plus the Z character class
	rgxFragSpaces = ` \t\n\v\f\r\x85\xa0\pZ`
	// A unary minus can directly follow another operator and as such needs to
	// be included in the list of characters that can signal the end of a single
	// operator.
	rgxUnaryMinus = "\\-"
	rgxFunction   = regexp.MustCompile(`^timestamp\(.*?\)`)
)

// Returns the number of contiguous bytes representing "inf" or "infinity" (case-insensitive)
func peekInfLen(s string) int {
	loc := rgxInf.FindStringIndex(s)
	if loc == nil {
		return 0
	}
	match := s[:loc[1]]
	if len(match) >= 8 && strings.EqualFold(match[:8], "infinity") {
		return 8
	}
	if len(match) >= 3 && strings.EqualFold(match[:3], "inf") {
		return 3
	}
	return 0
}

// Returns the number of contiguous bytes that might comprise a number
func peekNumberLen(s string) int {
	loc := rgxNumber.FindStringIndex(s)
	if loc == nil {
		return 0
	}
	return loc[1]
}

// Returns the number of contiguous bytes that represent a function expression
// (from the first letter of the function name to its closing bracket) or 0 if
// no function was found
func peekFunctionLen(s string) int {
	loc := rgxFunction.FindStringIndex(s)
	if loc == nil {
		return 0
	}
	return loc[1]
}

// Parses a string into a FunctionLeafNode or returns an error if the parameters
// to the function are invalid
func parseFunction(s string) (*pb.Node, error) {
	switch s {
	case "timestamp(REALTIME_CLOCK)":
		return pb.Node_builder{
			FunctionLeafNode: pb.FunctionLeafNode_builder{
				GetCurrentTimestamp: pb.GetCurrentTimestampFunction_builder{
					TimeSource: pb.GetCurrentTimestampFunction_REALTIME_CLOCK,
				}.Build(),
			}.Build(),
		}.Build(), nil
	case "timestamp(MONOTONIC_TIME_SINCE_BOOT)":
		return pb.Node_builder{
			FunctionLeafNode: pb.FunctionLeafNode_builder{
				GetCurrentTimestamp: pb.GetCurrentTimestampFunction_builder{
					TimeSource: pb.GetCurrentTimestampFunction_MONOTONIC_TIME_SINCE_BOOT,
				}.Build(),
			}.Build(),
		}.Build(), nil
	case "timestamp(MONOTONIC_TIME_SINCE_BOOT_OR_RESUME)":
		return pb.Node_builder{
			FunctionLeafNode: pb.FunctionLeafNode_builder{
				GetCurrentTimestamp: pb.GetCurrentTimestampFunction_builder{
					TimeSource: pb.GetCurrentTimestampFunction_MONOTONIC_TIME_SINCE_BOOT_OR_RESUME,
				}.Build(),
			}.Build(),
		}.Build(), nil
	default:
		param := regexp.MustCompile(`^timestamp\((.*)\)`).FindStringSubmatch(s)
		if strings.TrimSpace(param[1]) == "" {
			return nil, fmt.Errorf("timestamp function requires a parameter")
		}
		if strings.Contains(param[1], ",") {
			return nil, fmt.Errorf("timestamp function expects exactly one parameter")
		}
		return nil, fmt.Errorf("%q is not a valid timestamp parameter", param[1])
	}
}

// Returns the number of contiguous bytes that satisfy the number-or-identifier check.
func peekValueLen(s string, startAt int) int {
	loc := rgxNumberOrIdentifier.FindStringIndex(s)
	if loc == nil {
		return 0
	}
	if loc[0] != 0 {
		panic("peekValueLen regex not anchored")
	}
	if loc[1] < startAt {
		panic("peekValueLen found less than 1 character")
	}
	return loc[1]
}

// Returns the number of contiguous bytes that might comprise an operator.
//
// Uses a regex search for the first thing that parses as anything other than an operator.
func peekOperator(s string) int {
	loc := rgxEndOperator.FindStringIndex(s)
	if loc == nil {
		return len(s)
	}
	return loc[0]
}

func parseValue(s string) (token, error) {
	switch s {
	case "":
		return nil, fmt.Errorf("internal parsing error: empty string")

	case "true":
		return bool(true), nil
	case "false":
		return bool(false), nil

	case "alleq":
		return OperatorAllEq, nil
	case "contains":
		return OperatorContains, nil
	case "doesnotcontain":
		return OperatorDoesNotContain, nil
	case "floor":
		return OperatorFloor, nil
	case "round":
		return OperatorRound, nil
	case "ceil":
		return OperatorCeil, nil
	case "abs":
		return OperatorAbsolute, nil
	case "length":
		return OperatorLength, nil
	}

	i32, err := strconv.ParseInt(s, 10, 32)
	if err == nil {
		return int32(i32), nil
	}
	i64, err := strconv.ParseInt(s, 10, 64)
	if err == nil {
		return int64(i64), nil
	}

	if !strings.ContainsAny(s, "eEpP") {
		f64, err := strconv.ParseFloat(s, 64)
		if err == nil {
			if f32 := float32(f64); float64(f32) == f64 || (math.IsNaN(float64(f32)) && math.IsNaN(f64)) {
				return f32, nil
			}
			return float64(f64), nil
		}
	}
	split := strings.Split(s, ".")
	if strings.HasPrefix(s, ".") {
		if len(split) <= 1 {
			return nil, fmt.Errorf("invalid field path %q: empty field part", s)
		}
		for _, part := range split[1:] {
			if part == "" {
				return nil, fmt.Errorf("invalid field path %q: empty field part", s)
			}
		}
	} else {
		for _, part := range split {
			if part == "" {
				return nil, fmt.Errorf("invalid field path %q: empty field part", s)
			}
		}
	}
	return []string(split), nil
}

// Operator is a type internal to the parser.
type Operator int8

// LINT.IfChange

const (
	OperatorInvalid Operator = iota
	// go/keep-sorted start
	OperatorAbsolute
	OperatorAdd
	OperatorAllEq
	OperatorAnd
	OperatorCeil
	OperatorContains
	OperatorDivide
	OperatorDoesNotContain
	OperatorEq
	OperatorFloor
	OperatorGt
	OperatorGtEq
	OperatorLeftParen
	OperatorLeftSquareBracket
	OperatorLength
	OperatorLt
	OperatorLtEq
	OperatorModulo
	OperatorMultiply
	OperatorNot
	OperatorNotEq
	OperatorOr
	OperatorPower
	OperatorRightParen
	OperatorRightSquareBracket
	OperatorRound
	OperatorSubscript
	OperatorSubtract
	OperatorUnaryMinus
	OperatorXor
	// go/keep-sorted end
)

// LINT.ThenChange(parse_test.go)

var precedence []int8 = []int8{
	OperatorInvalid: 0, OperatorLeftParen: 0, OperatorRightParen: 0, OperatorLeftSquareBracket: 0, OperatorRightSquareBracket: 0,
	OperatorOr:  4,
	OperatorAnd: 5,
	OperatorXor: 7,
	OperatorEq:  9, OperatorNotEq: 9,
	OperatorGtEq: 10, OperatorGt: 10, OperatorLtEq: 10, OperatorLt: 10,
	OperatorAdd: 12, OperatorSubtract: 12, OperatorUnaryMinus: 12,
	OperatorMultiply: 13, OperatorDivide: 13, OperatorModulo: 13,
	OperatorNot:   14,
	OperatorPower: 15, OperatorAllEq: 15, OperatorContains: 15, OperatorDoesNotContain: 15, OperatorFloor: 15, OperatorRound: 15, OperatorCeil: 15, OperatorAbsolute: 15, OperatorLength: 15, OperatorSubscript: 15,
}

var operatorToProto []*pb.CombinationNode = []*pb.CombinationNode{
	// go/keep-sorted start
	OperatorAbsolute:       pb.CombinationNode_builder{ArithmeticOperator: pb.CombinationNode_ABSOLUTE.Enum()}.Build(),
	OperatorAdd:            pb.CombinationNode_builder{ArithmeticOperator: pb.CombinationNode_ADD.Enum()}.Build(),
	OperatorAllEq:          pb.CombinationNode_builder{RelationalOperator: pb.CombinationNode_EQ.Enum()}.Build(),
	OperatorAnd:            pb.CombinationNode_builder{LogicalOperator: pb.CombinationNode_AND.Enum()}.Build(),
	OperatorCeil:           pb.CombinationNode_builder{RoundingOperator: pb.CombinationNode_CEIL.Enum()}.Build(),
	OperatorContains:       pb.CombinationNode_builder{RelationalOperator: pb.CombinationNode_CONTAINS.Enum()}.Build(),
	OperatorDivide:         pb.CombinationNode_builder{ArithmeticOperator: pb.CombinationNode_DIVIDE.Enum()}.Build(),
	OperatorDoesNotContain: pb.CombinationNode_builder{RelationalOperator: pb.CombinationNode_DOES_NOT_CONTAIN.Enum()}.Build(),
	OperatorEq:             pb.CombinationNode_builder{RelationalOperator: pb.CombinationNode_EQ.Enum()}.Build(),
	OperatorFloor:          pb.CombinationNode_builder{RoundingOperator: pb.CombinationNode_FLOOR.Enum()}.Build(),
	OperatorGt:             pb.CombinationNode_builder{RelationalOperator: pb.CombinationNode_GT.Enum()}.Build(),
	OperatorGtEq:           pb.CombinationNode_builder{RelationalOperator: pb.CombinationNode_GT_OR_EQ.Enum()}.Build(),
	OperatorLength:         pb.CombinationNode_builder{ListOperator: pb.CombinationNode_LENGTH.Enum()}.Build(),
	OperatorLt:             pb.CombinationNode_builder{RelationalOperator: pb.CombinationNode_LT.Enum()}.Build(),
	OperatorLtEq:           pb.CombinationNode_builder{RelationalOperator: pb.CombinationNode_LT_OR_EQ.Enum()}.Build(),
	OperatorModulo:         pb.CombinationNode_builder{ArithmeticOperator: pb.CombinationNode_MODULO_TRUNC.Enum()}.Build(),
	OperatorMultiply:       pb.CombinationNode_builder{ArithmeticOperator: pb.CombinationNode_MULTIPLY.Enum()}.Build(),
	OperatorNot:            pb.CombinationNode_builder{LogicalOperator: pb.CombinationNode_NOT.Enum()}.Build(),
	OperatorNotEq:          pb.CombinationNode_builder{RelationalOperator: pb.CombinationNode_NOT_EQ.Enum()}.Build(),
	OperatorOr:             pb.CombinationNode_builder{LogicalOperator: pb.CombinationNode_OR.Enum()}.Build(),
	OperatorPower:          pb.CombinationNode_builder{ArithmeticOperator: pb.CombinationNode_POWER.Enum()}.Build(),
	OperatorRound:          pb.CombinationNode_builder{RoundingOperator: pb.CombinationNode_ROUND.Enum()}.Build(),
	OperatorSubscript:      pb.CombinationNode_builder{ListOperator: pb.CombinationNode_SUBSCRIPT.Enum()}.Build(),
	OperatorSubtract:       pb.CombinationNode_builder{ArithmeticOperator: pb.CombinationNode_SUBTRACT.Enum()}.Build(),
	OperatorUnaryMinus:     pb.CombinationNode_builder{ArithmeticOperator: pb.CombinationNode_UNARY_MINUS.Enum()}.Build(),
	OperatorXor:            pb.CombinationNode_builder{LogicalOperator: pb.CombinationNode_XOR.Enum()}.Build(),
	// go/keep-sorted end
}

func parseOperator(s string) (token, error) {
	switch s {
	case "+":
		return OperatorAdd, nil
	case "-":
		return OperatorSubtract, nil
	case "*":
		return OperatorMultiply, nil
	case "/":
		return OperatorDivide, nil
	case "**":
		return OperatorPower, nil
	case "%":
		return OperatorModulo, nil
	case "&&":
		return OperatorAnd, nil
	case "||":
		return OperatorOr, nil
	case "^":
		return OperatorXor, nil
	case "==":
		return OperatorEq, nil
	case ">=":
		return OperatorGtEq, nil
	case ">":
		return OperatorGt, nil
	case "<=":
		return OperatorLtEq, nil
	case "<":
		return OperatorLt, nil
	case "alleq":
		return OperatorAllEq, nil
	case "contains":
		return OperatorContains, nil
	case "doesnotcontain":
		return OperatorDoesNotContain, nil
	case "ceil":
		return OperatorCeil, nil
	case "floor":
		return OperatorFloor, nil
	case "round":
		return OperatorRound, nil
	case "abs":
		return OperatorAbsolute, nil
	case "length":
		return OperatorLength, nil
	}
	return nil, fmt.Errorf("Unknown operator %q", s)
}

// String implements fmt.Stringer.
func (op Operator) String() string {
	return operatorToString[op]
}

var operatorToString []string = []string{
	// go/keep-sorted start
	OperatorAbsolute:           "OperatorAbsolute",
	OperatorAdd:                "OperatorAdd",
	OperatorAllEq:              "OperatorAllEq",
	OperatorAnd:                "OperatorAnd",
	OperatorCeil:               "OperatorCeil",
	OperatorContains:           "OperatorContains",
	OperatorDivide:             "OperatorDivide",
	OperatorDoesNotContain:     "OperatorDoesNotContain",
	OperatorEq:                 "OperatorEq",
	OperatorFloor:              "OperatorFloor",
	OperatorGt:                 "OperatorGt",
	OperatorGtEq:               "OperatorGtEq",
	OperatorLeftParen:          "OperatorLeftParen",
	OperatorLeftSquareBracket:  "OperatorLeftSquareBracket",
	OperatorLength:             "OperatorLength",
	OperatorLt:                 "OperatorLt",
	OperatorLtEq:               "OperatorLtEq",
	OperatorModulo:             "OperatorModulo",
	OperatorMultiply:           "OperatorMultiply",
	OperatorNot:                "OperatorNot",
	OperatorNotEq:              "OperatorNotEq",
	OperatorOr:                 "OperatorOr",
	OperatorPower:              "OperatorPower",
	OperatorRightParen:         "OperatorRightParen",
	OperatorRightSquareBracket: "OperatorRightSquareBracket",
	OperatorRound:              "OperatorRound",
	OperatorSubtract:           "OperatorSubtract",
	OperatorSubscript:          "OperatorSubscript",
	OperatorUnaryMinus:         "OperatorUnaryMinus",
	OperatorXor:                "OperatorXor",
	// go/keep-sorted end
}
