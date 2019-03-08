package front

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const eof = -1

type lexer struct {
	input  []byte
	pos    int
	start  int
	width  int
	stream []Token
}

type stateFn func(*lexer) stateFn

func (l *lexer) emit(t TokenType) {
	start, end := l.start, l.pos
	lexeme := string(l.input[l.start:l.pos])
	l.start = l.pos
	l.stream = append(l.stream, NewToken(lexeme, t, start, end))
}

func (l *lexer) ignore() {
	l.start = l.pos
}

func (l *lexer) rewind() {
	l.pos -= l.width
}

func (l *lexer) acceptRun(valid string) {
	for strings.IndexRune(valid, l.consume()) >= 0 {
	}
	l.rewind()
}

func (l *lexer) accept(valid string) bool {
	if strings.IndexRune(valid, l.consume()) >= 0 {
		return true
	}
	l.rewind()
	return false
}

func (l *lexer) peek() rune {
	res := l.consume()
	l.rewind()
	return res
}

func (l *lexer) consume() rune {
	if l.pos >= len(l.input) {
		l.width = 0
		return eof
	}

	res, width := utf8.DecodeRune(l.input[l.pos:])
	if res == utf8.RuneError {
		l.width = 0
		panic("oh")
	}
	l.width = width
	l.pos += l.width
	return res
}

func lexIdentifier(l *lexer) stateFn {
	for {
		switch c := l.consume(); {
		case isAlphaNumeric(c):
			// consume
		default:
			l.rewind()
			l.emit(Identifier)
			return lexStart
		}
	}
}

func lexNumber(l *lexer) stateFn {
	l.acceptRun("0123456789")
	if l.accept(".") {
		l.acceptRun("012345689")
	}
	l.emit(Number)
	return lexStart
}

func lexQuote(l *lexer) stateFn {
	if !l.accept(`"`) {
		panic("expect")
	}
	for {
		switch r := l.consume(); {
		default:
			// consume
		case r == '"':
			l.emit(String)
			return lexStart
		}
	}
}

var doubleSym = map[string]bool{
	"==": true,
	"!=": true,
	"&&": true,
	"||": true,
	"<=": true,
	">=": true,
}

func lexSymbol(l *lexer) stateFn {
	curr := string(l.consume()) + string(l.peek())

	if _, ok := doubleSym[curr]; ok {
		l.consume()
		l.emit(Symbol)
		return lexStart
	}

	l.emit(Symbol)
	return lexStart
}

func lexStart(l *lexer) stateFn {
	switch c := l.consume(); {
	case c == eof:
		l.rewind()
		return nil
	case unicode.IsDigit(c):
		l.rewind()
		return lexNumber
	case isAlphaNumeric(c):
		l.rewind()
		return lexIdentifier
	case isSymbol(c):
		l.rewind()
		return lexSymbol
	case c == '"':
		l.rewind()
		return lexQuote
	case c <= ' ':
		// layout, ignore.
		l.ignore()
		return lexStart
	default:
		fmt.Println("Unhandled char'", string(c), "' which is ", c)
		l.rewind()
		l.emit(EndOfFile)
		return nil
	}
}

func tokenizeInput(input string) []Token {
	l := &lexer{
		[]byte(input), 0, 0, 0, []Token{},
	}
	for s := lexStart; s != nil; {
		s = s(l)
	}
	return l.stream
}

var symbols = map[rune]bool{}

func registerSymbols(syms ...rune) {
	for _, sym := range syms {
		symbols[sym] = true
	}
}

func init() {
	registerSymbols(
		// arithmetic
		'+', '-', '/', '*', '%', '=',
		'(', ')', '{', '}', '[', ']', '<', '>',
		'.', '$', '!', '?', '#', '/', ',', '|', '&',
		'_', '~', ';', ':',
	)
}

func isSymbol(r rune) (ok bool) {
	_, ok = symbols[r]
	return ok
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

func isEndOfLine(r rune) bool {
	return r == '\r' || r == '\n'
}

func isAlphaNumeric(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}