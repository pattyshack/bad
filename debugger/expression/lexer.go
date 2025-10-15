package expression

import (
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/pattyshack/gt/parseutil"
	"github.com/pattyshack/gt/stringutil"
)

type Token = parseutil.Token[SymbolId]
type TokenValue = parseutil.TokenValue[SymbolId]

type Lexer = parseutil.Lexer[Token]

var (
	keywords = map[string]SymbolId{
		"true":  TrueToken,
		"false": FalseToken,
	}
)

type lexerImpl struct {
	parseutil.BufferedByteLocationReader
	*stringutil.InternPool
}

func newLexer(expression string) *lexerImpl {
	reader := parseutil.NewBufferedByteLocationReaderFromSlice(
		"",
		[]byte(expression))

	return &lexerImpl{
		BufferedByteLocationReader: reader,
		InternPool:                 stringutil.NewInternPool(),
	}
}

func (lexer *lexerImpl) CurrentLocation() parseutil.Location {
	return lexer.Location
}

func (lexer *lexerImpl) peekNextToken() (SymbolId, string, error) {
	peeked, err := lexer.Peek(utf8.UTFMax)
	if len(peeked) > 0 && err == io.EOF {
		err = nil
	}
	if err != nil {
		return 0, "", err
	}

	char := peeked[0]

	if ('a' <= char && char <= 'z') ||
		('A' <= char && char <= 'Z') ||
		char == '_' {

		return IdentifierToken, "", nil
	}

	if '0' <= char && char <= '9' {
		return IntegerLiteralToken, "", nil
	}

	switch char {
	case '.':
		if len(peeked) > 1 && '0' <= peeked[1] && peeked[1] <= '9' {
			return FloatLiteralToken, "", nil
		}
		return DotToken, ".", nil

	case '-':
		if len(peeked) > 1 && peeked[1] == '>' {
			return ArrowToken, "->", nil
		}
		return IntegerLiteralToken, "", nil

	case ',':
		return CommaToken, ",", nil
	case '\'':
		return RuneLiteralToken, "", nil
	case '"':
		return StringLiteralToken, "", nil
	case '$':
		return DollarIntegerToken, "", nil
	case '(':
		return LparenToken, "(", nil
	case ')':
		return RparenToken, ")", nil
	case '[':
		return LbracketToken, "[", nil
	case ']':
		return RbracketToken, "]", nil
	}

	utf8Char, size := utf8.DecodeRune(peeked)
	if size == 1 || utf8Char == utf8.RuneError {
		return 0, "", fmt.Errorf("Unexpected rune (%v)", utf8Char)
	}

	return IdentifierToken, "", nil
}

func (lexer *lexerImpl) lexIntegerOrFloatLiteralToken() (Token, error) {
	token, hasNoDigits, err := parseutil.MaybeTokenizeIntegerOrFloatLiteral(
		lexer.BufferedByteLocationReader,
		64,
		lexer.InternPool,
		IntegerLiteralToken,
		FloatLiteralToken)
	if err != nil {
		return nil, err
	}

	if token == nil {
		panic("should never happen")
	}

	if hasNoDigits {
		return nil, fmt.Errorf("%s has no digits", token.SubType)
	}

	return token, nil
}

func (lexer *lexerImpl) lexRuneLiteralToken() (Token, error) {
	token, errMsg, err := parseutil.MaybeTokenizeRuneLiteral(
		lexer.BufferedByteLocationReader,
		6,
		lexer.InternPool,
		RuneLiteralToken)
	if err != nil {
		return nil, err
	}

	if token == nil {
		panic("should never happen")
	}

	if errMsg != "" {
		return nil, fmt.Errorf("%s", errMsg)
	}

	return token, nil
}

func (lexer *lexerImpl) lexStringLiteralToken() (Token, error) {
	token, errMsg, err := parseutil.MaybeTokenizeStringLiteral(
		lexer.BufferedByteLocationReader,
		64,
		lexer.InternPool,
		StringLiteralToken,
		parseutil.SingleLineString,
		false)
	if err != nil {
		return nil, err
	}

	if token == nil {
		panic("should never happen")
	}

	if errMsg != "" {
		return nil, fmt.Errorf("%s", errMsg)
	}

	return token, nil
}

func (lexer *lexerImpl) lexDollarIntegerToken() (Token, error) {
	start := lexer.Location
	bytes := []byte{}

	for {
		peeked, err := lexer.Peek(1)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if len(bytes) == 0 {
			if peeked[0] != '$' {
				panic("should never happen")
			}
		} else if '0' <= peeked[0] && peeked[0] <= '9' {
			// do nothing
		} else {
			break
		}

		bytes = append(bytes, peeked[0])

		n, err := lexer.Discard(1)
		if err != nil {
			return nil, err
		}
		if n != 1 {
			panic("should never happen")
		}
	}

	if len(bytes) == 1 {
		return nil, fmt.Errorf("Dollar not followed by integer")
	}

	return &TokenValue{
		SymbolId:    DollarIntegerToken,
		StartEndPos: parseutil.NewStartEndPos(start, lexer.Location),
		Value:       string(bytes),
	}, nil
}

func (lexer *lexerImpl) lexIdentifierOrKeyword() (Token, error) {
	token, err := parseutil.MaybeTokenizeIdentifier(
		lexer.BufferedByteLocationReader,
		64,
		lexer.InternPool,
		IdentifierToken)
	if err != nil {
		return nil, err
	}

	if token == nil {
		panic("Should never hapapen")
	}

	kwSymbolId, ok := keywords[token.Value]
	if ok {
		token.SymbolId = kwSymbolId
	}

	return token, nil
}

func (lexer *lexerImpl) Next() (Token, error) {
	err := parseutil.StripLeadingWhitespaces(lexer.BufferedByteLocationReader)
	if err != nil {
		return nil, err
	}

	symbolId, value, err := lexer.peekNextToken()
	if err != nil {
		return nil, err
	}

	// fixed length token
	if len(value) > 0 {
		loc := lexer.Location

		_, err := lexer.Discard(len(value))
		if err != nil {
			panic("should never happen")
		}

		return &TokenValue{
			SymbolId:    symbolId,
			StartEndPos: parseutil.NewStartEndPos(loc, lexer.Location),
			Value:       value,
		}, nil
	}

	switch symbolId {
	case IntegerLiteralToken, FloatLiteralToken:
		return lexer.lexIntegerOrFloatLiteralToken()
	case RuneLiteralToken:
		return lexer.lexRuneLiteralToken()
	case StringLiteralToken:
		return lexer.lexStringLiteralToken()
	case DollarIntegerToken:
		return lexer.lexDollarIntegerToken()
	case IdentifierToken:
		return lexer.lexIdentifierOrKeyword()
	}

	panic(fmt.Sprintf("unhandled variable length token: %v", symbolId))
}
