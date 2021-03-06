package front

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/krug-lang/caasper/api"
)

// keywords
const (
	fn       string = "fn"
	let             = "let"
	typ             = "type"
	mut             = "mut"
	brk             = "break"
	ret             = "return"
	next            = "next"
	trait           = "trait"
	struc           = "struct"
	impl            = "impl"
	comptime        = "comptime"
	loop            = "loop"
	deferr          = "defer"
	while           = "while"
	iff             = "if"
)

type astParser struct {
	parser
}

// BadToken represents a bad token that
// is unsupported in the language
var BadToken Token

func (p *astParser) parsePointerType() *TypeNode {
	start := p.pos
	p.expect("*")
	base := p.parseTypeExpression()
	if base == nil {
		p.error(api.NewParseError("type after pointer", start, p.pos))
		return nil
	}

	return &TypeNode{
		Kind: PointerType,
		PointerTypeNode: &PointerTypeNode{
			Base: base,
		},
	}
}

func (p *astParser) parseArrayType() *TypeNode {
	start := p.pos

	p.expect("[")
	base := p.parseTypeExpression()
	if base == nil {
		p.error(api.NewParseError("array type", start, p.pos))
	}

	p.expect(";")

	size := p.parseExpression()
	if size == nil {
		p.error(api.NewParseError("array length constant", start, p.pos))
	}

	p.expect("]")
	return &TypeNode{
		Kind: ArrayType,
		ArrayTypeNode: &ArrayTypeNode{
			Base: base,
			Size: size,
		},
	}
}

func (p *astParser) parseUnresolvedType() *TypeNode {
	name := p.expectKind(Identifier)
	return &TypeNode{
		Kind: UnresolvedType,
		UnresolvedTypeNode: &UnresolvedTypeNode{
			Name: name.Value,
		},
	}
}

func (p *astParser) parseTupleType() *TypeNode {
	p.expect("(")
	var types []*ExpressionNode
	for p.hasNext() {
		if p.next().Matches(",") {
			p.consume()
		}

		if p.next().Matches(")") {
			break
		}

		if typ := p.parseTypeExpression(); typ != nil {
			types = append(types, typ)
		}
	}
	p.expect(")")

	return &TypeNode{
		Kind: TupleType,
		TupleTypeNode: &TupleTypeNode{
			types,
		},
	}
}

// parseTypeExpression is an expression, though it is constrained
// to be either a function, or a type, e.g. a pointer, or an array.
func (p *astParser) parseTypeExpression() *ExpressionNode {
	start := p.pos
	curr := p.next()

	res := &ExpressionNode{
		Kind: TypeExpression,
	}

	switch {
	case curr.Matches("*"):
		res.TypeExpressionNode = p.parsePointerType()
	case curr.Matches("["):
		res.TypeExpressionNode = p.parseArrayType()
	case curr.Matches("("):
		res.TypeExpressionNode = p.parseTupleType()
	case curr.Matches(struc):
		res.TypeExpressionNode = p.parseStructureType()
	case curr.Kind == Identifier:
		res.TypeExpressionNode = p.parseUnresolvedType()
	default:
		p.error(api.NewUnimplementedError("parseTypeExpression", "type", start, p.pos))
		return nil
	}

	return res
}

func (p *astParser) parseStructureType() *TypeNode {
	start := p.pos

	fst := p.expect("struct")

	fields := []*NamedType{}

	p.expect("{")
	for p.hasNext() {
		if p.next().Matches("}") {
			break
		}

		name := p.expectKind(Identifier)

		typ := p.parseTypeExpression()
		if typ == nil {
			p.error(api.NewParseError("type", start, p.pos))
		}

		// NOTE: structure fields are mutable by default.
		// immutable structure fields will not be an option
		// in the future as it's too confusing and doesn't
		// really make sense.
		// IN ADDITION the fields are not owned by anything.
		// FIXME how should this be?
		fields = append(fields, &NamedType{true, name, false, typ})

		// trailing commas are enforced.
		p.expect(",")
	}
	p.expect("}")

	return &TypeNode{
		Kind: StructureType,
		StructureTypeNode: &StructureTypeNode{
			Name:   fst,
			Fields: fields,
		},
	}
}

func (p *astParser) parseFunctionPrototypeDeclaration() *FunctionPrototypeDeclaration {
	start := p.pos

	p.expect(fn)
	name := p.expectKind(Identifier)

	args := []*NamedType{}

	p.expect("(")
	for idx := 0; p.hasNext(); idx++ {
		if p.next().Matches(")") {
			break
		}

		// no trailing commas allowed here.
		if idx != 0 {
			p.expect(",")
		}

		mutable := false
		if p.next().Matches(mut) {
			mutable = true
			p.consume()
		}

		owned := true
		if p.next().Matches("~") {
			owned = false
			p.consume()
		}

		name := p.expectKind(Identifier)
		typ := p.parseExpression()
		if typ == nil {
			p.error(api.NewParseError("type after pointer", start, p.pos))
		}

		args = append(args, &NamedType{mutable, name, owned, typ})
	}
	p.expect(")")

	var typ *ExpressionNode

	// { 	FuncDecl body
	// ; 	type in a let statement, e.g.
	// , 	member in a structure
	// if we dont have any of these, parse a type!
	if !p.next().Matches("{", ";", ",") {
		typ = p.parseTypeExpression()
	}

	return &FunctionPrototypeDeclaration{
		Name:      name,
		Arguments: args,

		// could be nil!
		ReturnType: typ,
	}
}

// mut x [ type ] [ = val ]
func (p *astParser) parseMut() *ParseTreeNode {
	start := p.pos

	p.expect(mut)

	owned := true
	if p.next().Matches("~") {
		p.consume()
		owned = false
	}

	name := p.expectKind(Identifier)

	var typ *ExpressionNode
	if !p.next().Matches("=") {
		typ = p.parseTypeExpression()
		if typ == nil {
			p.error(api.NewParseError("type after assignment", start, p.pos))
		}
	}

	var val *ExpressionNode
	if p.next().Matches("=") {
		p.expect("=")

		val = p.parseExpression()
		if val == nil {
			p.error(api.NewParseError("assignment", start, p.pos))
		}
	}

	if val == nil && typ == nil {
		p.error(api.NewParseError("value or type in mut statement", start, p.pos))
	}

	return &ParseTreeNode{
		Kind: MutableStatement,
		MutableStatementNode: &MutableStatementNode{
			Name:  name,
			Owned: owned,
			Type:  typ,
			Value: val,
		},
	}
}

// let is a constant variable.
func (p *astParser) parseLet() *ParseTreeNode {
	start := p.pos

	p.expect(let)

	owned := true
	if p.next().Matches("~") {
		p.consume()
		owned = false
	}

	name := p.expectKind(Identifier)

	var typ *ExpressionNode
	if !p.next().Matches("=") {
		typ = p.parseTypeExpression()
		if typ == nil {
			p.error(api.NewParseError("type or assignment", start, p.pos))
		}
	}

	var value *ExpressionNode
	if p.next().Matches("=") {
		p.expect("=")
		value = p.parseExpression()
		if value == nil {
			p.error(api.NewParseError("expression in let statement", start, p.pos))
		}
	}

	return &ParseTreeNode{
		Kind: LetStatement,
		LetStatementNode: &LetStatementNode{
			Name:  name,
			Owned: owned,
			Type:  typ,
			Value: value,
		},
	}
}

func (p *astParser) parseReturn() *ParseTreeNode {
	if !p.next().Matches("return") {
		return nil
	}
	start := p.pos
	p.expect("return")

	var res *ExpressionNode
	if !p.next().Matches(";") {
		res = p.parseExpression()
		if res == nil {
			p.error(api.NewParseError("semi-colon or expression", start, p.pos))
		}
	}

	return &ParseTreeNode{
		Kind: ReturnStatement,
		ReturnStatementNode: &ReturnStatementNode{
			Value: res,
		},
	}
}

func (p *astParser) parseNext() *ParseTreeNode {
	p.expect("next")
	return &ParseTreeNode{
		Kind: NextStatement,
	}
}

func (p *astParser) parseBreak() *ParseTreeNode {
	p.expect("break")
	return &ParseTreeNode{
		Kind: BreakStatement,
	}
}

func (p *astParser) parseTypeAlias() *ParseTreeNode {
	start := p.pos

	p.expect(typ)

	name := p.expectKind(Identifier)

	p.expect("=")
	typ := p.parseTypeExpression()
	if typ == nil {
		p.error(api.NewParseError("type or assignment", start, p.pos))
	}

	return &ParseTreeNode{
		Kind: TypeAliasStatement,
		TypeAliasNode: &TypeAliasNode{
			Name: name,
			Type: typ,
		},
	}
}

func (p *astParser) parseSemicolonStatement() *ParseTreeNode {
	switch curr := p.next(); {
	case curr.Matches(mut):
		return p.parseMut()
	case curr.Matches(let):
		return p.parseLet()
	case curr.Matches(typ):
		return p.parseTypeAlias()
	case curr.Matches(ret):
		return p.parseReturn()
	case curr.Matches("$"):
		return p.parseLabel()
	case curr.Matches("jump"):
		return p.parseJump()
	case curr.Matches(next):
		return p.parseNext()
	case curr.Matches(brk):
		return p.parseBreak()
	}

	return p.parseExpressionStatement()
}

func (p *astParser) parseStatBlock() *BlockNode {
	if !p.next().Matches("{") {
		return nil
	}

	stats := []*ParseTreeNode{}
	p.expect("{")
	for p.hasNext() {
		if p.next().Matches("}") {
			break
		}

		if stat := p.parseStatement(); stat != nil {
			stats = append(stats, stat)
		}
	}
	p.expect("}")

	return &BlockNode{
		Statements: stats,
	}
}

func (p *astParser) parseIfElseChain() *ParseTreeNode {
	if !p.next().Matches("if") {
		return nil
	}

	start := p.pos

	// the first if.
	p.expect("if")
	expr := p.parseExpression()
	if expr == nil {
		p.error(api.NewParseError("condition", start, p.pos))
	}

	block := p.parseStatBlock()
	if block == nil {
		p.error(api.NewParseError("block after condition", start, p.pos))
	}

	var elseBlock *BlockNode
	elses := []*ElseIfNode{}

	if p.next().Matches("else") && (p.hasNext() && p.peek(1).Matches("if")) {
		for p.hasNext() {
			p.expect("else")
			p.expect("if")

			cond := p.parseExpression()
			if cond == nil {
				p.error(api.NewParseError("condition in else if", start, p.pos))
				break
			}

			fmt.Println(cond, " and then ", p.next())

			body := p.parseStatBlock()
			if body == nil {
				p.error(api.NewParseError("block after else if", start, p.pos))
				break
			}

			elses = append(elses, &ElseIfNode{cond, body})
		}
	} else if p.next().Matches("else") {
		p.expect("else")

		body := p.parseStatBlock()
		if body == nil {
			p.error(api.NewParseError("block after else", start, p.pos))
		}

		if elseBlock != nil {
			// we already have an else, throw an error.
		}
		elseBlock = body
	}

	return &ParseTreeNode{
		Kind: IfStatement,
		IfNode: &IfNode{
			Cond:    expr,
			Block:   block,
			Else:    elseBlock,
			ElseIfs: elses,
		},
	}
}

func (p *astParser) parseWhileLoop() *ParseTreeNode {
	if !p.next().Matches("while") {
		return nil
	}
	start := p.pos

	p.expect("while")
	val := p.parseExpression()
	if val == nil {
		p.error(api.NewParseError("condition after while", start, p.pos))
	}

	var post *ExpressionNode
	if p.next().Matches(";") {
		p.expect(";")
		post = p.parseExpression()
		if post == nil {
			p.error(api.NewParseError("step expression in while loop", start, p.pos))
		}
	}

	if block := p.parseStatBlock(); block != nil {
		return &ParseTreeNode{
			Kind: WhileLoopStatement,
			WhileLoopNode: &WhileLoopNode{
				Cond:  val,
				Post:  post,
				Block: block,
			},
		}
	}

	return nil
}

func (p *astParser) parseLabel() *ParseTreeNode {
	if !p.next().Matches("$") {
		return nil
	}
	p.expect("$")
	labelName := p.expectKind(Identifier)

	return &ParseTreeNode{
		Kind:      LabelStatement,
		LabelNode: &LabelNode{labelName},
	}
}

func (p *astParser) parseJump() *ParseTreeNode {
	if !p.next().Matches("jump") {
		return nil
	}
	p.expect("jump")

	label := p.expectKind(Identifier)
	return &ParseTreeNode{
		Kind:     JumpStatement,
		JumpNode: &JumpNode{label},
	}
}

func (p *astParser) parseDefer() *ParseTreeNode {
	if !p.next().Matches("defer") {
		return nil
	}
	p.expect("defer")

	var block *BlockNode
	var stat *ParseTreeNode

	// block or statement
	if p.next().Matches("{") {
		b := p.parseStatBlock()
		if b == nil {
			p.error(api.NewParseError("Expected a block yo"))
			return nil
		}
		block = b
	} else {
		stat = p.parseStatement()
	}

	return &ParseTreeNode{
		Kind: DeferStatement,
		DeferNode: &DeferNode{
			Block:     block,
			Statement: stat,
		},
	}
}

func (p *astParser) parseLoop() *ParseTreeNode {
	if !p.next().Matches(loop) {
		return nil
	}
	p.expect(loop)
	if block := p.parseStatBlock(); block != nil {
		return &ParseTreeNode{
			Kind: LoopStatement,
			LoopNode: &LoopNode{
				Block: block,
			},
		}
	}
	return nil
}

func (p *astParser) parseStatement() *ParseTreeNode {
	switch curr := p.next(); {
	case curr.Matches(iff):
		return p.parseIfElseChain()
	case curr.Matches(loop):
		return p.parseLoop()
	case curr.Matches(while):
		return p.parseWhileLoop()
	case curr.Matches(deferr):
		return p.parseDefer()
	case curr.Matches("{"):
		return &ParseTreeNode{
			Kind:      BlockStatement,
			BlockNode: p.parseStatBlock(),
		}
	}

	stat := p.parseSemicolonStatement()
	if stat != nil {
		p.expect(";")
	}
	return stat
}

func (p *astParser) parseFunctionDeclaration() *FunctionDeclaration {
	proto := p.parseFunctionPrototypeDeclaration()

	body := p.parseStatBlock()
	return &FunctionDeclaration{proto, body}
}

func (p *astParser) parseImplDeclaration() *ImplDeclaration {
	p.expect("impl")
	name := p.expectKind(Identifier)

	functions := []*FunctionDeclaration{}

	p.expect("{")
	for p.hasNext() {
		if p.next().Matches("}") {
			break
		}

		// NOTE: we dont care if impls are empty.
		if fn := p.parseFunctionDeclaration(); fn != nil {
			functions = append(functions, fn)
		}
	}
	p.expect("}")

	return &ImplDeclaration{
		name, functions,
	}
}

func (p *astParser) parseTraitDeclaration() *TraitDeclaration {
	p.expect("trait")

	name := p.expectKind(Identifier)

	members := []*FunctionPrototypeDeclaration{}

	p.expect("{")
	for p.hasNext() {
		if p.next().Matches("}") {
			break
		}

		// we only parse prototypes here, not
		// function bodies.

		pt := p.parseFunctionPrototypeDeclaration()
		if pt == nil {
			break
		}
		members = append(members, pt)

		// must have semi-colon after each prototype.
		p.expect(";")
	}
	p.expect("}")

	return &TraitDeclaration{name, members}
}

func (p *astParser) parseUnaryExpr() *ExpressionNode {
	if !p.hasNext() || !p.next().Matches(unaryOperators...) {
		return nil
	}

	start := p.pos

	op := p.consume()
	right := p.parseLeft()
	if right == nil {
		p.error(api.NewParseError("unary expression", start, p.pos))
	}

	return &ExpressionNode{
		Kind:                UnaryExpression,
		UnaryExpressionNode: &UnaryExpressionNode{op.Value, right},
	}
}

func (p *astParser) parseGroupedList(fst *ExpressionNode) *ExpressionNode {
	list := []*ExpressionNode{fst}

	for p.hasNext() && p.next().Matches(",") {
		p.expect(",")
		val := p.parseExpression()
		if val == nil {
			panic("todo")
		}
		list = append(list, val)
	}

	p.expect(")")

	return &ExpressionNode{
		Kind: ListExpression,
		ExprList: &ExprList{
			list,
		},
	}
}

func (p *astParser) parseOperand() *ExpressionNode {
	if !p.hasNext() {
		return nil
	}

	start := p.pos
	curr := p.next()

	// group or grouped list
	// i.e. (1 + 2)
	// or (1, 2, 3, 4)
	if curr.Matches("(") {
		p.expect("(")
		expr := p.parseExpression()
		if p.next().Matches(",") {
			return p.parseGroupedList(expr)
		}

		p.expect(")")
		return &ExpressionNode{
			Kind:         Grouping,
			GroupingNode: &GroupingNode{expr},
		}
	}

	switch curr := p.consume(); curr.Kind {
	case Number:
		// no dot means it's a whole number.
		if strings.Index(curr.Value, ".") == -1 {
			bigint := new(big.Int)
			bigint.SetString(curr.Value, 10)

			return &ExpressionNode{
				Kind: ConstantExpression,
				ConstantNode: &ConstantNode{
					Kind:                IntegerConstant,
					IntegerConstantNode: &IntegerConstantNode{bigint},
				},
			}
		}

		val, err := strconv.ParseFloat(curr.Value, 64)
		if err != nil {
			panic(err)
		}

		return &ExpressionNode{
			Kind: ConstantExpression,
			ConstantNode: &ConstantNode{
				Kind:                 FloatingConstant,
				FloatingConstantNode: &FloatingConstantNode{val},
			},
		}
	case Identifier:
		return &ExpressionNode{
			Kind: ConstantExpression,
			ConstantNode: &ConstantNode{
				Kind:                  VariableReference,
				VariableReferenceNode: &VariableReferenceNode{curr},
			},
		}

	case Char:
		return &ExpressionNode{
			Kind: ConstantExpression,
			ConstantNode: &ConstantNode{
				Kind:                  CharacterConstant,
				CharacterConstantNode: &CharacterConstantNode{curr.Value},
			},
		}

	case String:
		return &ExpressionNode{
			Kind: ConstantExpression,
			ConstantNode: &ConstantNode{
				Kind:               StringConstant,
				StringConstantNode: &StringConstantNode{curr.Value},
			},
		}

	case EndOfFile:
		return nil

	default:
		p.error(api.NewUnimplementedError("parse", string(curr.Kind), start, p.pos))
		return nil
	}
}

func (p *astParser) parseBuiltin() *ExpressionNode {
	builtin := p.expectKind(Identifier)
	p.expect("!")

	parens := false
	if p.next().Matches("(") {
		parens = true
		p.consume()
	}

	// could be type though?
	// alloc!int
	ref := p.expectKind(Identifier)

	args := []*ExpressionNode{}

	if parens {
		for p.hasNext() {
			if p.next().Matches(",") {
				p.consume()
			}
			if p.next().Matches(")") {
				break
			}

			arg := p.parseExpression()
			if arg != nil {
				args = append(args, arg)
			}
		}
		p.expect(")")
	}

	return &ExpressionNode{
		Kind: BuiltinExpression,
		BuiltinExpressionNode: &BuiltinExpressionNode{
			builtin.Value,
			&VariableReferenceNode{
				ref,
			},
			args,
		},
	}
}

func (p *astParser) parseCall(left *ExpressionNode) *ExpressionNode {
	start := p.pos

	var params []*ExpressionNode

	p.expect("(")
	for idx := 0; p.hasNext() && !p.next().Matches(")"); idx++ {
		if idx != 0 {
			p.expect(",")
		}

		val := p.parseExpression()
		if val == nil {
			p.error(api.NewParseError("parameter in call expression", start, p.pos))
		}
		params = append(params, val)
	}
	p.expect(")")

	return &ExpressionNode{
		Kind: CallExpression,
		CallExpressionNode: &CallExpressionNode{
			left, params,
		},
	}
}

func (p *astParser) parseIndex(left *ExpressionNode) *ExpressionNode {
	start := p.pos
	p.expect("[")
	val := p.parseExpression()
	if val == nil {
		p.error(api.NewParseError("expression in array index", start, p.pos))
	}
	p.expect("]")
	return &ExpressionNode{
		Kind: IndexExpression,
		IndexExpressionNode: &IndexExpressionNode{
			left, val,
		},
	}
}

func (p *astParser) parseLambda() *ExpressionNode {
	proto := p.parseFunctionPrototypeDeclaration()
	body := p.parseStatBlock()
	return &ExpressionNode{
		Kind: LambdaExpression,
		LambdaExpressionNode: &LambdaExpressionNode{
			proto, body,
		},
	}
}

func (p *astParser) parseInitializer() *ExpressionNode {
	p.expect(":")
	lhand := p.expectKind(Identifier)

	p.expect("{")
	var els []*ExpressionNode
	for p.hasNext() {
		if p.next().Matches(",") {
			p.consume()
		}

		if p.next().Matches("}") {
			break
		}

		expr := p.parseExpression()
		if expr != nil {
			els = append(els, expr)
		}
	}
	p.expect("}")

	return &ExpressionNode{
		Kind: InitializerExpression,
		InitializerExpressionNode: &InitializerExpressionNode{
			Kind:   InitStructure,
			LHand:  lhand,
			Values: els,
		},
	}
}

func (p *astParser) parsePrimaryExpr() *ExpressionNode {
	if !p.hasNext() {
		return nil
	}

	// hm.
	if p.next().Matches(struc, "*", "[", "(") {
		typ := p.parseTypeExpression()
		return typ
	}

	// ambiguity with tuple types and their initializers?
	// initializer syntax should change
	if p.next().Matches(":") {
		init := p.parseInitializer()
		if init != nil {
			return init
		}
	}

	if p.next().Matches(fn) {
		return p.parseLambda()
	}

	if p.next().Matches(unaryOperators...) {
		return p.parseUnaryExpr()
	}

	// builtins.
	switch curr := p.next(); {
	case curr.Matches(builtins...):
		return p.parseBuiltin()
	}

	left := p.parseOperand()
	if left == nil {
		return nil
	}

	switch curr := p.next(); {
	case curr.Matches("["):
		return p.parseIndex(left)
	case curr.Matches("("):
		return p.parseCall(left)
	}

	return left
}

func (p *astParser) parseLeft() *ExpressionNode {
	if expr := p.parsePrimaryExpr(); expr != nil {
		return expr
	}
	return p.parseUnaryExpr()
}

var opPrec = map[string]int{
	"*": 5,
	"/": 5,
	"%": 5,

	"+": 4,
	"-": 4,

	"==": 3,
	"!=": 3,
	"<":  3,
	"<=": 3,
	">":  3,
	">=": 3,

	"&&": 2,

	"||": 1,
}

func getOpPrec(op string) int {
	if prec, ok := opPrec[op]; ok {
		return prec
	}
	return -1
}

func (p *astParser) parsePrec(lastPrec int, left *ExpressionNode) *ExpressionNode {
	for p.hasNext() {
		prec := getOpPrec(p.next().Value)
		if prec < lastPrec {
			return left
		}

		// FIXME.
		if !p.hasNext() {
			return left
		}

		// next op is not a binary
		if _, ok := opPrec[p.next().Value]; !ok {
			return left
		}

		op := p.consume()
		right := p.parsePrimaryExpr()
		if right == nil {
			return nil
		}

		if !p.hasNext() {
			return &ExpressionNode{
				Kind:                 BinaryExpression,
				BinaryExpressionNode: &BinaryExpressionNode{left, op.Value, right},
			}
		}

		nextPrec := getOpPrec(p.next().Value)
		if prec < nextPrec {
			right = p.parsePrec(prec+1, right)
			if right == nil {
				return nil
			}
		}

		left = &ExpressionNode{
			Kind: BinaryExpression,
			BinaryExpressionNode: &BinaryExpressionNode{
				left, op.Value, right,
			},
		}
	}

	return left
}

func (p *astParser) parseAssign(left *ExpressionNode) *ExpressionNode {
	if !p.hasNext() {
		return nil
	}

	start := p.pos
	op := p.consume()

	right := p.parseExpression()
	if right == nil {
		p.error(api.NewParseError("expression after assignment operator", start, p.pos))
	}

	return &ExpressionNode{
		Kind: AssignStatement,
		AssignStatementNode: &AssignStatementNode{
			left, op.Value, right,
		},
	}
}

func (p *astParser) parseDotList(left *ExpressionNode) *ExpressionNode {
	start := p.pos
	list := []*ExpressionNode{}

	list = append(list, left)

	for p.hasNext() && p.next().Matches(".") {
		p.expect(".")
		val := p.parseExpression()
		if val == nil {
			p.error(api.NewParseError("expression in dot-list", start, p.pos))
		}
		list = append(list, val)
	}

	return &ExpressionNode{
		Kind:               PathExpression,
		PathExpressionNode: &PathExpressionNode{list},
	}
}

var assignOperators = []string{
	"=", "+=", "-=", "*=", "/=",
}
var unaryOperators = []string{
	"-", "!", "+", "@", "&", "~",
}
var builtins = []string{
	"alloc", "sizeof", "len", "free",
	"move", "ref",
}

func (p *astParser) parseExpression() *ExpressionNode {
	left := p.parseLeft()
	if left == nil {
		return nil
	}

	if p.next().Matches(".") {
		return p.parseDotList(left)
	}

	if p.next().Matches(assignOperators...) {
		return p.parseAssign(left)
	}

	if _, ok := opPrec[p.next().Value]; ok {
		return p.parsePrec(0, left)
	}
	return left
}

func (p *astParser) parseExpressionStatement() *ParseTreeNode {
	expr := p.parseExpression()
	if expr != nil {
		return &ParseTreeNode{
			Kind:                    ExpressionStatement,
			ExpressionStatementNode: expr,
		}
	}

	return nil
}

func (p *astParser) skipDirective() {
	p.expect("#")
	p.expect("{")
	for p.hasNext() && !p.next().Matches("}") {
		p.consume()
	}
	p.expect("}")
}

// parseNode returns whether the node was parsed
// with error or not, as well as the node. for example
// parsing a comment returns a nil node, but the
// parse was OK to do. however an error returns a nil node
// but it was not OK
func (p *astParser) parseNode() (*ParseTreeNode, bool) {
	start := p.pos
	startingTok := p.next()

	res := &ParseTreeNode{}

	switch curr := p.next(); {
	case curr.Matches("#"):
		p.skipDirective()
		return nil, true

	case curr.Matches(trait):
		res.TraitDeclaration = p.parseTraitDeclaration()
		res.Kind = TraitDeclStatement
	case curr.Matches(impl):
		res.ImplDeclaration = p.parseImplDeclaration()
		res.Kind = ImplDeclStatement
	case curr.Matches(fn):
		res.FunctionDeclaration = p.parseFunctionDeclaration()
		res.Kind = FunctionDeclStatement

	case curr.Matches(typ):
		res = p.parseTypeAlias()
		p.expect(";")

	case curr.Matches(mut):
		res = p.parseMut()
		p.expect(";")

	case curr.Matches(let):
		res = p.parseLet()
		p.expect(";")

	default:
		res = p.parseExpressionStatement()
		if res == nil {
			p.error(api.NewUnimplementedError("parse", startingTok.Value, start, p.pos))
		}
		p.expect(";")
	}

	return res, true
}

func ParseTokenStream(stream []Token) ([]*ParseTreeNode, []api.CompilerError) {
	p := &astParser{parser{stream, 0, []api.CompilerError{}}}
	fmt.Println("parsing ...")

	nodes := []*ParseTreeNode{}
	for p.hasNext() {
		node, ok := p.parseNode()
		if !ok {
			break
		}
		if node != nil {
			nodes = append(nodes, node)
		}
	}
	return nodes, p.errors
}
