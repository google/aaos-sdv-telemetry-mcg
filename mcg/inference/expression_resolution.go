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

package inference

import (
	"fmt"
	"strings"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"

	pb "sdv.googlesource.com/mcg/third_party/aosp/sdv/telemetry/metrics_configuration"
)

// ExpressionResolver provides stateless, recursive evaluation of AST nodes
// into their representative protobuf field descriptors.
type ExpressionResolver struct {
	expressionNodes      []*pb.Node
	getSourceMessageName func(sourceName string) (protoreflect.FullName, error)
	getDescriptorProto   func(protoreflect.FullName) *descriptorpb.DescriptorProto
}

// NewExpressionResolver initializes an ExpressionResolver with the provided AST nodes
// and resolution callback hooks.
func NewExpressionResolver(
	expressionNodes []*pb.Node,
	getSourceMessageName func(sourceName string) (protoreflect.FullName, error),
	getDescriptorProto func(protoreflect.FullName) *descriptorpb.DescriptorProto,
) *ExpressionResolver {
	return &ExpressionResolver{
		expressionNodes:      expressionNodes,
		getSourceMessageName: getSourceMessageName,
		getDescriptorProto:   getDescriptorProto,
	}
}

// Resolve evaluates the expression AST node at the given index and returns a
// FieldDescriptorProto that describes the output of the node when evaluated.
func (er *ExpressionResolver) Resolve(idx uint32) (*descriptorpb.FieldDescriptorProto, error) {
	expNode := er.expressionNodes[idx]
	switch expNode.WhichNodeType() {
	// FieldLeafNodes resolve external Data Sources or Aggregators.
	case pb.Node_FieldLeafNode_case:
		return er.resolveFieldLeafNode(expNode.GetFieldLeafNode())
	// Combination nodes continue recursion.
	case pb.Node_CombinationNode_case:
		return er.resolveCombinationNode(expNode.GetCombinationNode())
	// Functions and Constants terminate recursion.
	case pb.Node_FunctionLeafNode_case:
		return resolveFunctionLeafNode(expNode.GetFunctionLeafNode())
	case pb.Node_ConstantLeafNode_case:
		return resolveConstLeafNode(expNode.GetConstantLeafNode())
	default:
		return nil, fmt.Errorf("Could not resolve Expression Node Type %q", expNode.WhichNodeType().String())
	}
}

func resolveFunctionLeafNode(expNode *pb.FunctionLeafNode) (*descriptorpb.FieldDescriptorProto, error) {
	fieldDesc := &descriptorpb.FieldDescriptorProto{}

	switch expNode.WhichFunctionType() {
	case pb.FunctionLeafNode_GetCurrentTimestamp_case:
		// GetCurrentTimestamp Function returns an INT64
		fieldDesc.Type = descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()
	default:
		return nil, fmt.Errorf("Could not resolve FunctionLeafNode Type: %s", expNode.WhichFunctionType().String())
	}
	return fieldDesc, nil
}

func resolveConstLeafNode(expNode *pb.ConstantLeafNode) (*descriptorpb.FieldDescriptorProto, error) {
	fieldDesc := &descriptorpb.FieldDescriptorProto{}

	switch expNode.WhichNodeValue() {
	case pb.ConstantLeafNode_BoolValue_case:
		fieldDesc.Type = descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()
	case pb.ConstantLeafNode_Int32Value_case:
		fieldDesc.Type = descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum()
	case pb.ConstantLeafNode_Int64Value_case:
		fieldDesc.Type = descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()
	case pb.ConstantLeafNode_FloatValue_case:
		fieldDesc.Type = descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Enum()
	case pb.ConstantLeafNode_DoubleValue_case:
		fieldDesc.Type = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()
	default:
		return nil, fmt.Errorf("Could not resolve ConstantLeafNode NodeValue: %s", expNode.WhichNodeValue().String())
	}
	return fieldDesc, nil
}

func isInt(nodeType *descriptorpb.FieldDescriptorProto_Type) bool {
	if nodeType.Number() == descriptorpb.FieldDescriptorProto_TYPE_INT32.Number() ||
		nodeType.Number() == descriptorpb.FieldDescriptorProto_TYPE_INT64.Number() ||
		nodeType.Number() == descriptorpb.FieldDescriptorProto_TYPE_UINT32.Number() ||
		nodeType.Number() == descriptorpb.FieldDescriptorProto_TYPE_UINT64.Number() ||
		nodeType.Number() == descriptorpb.FieldDescriptorProto_TYPE_SINT32.Number() ||
		nodeType.Number() == descriptorpb.FieldDescriptorProto_TYPE_SINT64.Number() {
		return true
	}
	return false
}

func isNumeric(nodeType *descriptorpb.FieldDescriptorProto_Type) bool {
	if nodeType.Number() == descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Number() ||
		nodeType.Number() == descriptorpb.FieldDescriptorProto_TYPE_FLOAT.Number() ||
		nodeType.Number() == descriptorpb.FieldDescriptorProto_TYPE_FIXED32.Number() ||
		nodeType.Number() == descriptorpb.FieldDescriptorProto_TYPE_FIXED64.Number() ||
		nodeType.Number() == descriptorpb.FieldDescriptorProto_TYPE_SFIXED64.Number() ||
		nodeType.Number() == descriptorpb.FieldDescriptorProto_TYPE_SFIXED32.Number() ||
		isInt(nodeType) {
		return true
	}
	return false
}

func (er *ExpressionResolver) resolveFieldLeafNode(expNode *pb.FieldLeafNode) (*descriptorpb.FieldDescriptorProto, error) {
	sourceName := expNode.GetSourceName()
	fieldNames := expNode.GetFieldNames()

	msgFullName, err := er.getSourceMessageName(sourceName)
	if err != nil {
		return nil, err
	}

	return er.resolveFieldPath(msgFullName, fieldNames)
}

func (er *ExpressionResolver) resolveFieldPath(msgFullName protoreflect.FullName, fieldNames []string) (*descriptorpb.FieldDescriptorProto, error) {
	if len(fieldNames) == 0 {
		// If no field names are specified, the expression node directly yields
		// the sources's message type.
		return &descriptorpb.FieldDescriptorProto{
			Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
			TypeName: proto.String("." + string(msgFullName)),
		}, nil
	}

	for i, fieldName := range fieldNames {
		descProto := er.getDescriptorProto(msgFullName)
		if descProto == nil {
			return nil, fmt.Errorf("message type %q not found", msgFullName)
		}

		var fieldDesc *descriptorpb.FieldDescriptorProto
		for _, fd := range descProto.GetField() {
			if fd.GetName() == fieldName {
				fieldDesc = fd
				break
			}
		}
		if fieldDesc == nil {
			return nil, fmt.Errorf("field %q not found in %q", fieldName, msgFullName)
		}
		fieldDescTypeFullName := protoreflect.FullName(strings.TrimPrefix(fieldDesc.GetTypeName(), "."))

		if i == len(fieldNames)-1 {
			var typeLabel *descriptorpb.FieldDescriptorProto_Label
			if fieldDesc.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
				typeLabel = descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum()
			}
			return &descriptorpb.FieldDescriptorProto{
				Type:     fieldDesc.Type,
				TypeName: fieldDesc.TypeName,
				Label:    typeLabel,
			}, nil
		}

		if fieldDesc.GetType() != descriptorpb.FieldDescriptorProto_TYPE_MESSAGE {
			return nil, fmt.Errorf("non-message type %q cannot have nested type %q", msgFullName, fieldNames[i+1])
		}
		if fieldDesc.GetLabel() == descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
			return nil, fmt.Errorf("cannot access field %q of repeated-message-type field", fieldNames[i+1])
		}
		msgFullName = fieldDescTypeFullName
	}
	panic(fmt.Sprintf("unreachable: field traversal reached an unexpected state for message %q", msgFullName))
}

func (er *ExpressionResolver) resolveCombinationNode(expNode *pb.CombinationNode) (*descriptorpb.FieldDescriptorProto, error) {
	fieldDesc := &descriptorpb.FieldDescriptorProto{}

	switch expNode.WhichOperator() {
	case pb.CombinationNode_LogicalOperator_case,
		pb.CombinationNode_RelationalOperator_case:
		fieldDesc.Type = descriptorpb.FieldDescriptorProto_TYPE_BOOL.Enum()
	case pb.CombinationNode_RoundingOperator_case:
		fieldDesc.Type = descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()
	case pb.CombinationNode_ArithmeticOperator_case:
		return er.resolveArithExpressions(expNode)
	case pb.CombinationNode_ListOperator_case:
		return er.resolveListExpressions(expNode)
	default:
		return nil, fmt.Errorf("Unknown Combination Node Operator %q", expNode.WhichOperator().String())
	}
	return fieldDesc, nil
}

func (er *ExpressionResolver) resolveArithExpressions(expNode *pb.CombinationNode) (*descriptorpb.FieldDescriptorProto, error) {
	fieldDesc := &descriptorpb.FieldDescriptorProto{}
	op := expNode.GetArithmeticOperator()
	switch op {
	case *pb.CombinationNode_ADD.Enum(),
		*pb.CombinationNode_SUBTRACT.Enum(),
		*pb.CombinationNode_MULTIPLY.Enum(),
		*pb.CombinationNode_POWER.Enum():
		leftFieldDesc, err := er.Resolve(expNode.GetLeftIndex())
		if err != nil {
			return nil, err
		}
		rightFieldDesc, err := er.Resolve(expNode.GetRightIndex())
		if err != nil {
			return nil, err
		}
		if isInt(leftFieldDesc.Type) && isInt(rightFieldDesc.Type) &&
			op != *pb.CombinationNode_POWER.Enum() {
			// Power operator can yield double values even if both operands are integers: 3^(-2)
			fieldDesc.Type = descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()
			return fieldDesc, nil
		} else if isNumeric(leftFieldDesc.Type) && isNumeric(rightFieldDesc.Type) {
			fieldDesc.Type = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()
			return fieldDesc, nil
		} else {
			return nil, fmt.Errorf("Could not resolve Type from %q and %q", leftFieldDesc.Type, rightFieldDesc.Type)
		}
	case *pb.CombinationNode_DIVIDE.Enum():
		fieldDesc.Type = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()
		return fieldDesc, nil
	case *pb.CombinationNode_MODULO_TRUNC.Enum():
		fieldDesc.Type = descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()
		return fieldDesc, nil
	case *pb.CombinationNode_ABSOLUTE.Enum(),
		*pb.CombinationNode_UNARY_MINUS.Enum():
		leftFieldDesc, err := er.Resolve(expNode.GetLeftIndex())
		if err != nil {
			return nil, err
		}
		if isInt(leftFieldDesc.Type) {
			fieldDesc.Type = descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()
			return fieldDesc, nil
		} else if isNumeric(leftFieldDesc.Type) {
			fieldDesc.Type = descriptorpb.FieldDescriptorProto_TYPE_DOUBLE.Enum()
			return fieldDesc, nil
		} else {
			return nil, fmt.Errorf("%q is not numeric", leftFieldDesc.Type)
		}
	default:
		return nil, fmt.Errorf("Unknown ArithmeticOperator %q", expNode.GetArithmeticOperator())
	}
}

func (er *ExpressionResolver) resolveListExpressions(expNode *pb.CombinationNode) (*descriptorpb.FieldDescriptorProto, error) {
	fieldDesc := &descriptorpb.FieldDescriptorProto{}
	op := expNode.GetListOperator()
	switch op {
	case pb.CombinationNode_LENGTH:
		leftFieldDesc, err := er.Resolve(expNode.GetLeftIndex())
		if err != nil {
			return nil, err
		}
		if leftFieldDesc.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
			return nil, fmt.Errorf("length operator can only be applied to repeated fields, got %v", leftFieldDesc.GetLabel())
		}
		fieldDesc.Type = descriptorpb.FieldDescriptorProto_TYPE_INT64.Enum()
		return fieldDesc, nil

	case pb.CombinationNode_SUBSCRIPT:
		leftFieldDesc, err := er.Resolve(expNode.GetLeftIndex())
		if err != nil {
			return nil, err
		}
		if leftFieldDesc.GetLabel() != descriptorpb.FieldDescriptorProto_LABEL_REPEATED {
			return nil, fmt.Errorf("subscript operator can only be applied to repeated fields, got %v", leftFieldDesc.GetLabel())
		}
		rightFieldDesc, err := er.Resolve(expNode.GetRightIndex())
		if err != nil {
			return nil, err
		}
		if !isInt(rightFieldDesc.Type) {
			return nil, fmt.Errorf("subscript index must be an integer, got %v", rightFieldDesc.Type)
		}
		fieldDesc.Type = leftFieldDesc.Type
		fieldDesc.TypeName = leftFieldDesc.TypeName
		fieldDesc.Label = nil
		return fieldDesc, nil

	default:
		return nil, fmt.Errorf("Unknown ListOperator %q", op)
	}
}
