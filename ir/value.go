package ir

import (
	"encoding/gob"
	"math/big"
)

func init() {
	gob.Register(&IntegerValue{})
	gob.Register(&BinaryExpression{})
	gob.Register(&Identifier{})
	gob.Register(&Grouping{})
	gob.Register(&Builtin{})
	gob.Register(&UnaryExpression{})
}

type Value interface{}

// PAREN EXPR

type Grouping struct {
	Val Value
}

func NewGrouping(val Value) *Grouping {
	return &Grouping{val}
}

// BINARY

type BinaryExpression struct {
	LHand Value
	Op    string
	RHand Value
}

func NewBinaryExpression(lh Value, op string, rh Value) *BinaryExpression {
	return &BinaryExpression{lh, op, rh}
}

// UNARY

type UnaryExpression struct {
	Op  string
	Val Value
}

func NewUnaryExpression(op string, val Value) *UnaryExpression {
	return &UnaryExpression{op, val}
}

// INTEGER VALUE

type IntegerValue struct {
	RawValue *big.Int
}

func NewIntegerValue(val *big.Int) *IntegerValue {
	return &IntegerValue{val}
}

// IDENTIFIER

type Identifier struct {
	Name string
}

func NewIdentifier(name string) *Identifier {
	return &Identifier{name}
}

// BUILTIN

type Builtin struct {
	Name string
	Type Type
}

func NewBuiltin(name string, typ Type) *Builtin {
	return &Builtin{name, typ}
}
