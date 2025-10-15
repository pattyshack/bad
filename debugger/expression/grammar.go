// Auto-generated from source: grammar.lr

package expression

import (
	fmt "fmt"
	parseutil "github.com/pattyshack/gt/parseutil"
	io "io"
)

type SymbolId int

const (
	IntegerLiteralToken = SymbolId(256)
	FloatLiteralToken   = SymbolId(257)
	RuneLiteralToken    = SymbolId(258)
	StringLiteralToken  = SymbolId(259)
	TrueToken           = SymbolId(260)
	FalseToken          = SymbolId(261)
	IdentifierToken     = SymbolId(262)
	DollarIntegerToken  = SymbolId(263)
	DotToken            = SymbolId(264)
	CommaToken          = SymbolId(265)
	ArrowToken          = SymbolId(266)
	LparenToken         = SymbolId(267)
	RparenToken         = SymbolId(268)
	LbracketToken       = SymbolId(269)
	RbracketToken       = SymbolId(270)
)

type LiteralExprReducer interface {
	// 26:2: literal_expr -> TRUE: ...
	TrueToLiteralExpr(True_ *TokenValue) (*TypedData, error)

	// 27:2: literal_expr -> FALSE: ...
	FalseToLiteralExpr(False_ *TokenValue) (*TypedData, error)

	// 28:2: literal_expr -> INTEGER_LITERAL: ...
	IntegerLiteralToLiteralExpr(IntegerLiteral_ *TokenValue) (*TypedData, error)

	// 29:2: literal_expr -> FLOAT_LITERAL: ...
	FloatLiteralToLiteralExpr(FloatLiteral_ *TokenValue) (*TypedData, error)

	// 30:2: literal_expr -> RUNE_LITERAL: ...
	RuneLiteralToLiteralExpr(RuneLiteral_ *TokenValue) (*TypedData, error)

	// 31:2: literal_expr -> STRING_LITERAL: ...
	StringLiteralToLiteralExpr(StringLiteral_ *TokenValue) (*TypedData, error)
}

type NamedExprReducer interface {
	// 33:21: named_expr -> ...
	ToNamedExpr(Identifier_ *TokenValue) (*TypedData, error)
}

type PreviousResultExprReducer interface {
	// 35:31: previous_result_expr -> ...
	ToPreviousResultExpr(DollarInteger_ *TokenValue) (*TypedData, error)
}

type GroupedExprReducer interface {
	// 37:23: grouped_expr -> ...
	ToGroupedExpr(Lparen_ *TokenValue, Expression_ *TypedData, Rparen_ *TokenValue) (*TypedData, error)
}

type DirectAccessExprReducer interface {
	// 39:29: direct_access_expr -> ...
	ToDirectAccessExpr(AccessibleExpr_ *TypedData, Dot_ *TokenValue, Identifier_ *TokenValue) (*TypedData, error)
}

type IndirectAccessExprReducer interface {
	// 41:31: indirect_access_expr -> ...
	ToIndirectAccessExpr(AccessibleExpr_ *TypedData, Arrow_ *TokenValue, Identifier_ *TokenValue) (*TypedData, error)
}

type IndexExprReducer interface {
	// 43:21: index_expr -> ...
	ToIndexExpr(AccessibleExpr_ *TypedData, Lbracket_ *TokenValue, Expression_ *TypedData, Rbracket_ *TokenValue) (*TypedData, error)
}

type CallExprReducer interface {
	// 45:20: call_expr -> ...
	ToCallExpr(AccessibleExpr_ *TypedData, Lparen_ *TokenValue, Arguments_ []*TypedData, Rparen_ *TokenValue) (*TypedData, error)
}

type ArgumentsReducer interface {
	// 48:2: arguments -> empty_list: ...
	EmptyListToArguments() ([]*TypedData, error)

	// 49:2: arguments -> improper_list: ...
	ImproperListToArguments(NonEmptyArguments_ []*TypedData, Comma_ *TokenValue) ([]*TypedData, error)
}

type NonEmptyArgumentsReducer interface {
	// 53:2: non_empty_arguments -> new: ...
	NewToNonEmptyArguments(Expression_ *TypedData) ([]*TypedData, error)

	// 54:2: non_empty_arguments -> append: ...
	AppendToNonEmptyArguments(NonEmptyArguments_ []*TypedData, Comma_ *TokenValue, Expression_ *TypedData) ([]*TypedData, error)
}

type Reducer interface {
	LiteralExprReducer
	NamedExprReducer
	PreviousResultExprReducer
	GroupedExprReducer
	DirectAccessExprReducer
	IndirectAccessExprReducer
	IndexExprReducer
	CallExprReducer
	ArgumentsReducer
	NonEmptyArgumentsReducer
}

type ParseErrorHandler interface {
	Error(nextToken parseutil.Token[SymbolId], parseStack _Stack) error
}

type DefaultParseErrorHandler struct{}

func (DefaultParseErrorHandler) Error(nextToken parseutil.Token[SymbolId], stack _Stack) error {
	return parseutil.NewLocationError(
		nextToken.Loc(),
		"syntax error: unexpected symbol %s. expecting %v",
		nextToken.Id(),
		ExpectedTerminals(stack[len(stack)-1].StateId))
}

func ExpectedTerminals(id _StateId) []SymbolId {
	switch id {
	case _State1:
		return []SymbolId{IntegerLiteralToken, FloatLiteralToken, RuneLiteralToken, StringLiteralToken, TrueToken, FalseToken, IdentifierToken, DollarIntegerToken, LparenToken}
	case _State2:
		return []SymbolId{_EndMarker}
	case _State3:
		return []SymbolId{IntegerLiteralToken, FloatLiteralToken, RuneLiteralToken, StringLiteralToken, TrueToken, FalseToken, IdentifierToken, DollarIntegerToken, LparenToken}
	case _State5:
		return []SymbolId{RparenToken}
	case _State6:
		return []SymbolId{IdentifierToken}
	case _State7:
		return []SymbolId{IdentifierToken}
	case _State8:
		return []SymbolId{IntegerLiteralToken, FloatLiteralToken, RuneLiteralToken, StringLiteralToken, TrueToken, FalseToken, IdentifierToken, DollarIntegerToken, LparenToken}
	case _State10:
		return []SymbolId{RbracketToken}
	case _State11:
		return []SymbolId{RparenToken}
	}

	return nil
}

func Parse(lexer parseutil.Lexer[parseutil.Token[SymbolId]], reducer Reducer) (*TypedData, error) {

	return ParseWithCustomErrorHandler(
		lexer,
		reducer,
		DefaultParseErrorHandler{})
}

func ParseWithCustomErrorHandler(
	lexer parseutil.Lexer[parseutil.Token[SymbolId]],
	reducer Reducer,
	errHandler ParseErrorHandler,
) (
	*TypedData,
	error,
) {
	item, err := _Parse(lexer, reducer, errHandler, _State1)
	if err != nil {
		var errRetVal *TypedData
		return errRetVal, err
	}
	return item.Value, nil
}

// ================================================================
// Parser internal implementation
// User should normally avoid directly accessing the following code
// ================================================================

func _Parse(
	lexer parseutil.Lexer[parseutil.Token[SymbolId]],
	reducer Reducer,
	errHandler ParseErrorHandler,
	startState _StateId,
) (
	*_StackItem,
	error,
) {
	stateStack := _Stack{
		// Note: we don't have to populate the start symbol since its value
		// is never accessed.
		&_StackItem{startState, nil},
	}

	symbolStack := &_PseudoSymbolStack{lexer: lexer}

	for {
		nextSymbol, err := symbolStack.Top()
		if err != nil {
			return nil, err
		}

		action, ok := _ActionTable.Get(
			stateStack[len(stateStack)-1].StateId,
			nextSymbol.Id())
		if !ok {
			return nil, errHandler.Error(nextSymbol, stateStack)
		}

		if action.ActionType == _ShiftAction {
			stateStack = append(stateStack, action.ShiftItem(nextSymbol))

			_, err = symbolStack.Pop()
			if err != nil {
				return nil, err
			}
		} else if action.ActionType == _ReduceAction {
			var reduceSymbol *Symbol
			stateStack, reduceSymbol, err = action.ReduceSymbol(
				reducer,
				stateStack)
			if err != nil {
				return nil, err
			}

			symbolStack.Push(reduceSymbol)
		} else if action.ActionType == _ShiftAndReduceAction {
			stateStack = append(stateStack, action.ShiftItem(nextSymbol))

			_, err = symbolStack.Pop()
			if err != nil {
				return nil, err
			}

			var reduceSymbol *Symbol
			stateStack, reduceSymbol, err = action.ReduceSymbol(
				reducer,
				stateStack)
			if err != nil {
				return nil, err
			}

			symbolStack.Push(reduceSymbol)
		} else if action.ActionType == _AcceptAction {
			if len(stateStack) != 2 {
				panic("This should never happen")
			}
			return stateStack[1], nil
		} else {
			panic("Unknown action type: " + action.ActionType.String())
		}
	}
}

func (i SymbolId) String() string {
	switch i {
	case _EndMarker:
		return "$"
	case _WildcardMarker:
		return "*"
	case IntegerLiteralToken:
		return "INTEGER_LITERAL"
	case FloatLiteralToken:
		return "FLOAT_LITERAL"
	case RuneLiteralToken:
		return "RUNE_LITERAL"
	case StringLiteralToken:
		return "STRING_LITERAL"
	case TrueToken:
		return "TRUE"
	case FalseToken:
		return "FALSE"
	case IdentifierToken:
		return "IDENTIFIER"
	case DollarIntegerToken:
		return "DOLLAR_INTEGER"
	case DotToken:
		return "DOT"
	case CommaToken:
		return "COMMA"
	case ArrowToken:
		return "ARROW"
	case LparenToken:
		return "LPAREN"
	case RparenToken:
		return "RPAREN"
	case LbracketToken:
		return "LBRACKET"
	case RbracketToken:
		return "RBRACKET"
	case ExpressionType:
		return "expression"
	case AccessibleExprType:
		return "accessible_expr"
	case AtomExprType:
		return "atom_expr"
	case LiteralExprType:
		return "literal_expr"
	case NamedExprType:
		return "named_expr"
	case PreviousResultExprType:
		return "previous_result_expr"
	case GroupedExprType:
		return "grouped_expr"
	case DirectAccessExprType:
		return "direct_access_expr"
	case IndirectAccessExprType:
		return "indirect_access_expr"
	case IndexExprType:
		return "index_expr"
	case CallExprType:
		return "call_expr"
	case ArgumentsType:
		return "arguments"
	case NonEmptyArgumentsType:
		return "non_empty_arguments"
	default:
		return fmt.Sprintf("?unknown symbol %d?", int(i))
	}
}

const (
	_EndMarker      = SymbolId(0)
	_WildcardMarker = SymbolId(-1)

	ExpressionType         = SymbolId(271)
	AccessibleExprType     = SymbolId(272)
	AtomExprType           = SymbolId(273)
	LiteralExprType        = SymbolId(274)
	NamedExprType          = SymbolId(275)
	PreviousResultExprType = SymbolId(276)
	GroupedExprType        = SymbolId(277)
	DirectAccessExprType   = SymbolId(278)
	IndirectAccessExprType = SymbolId(279)
	IndexExprType          = SymbolId(280)
	CallExprType           = SymbolId(281)
	ArgumentsType          = SymbolId(282)
	NonEmptyArgumentsType  = SymbolId(283)
)

type _ActionType int

const (
	// NOTE: error action is implicit
	_ShiftAction          = _ActionType(0)
	_ReduceAction         = _ActionType(1)
	_ShiftAndReduceAction = _ActionType(2)
	_AcceptAction         = _ActionType(3)
)

func (i _ActionType) String() string {
	switch i {
	case _ShiftAction:
		return "shift"
	case _ReduceAction:
		return "reduce"
	case _ShiftAndReduceAction:
		return "shift-and-reduce"
	case _AcceptAction:
		return "accept"
	default:
		return fmt.Sprintf("?Unknown action %d?", int(i))
	}
}

type _ReduceType int

const (
	_ReduceAccessibleExprToExpression         = _ReduceType(1)
	_ReduceAtomExprToAccessibleExpr           = _ReduceType(2)
	_ReduceDirectAccessExprToAccessibleExpr   = _ReduceType(3)
	_ReduceIndirectAccessExprToAccessibleExpr = _ReduceType(4)
	_ReduceIndexExprToAccessibleExpr          = _ReduceType(5)
	_ReduceCallExprToAccessibleExpr           = _ReduceType(6)
	_ReduceLiteralExprToAtomExpr              = _ReduceType(7)
	_ReduceNamedExprToAtomExpr                = _ReduceType(8)
	_ReducePreviousResultExprToAtomExpr       = _ReduceType(9)
	_ReduceGroupedExprToAtomExpr              = _ReduceType(10)
	_ReduceTrueToLiteralExpr                  = _ReduceType(11)
	_ReduceFalseToLiteralExpr                 = _ReduceType(12)
	_ReduceIntegerLiteralToLiteralExpr        = _ReduceType(13)
	_ReduceFloatLiteralToLiteralExpr          = _ReduceType(14)
	_ReduceRuneLiteralToLiteralExpr           = _ReduceType(15)
	_ReduceStringLiteralToLiteralExpr         = _ReduceType(16)
	_ReduceToNamedExpr                        = _ReduceType(17)
	_ReduceToPreviousResultExpr               = _ReduceType(18)
	_ReduceToGroupedExpr                      = _ReduceType(19)
	_ReduceToDirectAccessExpr                 = _ReduceType(20)
	_ReduceToIndirectAccessExpr               = _ReduceType(21)
	_ReduceToIndexExpr                        = _ReduceType(22)
	_ReduceToCallExpr                         = _ReduceType(23)
	_ReduceEmptyListToArguments               = _ReduceType(24)
	_ReduceImproperListToArguments            = _ReduceType(25)
	_ReduceNonEmptyArgumentsToArguments       = _ReduceType(26)
	_ReduceNewToNonEmptyArguments             = _ReduceType(27)
	_ReduceAppendToNonEmptyArguments          = _ReduceType(28)
)

func (i _ReduceType) String() string {
	switch i {
	case _ReduceAccessibleExprToExpression:
		return "AccessibleExprToExpression"
	case _ReduceAtomExprToAccessibleExpr:
		return "AtomExprToAccessibleExpr"
	case _ReduceDirectAccessExprToAccessibleExpr:
		return "DirectAccessExprToAccessibleExpr"
	case _ReduceIndirectAccessExprToAccessibleExpr:
		return "IndirectAccessExprToAccessibleExpr"
	case _ReduceIndexExprToAccessibleExpr:
		return "IndexExprToAccessibleExpr"
	case _ReduceCallExprToAccessibleExpr:
		return "CallExprToAccessibleExpr"
	case _ReduceLiteralExprToAtomExpr:
		return "LiteralExprToAtomExpr"
	case _ReduceNamedExprToAtomExpr:
		return "NamedExprToAtomExpr"
	case _ReducePreviousResultExprToAtomExpr:
		return "PreviousResultExprToAtomExpr"
	case _ReduceGroupedExprToAtomExpr:
		return "GroupedExprToAtomExpr"
	case _ReduceTrueToLiteralExpr:
		return "TrueToLiteralExpr"
	case _ReduceFalseToLiteralExpr:
		return "FalseToLiteralExpr"
	case _ReduceIntegerLiteralToLiteralExpr:
		return "IntegerLiteralToLiteralExpr"
	case _ReduceFloatLiteralToLiteralExpr:
		return "FloatLiteralToLiteralExpr"
	case _ReduceRuneLiteralToLiteralExpr:
		return "RuneLiteralToLiteralExpr"
	case _ReduceStringLiteralToLiteralExpr:
		return "StringLiteralToLiteralExpr"
	case _ReduceToNamedExpr:
		return "ToNamedExpr"
	case _ReduceToPreviousResultExpr:
		return "ToPreviousResultExpr"
	case _ReduceToGroupedExpr:
		return "ToGroupedExpr"
	case _ReduceToDirectAccessExpr:
		return "ToDirectAccessExpr"
	case _ReduceToIndirectAccessExpr:
		return "ToIndirectAccessExpr"
	case _ReduceToIndexExpr:
		return "ToIndexExpr"
	case _ReduceToCallExpr:
		return "ToCallExpr"
	case _ReduceEmptyListToArguments:
		return "EmptyListToArguments"
	case _ReduceImproperListToArguments:
		return "ImproperListToArguments"
	case _ReduceNonEmptyArgumentsToArguments:
		return "NonEmptyArgumentsToArguments"
	case _ReduceNewToNonEmptyArguments:
		return "NewToNonEmptyArguments"
	case _ReduceAppendToNonEmptyArguments:
		return "AppendToNonEmptyArguments"
	default:
		return fmt.Sprintf("?unknown reduce type %d?", int(i))
	}
}

type _StateId int

func (id _StateId) String() string {
	return fmt.Sprintf("State %d", int(id))
}

const (
	_State1  = _StateId(1)
	_State2  = _StateId(2)
	_State3  = _StateId(3)
	_State4  = _StateId(4)
	_State5  = _StateId(5)
	_State6  = _StateId(6)
	_State7  = _StateId(7)
	_State8  = _StateId(8)
	_State9  = _StateId(9)
	_State10 = _StateId(10)
	_State11 = _StateId(11)
	_State12 = _StateId(12)
	_State13 = _StateId(13)
)

type Symbol struct {
	SymbolId_ SymbolId

	Generic_ parseutil.TokenValue[SymbolId]

	Token  *TokenValue
	Value  *TypedData
	Values []*TypedData
}

func NewSymbol(token parseutil.Token[SymbolId]) (*Symbol, error) {
	symbol, ok := token.(*Symbol)
	if ok {
		return symbol, nil
	}

	symbol = &Symbol{SymbolId_: token.Id()}
	switch token.Id() {
	case _EndMarker:
		val, ok := token.(parseutil.TokenValue[SymbolId])
		if !ok {
			return nil, parseutil.NewLocationError(
				token.Loc(),
				"invalid value type for token %s. "+
					"expecting parseutil.TokenValue[SymbolId]",
				token.Id())
		}
		symbol.Generic_ = val
	case IntegerLiteralToken, FloatLiteralToken, RuneLiteralToken, StringLiteralToken, TrueToken, FalseToken, IdentifierToken, DollarIntegerToken, DotToken, CommaToken, ArrowToken, LparenToken, RparenToken, LbracketToken, RbracketToken:
		val, ok := token.(*TokenValue)
		if !ok {
			return nil, parseutil.NewLocationError(
				token.Loc(),
				"invalid value type for token %s. "+
					"expecting *TokenValue",
				token.Id())
		}
		symbol.Token = val
	default:
		return nil, parseutil.NewLocationError(
			token.Loc(),
			"unexpected token type: %s",
			token.Id())
	}
	return symbol, nil
}

func (s *Symbol) Id() SymbolId {
	return s.SymbolId_
}

func (s *Symbol) StartEnd() parseutil.StartEndPos {
	type locator interface{ StartEnd() parseutil.StartEndPos }
	switch s.SymbolId_ {
	case IntegerLiteralToken, FloatLiteralToken, RuneLiteralToken, StringLiteralToken, TrueToken, FalseToken, IdentifierToken, DollarIntegerToken, DotToken, CommaToken, ArrowToken, LparenToken, RparenToken, LbracketToken, RbracketToken:
		loc, ok := interface{}(s.Token).(locator)
		if ok {
			return loc.StartEnd()
		}
	case ExpressionType, AccessibleExprType, AtomExprType, LiteralExprType, NamedExprType, PreviousResultExprType, GroupedExprType, DirectAccessExprType, IndirectAccessExprType, IndexExprType, CallExprType:
		loc, ok := interface{}(s.Value).(locator)
		if ok {
			return loc.StartEnd()
		}
	case ArgumentsType, NonEmptyArgumentsType:
		loc, ok := interface{}(s.Values).(locator)
		if ok {
			return loc.StartEnd()
		}
	}
	return s.Generic_.StartEnd()
}

func (s *Symbol) Loc() parseutil.Location {
	type locator interface{ Loc() parseutil.Location }
	switch s.SymbolId_ {
	case IntegerLiteralToken, FloatLiteralToken, RuneLiteralToken, StringLiteralToken, TrueToken, FalseToken, IdentifierToken, DollarIntegerToken, DotToken, CommaToken, ArrowToken, LparenToken, RparenToken, LbracketToken, RbracketToken:
		loc, ok := interface{}(s.Token).(locator)
		if ok {
			return loc.Loc()
		}
	case ExpressionType, AccessibleExprType, AtomExprType, LiteralExprType, NamedExprType, PreviousResultExprType, GroupedExprType, DirectAccessExprType, IndirectAccessExprType, IndexExprType, CallExprType:
		loc, ok := interface{}(s.Value).(locator)
		if ok {
			return loc.Loc()
		}
	case ArgumentsType, NonEmptyArgumentsType:
		loc, ok := interface{}(s.Values).(locator)
		if ok {
			return loc.Loc()
		}
	}
	return s.Generic_.Loc()
}

func (s *Symbol) End() parseutil.Location {
	type locator interface{ End() parseutil.Location }
	switch s.SymbolId_ {
	case IntegerLiteralToken, FloatLiteralToken, RuneLiteralToken, StringLiteralToken, TrueToken, FalseToken, IdentifierToken, DollarIntegerToken, DotToken, CommaToken, ArrowToken, LparenToken, RparenToken, LbracketToken, RbracketToken:
		loc, ok := interface{}(s.Token).(locator)
		if ok {
			return loc.End()
		}
	case ExpressionType, AccessibleExprType, AtomExprType, LiteralExprType, NamedExprType, PreviousResultExprType, GroupedExprType, DirectAccessExprType, IndirectAccessExprType, IndexExprType, CallExprType:
		loc, ok := interface{}(s.Value).(locator)
		if ok {
			return loc.End()
		}
	case ArgumentsType, NonEmptyArgumentsType:
		loc, ok := interface{}(s.Values).(locator)
		if ok {
			return loc.End()
		}
	}
	return s.Generic_.End()
}

type _PseudoSymbolStack struct {
	lexer parseutil.Lexer[parseutil.Token[SymbolId]]
	top   []*Symbol
}

func (stack *_PseudoSymbolStack) Top() (*Symbol, error) {
	if len(stack.top) == 0 {
		token, err := stack.lexer.Next()
		if err != nil {
			if err != io.EOF {
				return nil, parseutil.NewLocationError(
					stack.lexer.CurrentLocation(),
					"unexpected lex error: %w",
					err)
			}
			token = parseutil.TokenValue[SymbolId]{
				SymbolId: _EndMarker,
				StartEndPos: parseutil.StartEndPos{
					StartPos: stack.lexer.CurrentLocation(),
					EndPos:   stack.lexer.CurrentLocation(),
				},
			}
		}
		item, err := NewSymbol(token)
		if err != nil {
			return nil, err
		}
		stack.top = append(stack.top, item)
	}
	return stack.top[len(stack.top)-1], nil
}

func (stack *_PseudoSymbolStack) Push(symbol *Symbol) {
	stack.top = append(stack.top, symbol)
}

func (stack *_PseudoSymbolStack) Pop() (*Symbol, error) {
	if len(stack.top) == 0 {
		return nil, fmt.Errorf("internal error: cannot pop an empty top")
	}
	ret := stack.top[len(stack.top)-1]
	stack.top = stack.top[:len(stack.top)-1]
	return ret, nil
}

type _StackItem struct {
	StateId _StateId

	*Symbol
}

type _Stack []*_StackItem

type _Action struct {
	ActionType _ActionType

	ShiftStateId _StateId
	ReduceType   _ReduceType
}

func (act *_Action) ShiftItem(symbol *Symbol) *_StackItem {
	return &_StackItem{StateId: act.ShiftStateId, Symbol: symbol}
}

func (act *_Action) ReduceSymbol(
	reducer Reducer,
	stack _Stack,
) (
	_Stack,
	*Symbol,
	error,
) {
	var err error
	symbol := &Symbol{}
	switch act.ReduceType {
	case _ReduceAccessibleExprToExpression:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = ExpressionType
		//line grammar.lr:10:4
		symbol.Value = args[0].Value
		err = nil
	case _ReduceAtomExprToAccessibleExpr:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = AccessibleExprType
		//line grammar.lr:13:4
		symbol.Value = args[0].Value
		err = nil
	case _ReduceDirectAccessExprToAccessibleExpr:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = AccessibleExprType
		//line grammar.lr:14:4
		symbol.Value = args[0].Value
		err = nil
	case _ReduceIndirectAccessExprToAccessibleExpr:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = AccessibleExprType
		//line grammar.lr:15:4
		symbol.Value = args[0].Value
		err = nil
	case _ReduceIndexExprToAccessibleExpr:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = AccessibleExprType
		//line grammar.lr:16:4
		symbol.Value = args[0].Value
		err = nil
	case _ReduceCallExprToAccessibleExpr:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = AccessibleExprType
		//line grammar.lr:17:4
		symbol.Value = args[0].Value
		err = nil
	case _ReduceLiteralExprToAtomExpr:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = AtomExprType
		//line grammar.lr:20:4
		symbol.Value = args[0].Value
		err = nil
	case _ReduceNamedExprToAtomExpr:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = AtomExprType
		//line grammar.lr:21:4
		symbol.Value = args[0].Value
		err = nil
	case _ReducePreviousResultExprToAtomExpr:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = AtomExprType
		//line grammar.lr:22:4
		symbol.Value = args[0].Value
		err = nil
	case _ReduceGroupedExprToAtomExpr:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = AtomExprType
		//line grammar.lr:23:4
		symbol.Value = args[0].Value
		err = nil
	case _ReduceTrueToLiteralExpr:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = LiteralExprType
		symbol.Value, err = reducer.TrueToLiteralExpr(args[0].Token)
	case _ReduceFalseToLiteralExpr:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = LiteralExprType
		symbol.Value, err = reducer.FalseToLiteralExpr(args[0].Token)
	case _ReduceIntegerLiteralToLiteralExpr:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = LiteralExprType
		symbol.Value, err = reducer.IntegerLiteralToLiteralExpr(args[0].Token)
	case _ReduceFloatLiteralToLiteralExpr:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = LiteralExprType
		symbol.Value, err = reducer.FloatLiteralToLiteralExpr(args[0].Token)
	case _ReduceRuneLiteralToLiteralExpr:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = LiteralExprType
		symbol.Value, err = reducer.RuneLiteralToLiteralExpr(args[0].Token)
	case _ReduceStringLiteralToLiteralExpr:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = LiteralExprType
		symbol.Value, err = reducer.StringLiteralToLiteralExpr(args[0].Token)
	case _ReduceToNamedExpr:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = NamedExprType
		symbol.Value, err = reducer.ToNamedExpr(args[0].Token)
	case _ReduceToPreviousResultExpr:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = PreviousResultExprType
		symbol.Value, err = reducer.ToPreviousResultExpr(args[0].Token)
	case _ReduceToGroupedExpr:
		args := stack[len(stack)-3:]
		stack = stack[:len(stack)-3]
		symbol.SymbolId_ = GroupedExprType
		symbol.Value, err = reducer.ToGroupedExpr(args[0].Token, args[1].Value, args[2].Token)
	case _ReduceToDirectAccessExpr:
		args := stack[len(stack)-3:]
		stack = stack[:len(stack)-3]
		symbol.SymbolId_ = DirectAccessExprType
		symbol.Value, err = reducer.ToDirectAccessExpr(args[0].Value, args[1].Token, args[2].Token)
	case _ReduceToIndirectAccessExpr:
		args := stack[len(stack)-3:]
		stack = stack[:len(stack)-3]
		symbol.SymbolId_ = IndirectAccessExprType
		symbol.Value, err = reducer.ToIndirectAccessExpr(args[0].Value, args[1].Token, args[2].Token)
	case _ReduceToIndexExpr:
		args := stack[len(stack)-4:]
		stack = stack[:len(stack)-4]
		symbol.SymbolId_ = IndexExprType
		symbol.Value, err = reducer.ToIndexExpr(args[0].Value, args[1].Token, args[2].Value, args[3].Token)
	case _ReduceToCallExpr:
		args := stack[len(stack)-4:]
		stack = stack[:len(stack)-4]
		symbol.SymbolId_ = CallExprType
		symbol.Value, err = reducer.ToCallExpr(args[0].Value, args[1].Token, args[2].Values, args[3].Token)
	case _ReduceEmptyListToArguments:
		symbol.SymbolId_ = ArgumentsType
		symbol.Values, err = reducer.EmptyListToArguments()
	case _ReduceImproperListToArguments:
		args := stack[len(stack)-2:]
		stack = stack[:len(stack)-2]
		symbol.SymbolId_ = ArgumentsType
		symbol.Values, err = reducer.ImproperListToArguments(args[0].Values, args[1].Token)
	case _ReduceNonEmptyArgumentsToArguments:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = ArgumentsType
		//line grammar.lr:50:4
		symbol.Values = args[0].Values
		err = nil
	case _ReduceNewToNonEmptyArguments:
		args := stack[len(stack)-1:]
		stack = stack[:len(stack)-1]
		symbol.SymbolId_ = NonEmptyArgumentsType
		symbol.Values, err = reducer.NewToNonEmptyArguments(args[0].Value)
	case _ReduceAppendToNonEmptyArguments:
		args := stack[len(stack)-3:]
		stack = stack[:len(stack)-3]
		symbol.SymbolId_ = NonEmptyArgumentsType
		symbol.Values, err = reducer.AppendToNonEmptyArguments(args[0].Values, args[1].Token, args[2].Value)
	default:
		panic("Unknown reduce type: " + act.ReduceType.String())
	}

	if err != nil {
		err = fmt.Errorf("unexpected %s reduce error: %w", act.ReduceType, err)
	}

	return stack, symbol, err
}

type _ActionTableKey struct {
	_StateId
	SymbolId
}

type _ActionTableType struct{}

func (_ActionTableType) Get(
	stateId _StateId,
	symbolId SymbolId,
) (
	_Action,
	bool,
) {
	switch stateId {
	case _State1:
		switch symbolId {
		case LparenToken:
			return _Action{_ShiftAction, _State3, 0}, true
		case ExpressionType:
			return _Action{_ShiftAction, _State2, 0}, true
		case AccessibleExprType:
			return _Action{_ShiftAction, _State4, 0}, true
		case IntegerLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceIntegerLiteralToLiteralExpr}, true
		case FloatLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceFloatLiteralToLiteralExpr}, true
		case RuneLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceRuneLiteralToLiteralExpr}, true
		case StringLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceStringLiteralToLiteralExpr}, true
		case TrueToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceTrueToLiteralExpr}, true
		case FalseToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceFalseToLiteralExpr}, true
		case IdentifierToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceToNamedExpr}, true
		case DollarIntegerToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceToPreviousResultExpr}, true
		case AtomExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceAtomExprToAccessibleExpr}, true
		case LiteralExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceLiteralExprToAtomExpr}, true
		case NamedExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceNamedExprToAtomExpr}, true
		case PreviousResultExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReducePreviousResultExprToAtomExpr}, true
		case GroupedExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceGroupedExprToAtomExpr}, true
		case DirectAccessExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceDirectAccessExprToAccessibleExpr}, true
		case IndirectAccessExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceIndirectAccessExprToAccessibleExpr}, true
		case IndexExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceIndexExprToAccessibleExpr}, true
		case CallExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceCallExprToAccessibleExpr}, true
		}
	case _State2:
		switch symbolId {
		case _EndMarker:
			return _Action{_AcceptAction, 0, 0}, true
		}
	case _State3:
		switch symbolId {
		case LparenToken:
			return _Action{_ShiftAction, _State3, 0}, true
		case ExpressionType:
			return _Action{_ShiftAction, _State5, 0}, true
		case AccessibleExprType:
			return _Action{_ShiftAction, _State4, 0}, true
		case IntegerLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceIntegerLiteralToLiteralExpr}, true
		case FloatLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceFloatLiteralToLiteralExpr}, true
		case RuneLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceRuneLiteralToLiteralExpr}, true
		case StringLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceStringLiteralToLiteralExpr}, true
		case TrueToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceTrueToLiteralExpr}, true
		case FalseToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceFalseToLiteralExpr}, true
		case IdentifierToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceToNamedExpr}, true
		case DollarIntegerToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceToPreviousResultExpr}, true
		case AtomExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceAtomExprToAccessibleExpr}, true
		case LiteralExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceLiteralExprToAtomExpr}, true
		case NamedExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceNamedExprToAtomExpr}, true
		case PreviousResultExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReducePreviousResultExprToAtomExpr}, true
		case GroupedExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceGroupedExprToAtomExpr}, true
		case DirectAccessExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceDirectAccessExprToAccessibleExpr}, true
		case IndirectAccessExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceIndirectAccessExprToAccessibleExpr}, true
		case IndexExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceIndexExprToAccessibleExpr}, true
		case CallExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceCallExprToAccessibleExpr}, true
		}
	case _State4:
		switch symbolId {
		case DotToken:
			return _Action{_ShiftAction, _State7, 0}, true
		case ArrowToken:
			return _Action{_ShiftAction, _State6, 0}, true
		case LparenToken:
			return _Action{_ShiftAction, _State9, 0}, true
		case LbracketToken:
			return _Action{_ShiftAction, _State8, 0}, true

		default:
			return _Action{_ReduceAction, 0, _ReduceAccessibleExprToExpression}, true
		}
	case _State5:
		switch symbolId {
		case RparenToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceToGroupedExpr}, true
		}
	case _State6:
		switch symbolId {
		case IdentifierToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceToIndirectAccessExpr}, true
		}
	case _State7:
		switch symbolId {
		case IdentifierToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceToDirectAccessExpr}, true
		}
	case _State8:
		switch symbolId {
		case LparenToken:
			return _Action{_ShiftAction, _State3, 0}, true
		case ExpressionType:
			return _Action{_ShiftAction, _State10, 0}, true
		case AccessibleExprType:
			return _Action{_ShiftAction, _State4, 0}, true
		case IntegerLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceIntegerLiteralToLiteralExpr}, true
		case FloatLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceFloatLiteralToLiteralExpr}, true
		case RuneLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceRuneLiteralToLiteralExpr}, true
		case StringLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceStringLiteralToLiteralExpr}, true
		case TrueToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceTrueToLiteralExpr}, true
		case FalseToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceFalseToLiteralExpr}, true
		case IdentifierToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceToNamedExpr}, true
		case DollarIntegerToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceToPreviousResultExpr}, true
		case AtomExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceAtomExprToAccessibleExpr}, true
		case LiteralExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceLiteralExprToAtomExpr}, true
		case NamedExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceNamedExprToAtomExpr}, true
		case PreviousResultExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReducePreviousResultExprToAtomExpr}, true
		case GroupedExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceGroupedExprToAtomExpr}, true
		case DirectAccessExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceDirectAccessExprToAccessibleExpr}, true
		case IndirectAccessExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceIndirectAccessExprToAccessibleExpr}, true
		case IndexExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceIndexExprToAccessibleExpr}, true
		case CallExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceCallExprToAccessibleExpr}, true
		}
	case _State9:
		switch symbolId {
		case LparenToken:
			return _Action{_ShiftAction, _State3, 0}, true
		case AccessibleExprType:
			return _Action{_ShiftAction, _State4, 0}, true
		case ArgumentsType:
			return _Action{_ShiftAction, _State11, 0}, true
		case NonEmptyArgumentsType:
			return _Action{_ShiftAction, _State12, 0}, true
		case IntegerLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceIntegerLiteralToLiteralExpr}, true
		case FloatLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceFloatLiteralToLiteralExpr}, true
		case RuneLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceRuneLiteralToLiteralExpr}, true
		case StringLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceStringLiteralToLiteralExpr}, true
		case TrueToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceTrueToLiteralExpr}, true
		case FalseToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceFalseToLiteralExpr}, true
		case IdentifierToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceToNamedExpr}, true
		case DollarIntegerToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceToPreviousResultExpr}, true
		case ExpressionType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceNewToNonEmptyArguments}, true
		case AtomExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceAtomExprToAccessibleExpr}, true
		case LiteralExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceLiteralExprToAtomExpr}, true
		case NamedExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceNamedExprToAtomExpr}, true
		case PreviousResultExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReducePreviousResultExprToAtomExpr}, true
		case GroupedExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceGroupedExprToAtomExpr}, true
		case DirectAccessExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceDirectAccessExprToAccessibleExpr}, true
		case IndirectAccessExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceIndirectAccessExprToAccessibleExpr}, true
		case IndexExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceIndexExprToAccessibleExpr}, true
		case CallExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceCallExprToAccessibleExpr}, true

		default:
			return _Action{_ReduceAction, 0, _ReduceEmptyListToArguments}, true
		}
	case _State10:
		switch symbolId {
		case RbracketToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceToIndexExpr}, true
		}
	case _State11:
		switch symbolId {
		case RparenToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceToCallExpr}, true
		}
	case _State12:
		switch symbolId {
		case CommaToken:
			return _Action{_ShiftAction, _State13, 0}, true

		default:
			return _Action{_ReduceAction, 0, _ReduceNonEmptyArgumentsToArguments}, true
		}
	case _State13:
		switch symbolId {
		case LparenToken:
			return _Action{_ShiftAction, _State3, 0}, true
		case AccessibleExprType:
			return _Action{_ShiftAction, _State4, 0}, true
		case IntegerLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceIntegerLiteralToLiteralExpr}, true
		case FloatLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceFloatLiteralToLiteralExpr}, true
		case RuneLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceRuneLiteralToLiteralExpr}, true
		case StringLiteralToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceStringLiteralToLiteralExpr}, true
		case TrueToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceTrueToLiteralExpr}, true
		case FalseToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceFalseToLiteralExpr}, true
		case IdentifierToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceToNamedExpr}, true
		case DollarIntegerToken:
			return _Action{_ShiftAndReduceAction, 0, _ReduceToPreviousResultExpr}, true
		case ExpressionType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceAppendToNonEmptyArguments}, true
		case AtomExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceAtomExprToAccessibleExpr}, true
		case LiteralExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceLiteralExprToAtomExpr}, true
		case NamedExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceNamedExprToAtomExpr}, true
		case PreviousResultExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReducePreviousResultExprToAtomExpr}, true
		case GroupedExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceGroupedExprToAtomExpr}, true
		case DirectAccessExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceDirectAccessExprToAccessibleExpr}, true
		case IndirectAccessExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceIndirectAccessExprToAccessibleExpr}, true
		case IndexExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceIndexExprToAccessibleExpr}, true
		case CallExprType:
			return _Action{_ShiftAndReduceAction, 0, _ReduceCallExprToAccessibleExpr}, true

		default:
			return _Action{_ReduceAction, 0, _ReduceImproperListToArguments}, true
		}
	}

	return _Action{}, false
}

var _ActionTable = _ActionTableType{}

/*
Parser Debug States:
  State 1:
    Kernel Items:
      #accept: ^.expression
    Reduce:
      (nil)
    ShiftAndReduce:
      INTEGER_LITERAL -> [literal_expr]
      FLOAT_LITERAL -> [literal_expr]
      RUNE_LITERAL -> [literal_expr]
      STRING_LITERAL -> [literal_expr]
      TRUE -> [literal_expr]
      FALSE -> [literal_expr]
      IDENTIFIER -> [named_expr]
      DOLLAR_INTEGER -> [previous_result_expr]
      atom_expr -> [accessible_expr]
      literal_expr -> [atom_expr]
      named_expr -> [atom_expr]
      previous_result_expr -> [atom_expr]
      grouped_expr -> [atom_expr]
      direct_access_expr -> [accessible_expr]
      indirect_access_expr -> [accessible_expr]
      index_expr -> [accessible_expr]
      call_expr -> [accessible_expr]
    Goto:
      LPAREN -> State 3
      expression -> State 2
      accessible_expr -> State 4

  State 2:
    Kernel Items:
      #accept: ^ expression., $
    Reduce:
      $ -> [#accept]
    ShiftAndReduce:
      (nil)
    Goto:
      (nil)

  State 3:
    Kernel Items:
      grouped_expr: LPAREN.expression RPAREN
    Reduce:
      (nil)
    ShiftAndReduce:
      INTEGER_LITERAL -> [literal_expr]
      FLOAT_LITERAL -> [literal_expr]
      RUNE_LITERAL -> [literal_expr]
      STRING_LITERAL -> [literal_expr]
      TRUE -> [literal_expr]
      FALSE -> [literal_expr]
      IDENTIFIER -> [named_expr]
      DOLLAR_INTEGER -> [previous_result_expr]
      atom_expr -> [accessible_expr]
      literal_expr -> [atom_expr]
      named_expr -> [atom_expr]
      previous_result_expr -> [atom_expr]
      grouped_expr -> [atom_expr]
      direct_access_expr -> [accessible_expr]
      indirect_access_expr -> [accessible_expr]
      index_expr -> [accessible_expr]
      call_expr -> [accessible_expr]
    Goto:
      LPAREN -> State 3
      expression -> State 5
      accessible_expr -> State 4

  State 4:
    Kernel Items:
      expression: accessible_expr., *
      direct_access_expr: accessible_expr.DOT IDENTIFIER
      indirect_access_expr: accessible_expr.ARROW IDENTIFIER
      index_expr: accessible_expr.LBRACKET expression RBRACKET
      call_expr: accessible_expr.LPAREN arguments RPAREN
    Reduce:
      * -> [expression]
    ShiftAndReduce:
      (nil)
    Goto:
      DOT -> State 7
      ARROW -> State 6
      LPAREN -> State 9
      LBRACKET -> State 8

  State 5:
    Kernel Items:
      grouped_expr: LPAREN expression.RPAREN
    Reduce:
      (nil)
    ShiftAndReduce:
      RPAREN -> [grouped_expr]
    Goto:
      (nil)

  State 6:
    Kernel Items:
      indirect_access_expr: accessible_expr ARROW.IDENTIFIER
    Reduce:
      (nil)
    ShiftAndReduce:
      IDENTIFIER -> [indirect_access_expr]
    Goto:
      (nil)

  State 7:
    Kernel Items:
      direct_access_expr: accessible_expr DOT.IDENTIFIER
    Reduce:
      (nil)
    ShiftAndReduce:
      IDENTIFIER -> [direct_access_expr]
    Goto:
      (nil)

  State 8:
    Kernel Items:
      index_expr: accessible_expr LBRACKET.expression RBRACKET
    Reduce:
      (nil)
    ShiftAndReduce:
      INTEGER_LITERAL -> [literal_expr]
      FLOAT_LITERAL -> [literal_expr]
      RUNE_LITERAL -> [literal_expr]
      STRING_LITERAL -> [literal_expr]
      TRUE -> [literal_expr]
      FALSE -> [literal_expr]
      IDENTIFIER -> [named_expr]
      DOLLAR_INTEGER -> [previous_result_expr]
      atom_expr -> [accessible_expr]
      literal_expr -> [atom_expr]
      named_expr -> [atom_expr]
      previous_result_expr -> [atom_expr]
      grouped_expr -> [atom_expr]
      direct_access_expr -> [accessible_expr]
      indirect_access_expr -> [accessible_expr]
      index_expr -> [accessible_expr]
      call_expr -> [accessible_expr]
    Goto:
      LPAREN -> State 3
      expression -> State 10
      accessible_expr -> State 4

  State 9:
    Kernel Items:
      call_expr: accessible_expr LPAREN.arguments RPAREN
    Reduce:
      * -> [arguments]
    ShiftAndReduce:
      INTEGER_LITERAL -> [literal_expr]
      FLOAT_LITERAL -> [literal_expr]
      RUNE_LITERAL -> [literal_expr]
      STRING_LITERAL -> [literal_expr]
      TRUE -> [literal_expr]
      FALSE -> [literal_expr]
      IDENTIFIER -> [named_expr]
      DOLLAR_INTEGER -> [previous_result_expr]
      expression -> [non_empty_arguments]
      atom_expr -> [accessible_expr]
      literal_expr -> [atom_expr]
      named_expr -> [atom_expr]
      previous_result_expr -> [atom_expr]
      grouped_expr -> [atom_expr]
      direct_access_expr -> [accessible_expr]
      indirect_access_expr -> [accessible_expr]
      index_expr -> [accessible_expr]
      call_expr -> [accessible_expr]
    Goto:
      LPAREN -> State 3
      accessible_expr -> State 4
      arguments -> State 11
      non_empty_arguments -> State 12

  State 10:
    Kernel Items:
      index_expr: accessible_expr LBRACKET expression.RBRACKET
    Reduce:
      (nil)
    ShiftAndReduce:
      RBRACKET -> [index_expr]
    Goto:
      (nil)

  State 11:
    Kernel Items:
      call_expr: accessible_expr LPAREN arguments.RPAREN
    Reduce:
      (nil)
    ShiftAndReduce:
      RPAREN -> [call_expr]
    Goto:
      (nil)

  State 12:
    Kernel Items:
      arguments: non_empty_arguments.COMMA
      arguments: non_empty_arguments., *
      non_empty_arguments: non_empty_arguments.COMMA expression
    Reduce:
      * -> [arguments]
    ShiftAndReduce:
      (nil)
    Goto:
      COMMA -> State 13

  State 13:
    Kernel Items:
      arguments: non_empty_arguments COMMA., *
      non_empty_arguments: non_empty_arguments COMMA.expression
    Reduce:
      * -> [arguments]
    ShiftAndReduce:
      INTEGER_LITERAL -> [literal_expr]
      FLOAT_LITERAL -> [literal_expr]
      RUNE_LITERAL -> [literal_expr]
      STRING_LITERAL -> [literal_expr]
      TRUE -> [literal_expr]
      FALSE -> [literal_expr]
      IDENTIFIER -> [named_expr]
      DOLLAR_INTEGER -> [previous_result_expr]
      expression -> [non_empty_arguments]
      atom_expr -> [accessible_expr]
      literal_expr -> [atom_expr]
      named_expr -> [atom_expr]
      previous_result_expr -> [atom_expr]
      grouped_expr -> [atom_expr]
      direct_access_expr -> [accessible_expr]
      indirect_access_expr -> [accessible_expr]
      index_expr -> [accessible_expr]
      call_expr -> [accessible_expr]
    Goto:
      LPAREN -> State 3
      accessible_expr -> State 4

Number of states: 13
Number of shift actions: 20
Number of reduce actions: 5
Number of shift-and-reduce actions: 92
Number of shift/reduce conflicts: 0
Number of reduce/reduce conflicts: 0
Number of unoptimized states: 130
Number of unoptimized shift actions: 325
Number of unoptimized reduce actions: 478
*/
