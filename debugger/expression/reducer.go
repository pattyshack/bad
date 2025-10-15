package expression

import (
	"fmt"
	"math"
	"strconv"

	"github.com/pattyshack/gt/parseutil"
)

type reducerImpl struct {
	EvaluationContext
}

func newReducer(ctx EvaluationContext) Reducer {
	return &reducerImpl{
		EvaluationContext: ctx,
	}
}

func (reducer *reducerImpl) TrueToLiteralExpr(
	true_ *TokenValue,
) (
	*TypedData,
	error,
) {
	return reducer.DescriptorPool().NewBool(true), nil
}

func (reducer *reducerImpl) FalseToLiteralExpr(
	false_ *TokenValue,
) (
	*TypedData,
	error,
) {
	return reducer.DescriptorPool().NewBool(false), nil
}

func (reducer *reducerImpl) IntegerLiteralToLiteralExpr(
	integerLiteral *TokenValue,
) (
	*TypedData,
	error,
) {
	value, err := strconv.ParseInt(integerLiteral.Value, 0, 64)

	if err != nil {
		return nil, fmt.Errorf(
			"cannot parse int literal (%s): %w",
			integerLiteral.Value,
			err)
	}

	// Default to int32 whenever possible since it's the more common size
	if math.MinInt32 <= value && value <= math.MaxInt32 {
		return reducer.DescriptorPool().NewInt32(
			integerLiteral.Value,
			int32(value)), nil
	}

	return reducer.DescriptorPool().NewInt64(integerLiteral.Value, value), nil
}

func (reducer *reducerImpl) FloatLiteralToLiteralExpr(
	floatLiteral *TokenValue,
) (
	*TypedData,
	error,
) {
	value, err := strconv.ParseFloat(floatLiteral.Value, 64)

	if err != nil {
		return nil, fmt.Errorf(
			"cannot parse float literal (%s): %w",
			floatLiteral.Value,
			err)
	}

	return reducer.DescriptorPool().NewFloat64(floatLiteral.Value, value), nil
}

func (reducer *reducerImpl) RuneLiteralToLiteralExpr(
	charLiteral *TokenValue,
) (
	*TypedData,
	error,
) {
	char := parseutil.Unescape(charLiteral.Value[1 : len(charLiteral.Value)-1])
	data := []byte(char)
	if len(data) != 1 {
		return nil, fmt.Errorf("non-ascii utf8 rune literal not supported")
	}

	return reducer.DescriptorPool().NewChar(charLiteral.Value, data[0]), nil
}

func (reducer *reducerImpl) StringLiteralToLiteralExpr(
	stringLiteral *TokenValue,
) (
	*TypedData,
	error,
) {
	return reducer.DescriptorPool().NewCString(
		reducer,
		stringLiteral.Value,
		parseutil.Unescape(stringLiteral.Value[1:len(stringLiteral.Value)-1]))
}

func (reducer *reducerImpl) ToNamedExpr(name *TokenValue) (*TypedData, error) {
	return reducer.ReadInspectFrameVariableOrFunction(name.Value)
}

func (reducer *reducerImpl) ToPreviousResultExpr(
	dollarInteger *TokenValue,
) (
	*TypedData,
	error,
) {
	idx, err := strconv.ParseInt(dollarInteger.Value[1:], 0, 32)
	if err != nil {
		return nil, fmt.Errorf(
			"cannot parse previous result idx (%s): %w",
			dollarInteger.Value,
			err)
	}

	result, err := reducer.GetEvaluatedResult(int(idx))
	if err != nil {
		return nil, err
	}

	return result.TypedData, nil
}

func (reducerImpl) ToGroupedExpr(
	lparen *TokenValue,
	expr *TypedData,
	rparen *TokenValue,
) (
	*TypedData,
	error,
) {
	return expr, nil
}

func (reducerImpl) ToDirectAccessExpr(
	accessible *TypedData,
	dot *TokenValue,
	name *TokenValue,
) (
	*TypedData,
	error,
) {
	return accessible.FieldOrMethodByName(name.Value)
}

func (reducerImpl) ToIndirectAccessExpr(
	accessible *TypedData,
	arrow *TokenValue,
	name *TokenValue,
) (
	*TypedData,
	error,
) {
	deref, err := accessible.Dereference()
	if err != nil {
		return nil, err
	}

	return deref.FieldOrMethodByName(name.Value)
}

func (reducerImpl) ToIndexExpr(
	accessible *TypedData,
	lbracket *TokenValue,
	idxExpr *TypedData,
	rbracket *TokenValue,
) (
	*TypedData,
	error,
) {
	if idxExpr.Kind != IntKind || idxExpr.ByteSize != 4 {
		return nil, fmt.Errorf(
			"invalid index value type (%s). expected int32",
			idxExpr.TypeName())
	}

	value, err := idxExpr.DecodeSimpleValue()
	if err != nil {
		return nil, err
	}

	return accessible.Index(int(value.(int32)))
}

func (reducer *reducerImpl) ToCallExpr(
	accessible *TypedData,
	lparen *TokenValue,
	arguments []*TypedData,
	rparen *TokenValue,
) (
	*TypedData,
	error,
) {
	return reducer.InvokeInCurrentThread(accessible, arguments)
}

func (reducerImpl) EmptyListToArguments() ([]*TypedData, error) {
	return nil, nil
}

func (reducerImpl) ImproperListToArguments(
	arguments []*TypedData,
	comma *TokenValue,
) (
	[]*TypedData,
	error,
) {
	return arguments, nil
}

func (reducerImpl) NewToNonEmptyArguments(
	expr *TypedData,
) (
	[]*TypedData,
	error,
) {
	return []*TypedData{expr}, nil
}

func (reducerImpl) AppendToNonEmptyArguments(
	arguments []*TypedData,
	comma *TokenValue,
	expr *TypedData,
) (
	[]*TypedData,
	error,
) {
	return append(arguments, expr), nil
}
