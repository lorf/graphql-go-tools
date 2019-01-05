package lexer

import (
	"fmt"
	"github.com/jensneuse/graphql-go-tools/pkg/lexing/keyword"
	"github.com/jensneuse/graphql-go-tools/pkg/lexing/literal"
	"github.com/jensneuse/graphql-go-tools/pkg/lexing/position"
	"github.com/jensneuse/graphql-go-tools/pkg/lexing/runes"
	"github.com/jensneuse/graphql-go-tools/pkg/lexing/token"
	"strings"
	"unicode"
)

// Lexer emits tokens from a input reader
type Lexer struct {
	input                                string
	inputPosition                        int
	textPosition                         position.Position
	beforeLastLineTerminatorTextPosition position.Position
}

type parsedRune struct {
	r   rune
	pos position.Position
}

// NewLexer initializes a new lexer
func NewLexer() *Lexer {
	return &Lexer{}
}

// SetInput sets the new reader as input and resets all position stats
func (l *Lexer) SetInput(input string) {
	l.input = input
	l.inputPosition = 0
	l.textPosition.Line = 1
	l.textPosition.Char = 1
}

// Read emits the next token, this cannot be undone
func (l *Lexer) Read() (tok token.Token, err error) {

	var next parsedRune

	for {
		next = l.readRune()
		if !l.runeIsWhitespace(next.r) {
			break
		}
	}

	var matched bool
	tok, matched = l.matchSingleRuneToken(next)
	if matched {
		return tok, nil
	}

	switch next.r {
	case runes.QUOTE:
		return l.readString(next)
	case runes.DOT:
		return l.readSpread(next)
	case runes.DOLLAR:
		return l.readVariable(next)
	}

	if runeIsDigit(next.r) {
		return l.readDigit(next)
	}

	return l.readIdent(next)
}

func (l *Lexer) swallowWhitespace() (err error) {

	var next parsedRune

	for {
		next = l.readRune()

		if next.r == runes.EOF {
			return nil
		}

		if !l.runeIsWhitespace(next.r) {
			return l.unreadRune()
		}
	}
}

// Peek will emit the next keyword without advancing the reader position
func (l *Lexer) Peek(ignoreWhitespace bool) (key keyword.Keyword, err error) {

	if ignoreWhitespace {
		err = l.swallowWhitespace()
		if err != nil {
			return key, err
		}
	}

	next := l.peekRune()
	if err != nil {
		return key, err
	}

	return l.keywordFromRune(next)
}

func (l *Lexer) keywordFromRune(r rune) (key keyword.Keyword, err error) {

	switch r {
	case runes.EOF:
		return keyword.EOF, nil
	case runes.SPACE:
		return keyword.SPACE, nil
	case runes.TAB:
		return keyword.TAB, nil
	case runes.COMMA:
		return keyword.COMMA, nil
	case runes.LINETERMINATOR:
		return runes.LINETERMINATOR, nil
	case runes.QUOTE:
		return keyword.STRING, nil
	case runes.DOLLAR:
		return keyword.VARIABLE, nil
	case runes.PIPE:
		return keyword.PIPE, nil
	case runes.EQUALS:
		return keyword.EQUALS, nil
	case runes.AT:
		return keyword.AT, nil
	case runes.COLON:
		return keyword.COLON, nil
	case runes.BANG:
		return keyword.BANG, nil
	case runes.BRACKETOPEN:
		return keyword.BRACKETOPEN, nil
	case runes.BRACKETCLOSE:
		return keyword.BRACKETCLOSE, nil
	case runes.CURLYBRACKETOPEN:
		return keyword.CURLYBRACKETOPEN, nil
	case runes.CURLYBRACKETCLOSE:
		return keyword.CURLYBRACKETCLOSE, nil
	case runes.SQUAREBRACKETOPEN:
		return keyword.SQUAREBRACKETOPEN, nil
	case runes.SQUAREBRACKETCLOSE:
		return keyword.SQUAREBRACKETCLOSE, nil
	case runes.AND:
		return keyword.AND, nil
	case runes.DOT:
		if l.peekEquals("...") {
			return keyword.SPREAD, nil
		}
		return key, fmt.Errorf("keywordFromRune: must be '...'")
	}

	if unicode.IsDigit(r) {
		if l.peekIsFloat() {
			return keyword.FLOAT, nil
		}
		return keyword.INTEGER, nil
	}

	return l.peekIdent(), nil
}

func (l *Lexer) peekIsFloat() (isFloat bool) {

	var hasDot bool
	var peeked rune

	for i := l.inputPosition; i < len(l.input); i++ {

		peeked = rune(l.input[i])

		if peeked == runes.EOF {
			return hasDot
		} else if l.runeIsWhitespace(peeked) {
			return hasDot
		} else if peeked == runes.DOT && !hasDot {
			hasDot = true
		} else if peeked == runes.DOT && hasDot {
			return false
		} else if !unicode.IsDigit(peeked) {
			return false
		}
	}

	return hasDot
}

func (l *Lexer) matchSingleRuneToken(r parsedRune) (tok token.Token, matched bool) {

	matched = true

	switch r.r {
	case runes.EOF:
		tok = token.EOF
	case runes.PIPE:
		tok = token.Pipe
	case runes.EQUALS:
		tok = token.Equals
	case runes.AT:
		tok = token.At
	case runes.COLON:
		tok = token.Colon
	case runes.BANG:
		tok = token.Bang
	case runes.BRACKETOPEN:
		tok = token.BracketOpen
	case runes.BRACKETCLOSE:
		tok = token.BracketClose
	case runes.CURLYBRACKETOPEN:
		tok = token.CurlyBracketOpen
	case runes.CURLYBRACKETCLOSE:
		tok = token.CurlyBracketClose
	case runes.SQUAREBRACKETOPEN:
		tok = token.SquaredBracketOpen
	case runes.SQUAREBRACKETCLOSE:
		tok = token.SquaredBracketClose
	case runes.AND:
		tok = token.And
	default:
		matched = false
	}

	tok.Position = r.pos

	return
}

func (l *Lexer) readIdent(startRune parsedRune) (tok token.Token, err error) {

	tok.Position = startRune.pos
	start := l.inputPosition - 1
	var r parsedRune

	for {
		r = l.readRune()
		if !runeIsIdent(r.r) {
			break
		}
	}

	if r.r != runes.EOF && l.inputPosition > start+1 {
		err = l.unreadRune()
		if err != nil {
			return tok, err
		}
	}

	end := l.inputPosition

	tok.Literal = l.input[start:end]
	tok.Keyword = l.keywordFromIdentString(tok.Literal)

	return
}

const identWantRunes = 13

func (l *Lexer) peekIdent() (k keyword.Keyword) {

	start := l.inputPosition

	end := l.inputPosition + identWantRunes
	if end > len(l.input) {
		end = len(l.input)
	}

	peeked := l.input[start:end]

	for i, r := range peeked {
		if !runeIsIdent(r) {
			peeked = peeked[:i]
			break
		}
	}

	return l.keywordFromIdentString(peeked)
}

func (l *Lexer) keywordFromIdentString(ident string) (k keyword.Keyword) {
	switch ident {
	case "on":
		return keyword.ON
	case "true":
		return keyword.TRUE
	case "type":
		return keyword.TYPE
	case "null":
		return keyword.NULL
	case "enum":
		return keyword.ENUM
	case "false":
		return keyword.FALSE
	case "union":
		return keyword.UNION
	case "query":
		return keyword.QUERY
	case "input":
		return keyword.INPUT
	case "schema":
		return keyword.SCHEMA
	case "scalar":
		return keyword.SCALAR
	case "mutation":
		return keyword.MUTATION
	case "fragment":
		return keyword.FRAGMENT
	case "interface":
		return keyword.INTERFACE
	case "directive":
		return keyword.DIRECTIVE
	case "implements":
		return keyword.IMPLEMENTS
	case "subscription":
		return keyword.SUBSCRIPTION
	default:
		return keyword.IDENT
	}
}

func (l *Lexer) readVariable(startRune parsedRune) (tok token.Token, err error) {

	tok.Position = startRune.pos
	tok.Keyword = keyword.VARIABLE

	peeked, err := l.Peek(false)
	if err != nil {
		return tok, err
	}

	if peeked == keyword.SPACE ||
		peeked == keyword.TAB ||
		peeked == keyword.COMMA ||
		peeked == keyword.LINETERMINATOR {
		return tok, fmt.Errorf("readVariable: must not have whitespace after $")
	}

	ident, err := l.readIdent(startRune)
	if err != nil {
		return tok, err
	}

	tok.Literal = ident.Literal[1:]
	return
}

func (l *Lexer) readSpread(startRune parsedRune) (tok token.Token, err error) {

	isSpread := l.peekEquals("..")

	if !isSpread {
		tok.Position = startRune.pos
		return tok, fmt.Errorf("readSpread: invalid '.' at position %s", startRune.pos.String())
	}

	l.swallowAmount(2)

	tok = token.Spread
	tok.Position = startRune.pos
	return
}

func (l *Lexer) readString(startRune parsedRune) (tok token.Token, err error) {

	isMultiLineString := l.peekEquals("\"\"")

	if isMultiLineString {
		l.swallowAmount(2)
		return l.readMultiLineString(startRune)
	}

	return l.readSingleLineString(startRune)
}

func (l *Lexer) swallowAmount(amount int) {
	for i := 0; i < amount; i++ {
		l.readRune()
	}
}

func (l *Lexer) peekEquals(equals string) bool {

	start := l.inputPosition
	end := l.inputPosition + len(equals)

	if end > len(l.input) {
		return false
	}

	return l.input[start:end] == equals
}

func (l *Lexer) readDigit(startRune parsedRune) (tok token.Token, err error) {

	tok.Position = startRune.pos

	start := l.inputPosition - 1

	var r parsedRune
	for {
		r = l.readRune()
		if !runeIsDigit(r.r) {
			break
		}
	}

	isFloat := r.r == runes.DOT

	if isFloat {
		l.swallowAmount(1)
		return l.readFloat(startRune.pos, start)
	}

	if r.r != runes.EOF {
		err = l.unreadRune()
		if err != nil {
			return tok, err
		}
	}

	end := l.inputPosition

	tok.Keyword = keyword.INTEGER
	tok.Literal = l.input[start:end]

	return
}

func (l *Lexer) readFloat(position position.Position, start int) (tok token.Token, err error) {

	tok.Position = position

	var valid bool

	var r parsedRune
	for {
		r = l.readRune()
		if !runeIsDigit(r.r) {
			break
		} else if !valid {
			valid = true
		}
	}

	if !valid {
		return tok, fmt.Errorf("readFloat: incomplete float, must have digits after dot")
	}

	if r.r != runes.EOF {
		err = l.unreadRune()
		if err != nil {
			return tok, err
		}
	}

	end := l.inputPosition

	tok.Keyword = keyword.FLOAT
	tok.Literal = l.input[start:end]

	return
}

func (l *Lexer) trimStartEnd(input, trim string) string {
	return strings.TrimSuffix(strings.TrimPrefix(input, trim), trim)
}

func (l *Lexer) readRune() (r parsedRune) {

	r.pos.Line = l.textPosition.Line
	r.pos.Char = l.textPosition.Char

	if l.inputPosition < len(l.input) {
		r.r = rune(l.input[l.inputPosition])

		if r.r == runes.LINETERMINATOR {
			l.beforeLastLineTerminatorTextPosition = l.textPosition
			l.textPosition.Line++
			l.textPosition.Char = 1
		} else {
			l.textPosition.Char++
		}

		l.inputPosition++
	} else {
		r.r = runes.EOF
	}

	return
}

func (l *Lexer) unreadRune() error {

	if l.inputPosition == 0 {
		return fmt.Errorf("unreadRune: cannot unread from inputPosition 0")
	}

	l.inputPosition--

	r := rune(l.input[l.inputPosition])
	if r == runes.LINETERMINATOR {
		l.textPosition = l.beforeLastLineTerminatorTextPosition
	} else {
		l.textPosition.Char--
	}

	return nil
}

func (l *Lexer) peekRune() (r rune) {

	if l.inputPosition < len(l.input) {
		return rune(l.input[l.inputPosition])
	}

	return runes.EOF
}

func runeIsIdent(r rune) bool {
	switch r {
	case 'a', 'b', 'c', 'd', 'e', 'f', 'g', 'h', 'i', 'j', 'k', 'l', 'm', 'n', 'o', 'p', 'q', 'r', 's', 't', 'u', 'v', 'w', 'x', 'y', 'z', 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M', 'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z', '1', '2', '3', '4', '5', '6', '7', '8', '9', '0', runes.NEGATIVESIGN, runes.UNDERSCORE:
		return true
	default:
		return false
	}
}

func runeIsDigit(r rune) bool {
	switch r {
	case '1', '2', '3', '4', '5', '6', '7', '8', '9', '0':
		return true
	default:
		return false
	}
}

func (l *Lexer) runeIsWhitespace(r rune) bool {
	switch r {
	case runes.SPACE, runes.TAB, runes.LINETERMINATOR, runes.COMMA:
		return true
	default:
		return false
	}
}

func (l *Lexer) readMultiLineString(startRune parsedRune) (tok token.Token, err error) {

	tok.Keyword = keyword.STRING
	tok.Position = startRune.pos

	start := l.inputPosition

	var escaped bool

	for {

		nextRune := l.readRune()

		switch nextRune.r {
		case runes.QUOTE:
			if escaped {
				escaped = false
			} else {

				isMultiLineStringEnd := l.peekEquals("\"\"")

				if !isMultiLineStringEnd {
					escaped = false
				} else {

					end := l.inputPosition - 1
					l.swallowAmount(2)

					tok.Literal = l.trimStartEnd(l.input[start:end], literal.LINETERMINATOR)
					return tok, nil
				}
			}
		case runes.BACKSLASH:
			if escaped {
				escaped = false
			} else {
				escaped = true
			}
		default:
			escaped = false
		}
	}
}

func (l *Lexer) readSingleLineString(startRune parsedRune) (tok token.Token, err error) {

	tok.Keyword = keyword.STRING
	tok.Position = startRune.pos

	start := l.inputPosition

	var escaped bool

	for {

		nextRune := l.readRune()

		switch nextRune.r {
		case runes.QUOTE:
			if escaped {
				escaped = false
			} else {
				end := l.inputPosition - 1
				tok.Literal = l.input[start:end]
				return tok, nil
			}
		case runes.BACKSLASH:
			if escaped {
				escaped = false
			} else {
				escaped = true
			}
		default:
			escaped = false
		}
	}
}