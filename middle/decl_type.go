package middle

import (
	"reflect"

	"github.com/krug-lang/krugc-api/api"
	"github.com/krug-lang/krugc-api/ir"
)

type decl struct {
	mod    *ir.Module
	errors []api.CompilerError
	curr   *ir.SymbolTable
}

func (d *decl) error(e api.CompilerError) {
	d.errors = append(d.errors, e)
}

func (d *decl) push(stab *ir.SymbolTable) {
	d.curr = stab
}

func (d *decl) pop() {
	d.curr = d.curr.Outer
}

func (d *decl) regType(name string, t ir.Type) {
	d.curr.RegisterType(name, t)
}

func (d *decl) visitLocal(l *ir.Local) {
	if l.Type == nil {
		d.error(api.NewUnimplementedError("type inference"))
		return
	}

	// if the type its a reference type,
	// try and link this to the type it references.
	if refType, ok := l.Type.(*ir.ReferenceType); ok {
		name := refType.Name

		if structure, ok := d.mod.Structures[name]; ok {
			l.Type = structure
		} else {
			// couldn't find reference type.
		}
	}

	d.regType(l.Name.Value, l.Type)
}

func (d *decl) visitAlloca(a *ir.Alloca) {
	if a.Type == nil {
		d.error(api.NewUnimplementedError("type inference"))
		return
	}
	d.regType(a.Name.Value, a.Type)
}

func (d *decl) visitInstr(i ir.Instruction) {
	switch instr := i.(type) {
	case *ir.Block:
		d.visitBlock(instr)

	case *ir.Local:
		d.visitLocal(instr)
	case *ir.Alloca:
		d.visitAlloca(instr)

	case *ir.Path:
		return
	case *ir.Return:
		return

	default:
		d.error(api.NewUnimplementedError("visitInstr: " + reflect.TypeOf(instr).String()))
	}
}

func (d *decl) visitBlock(b *ir.Block) {
	d.push(b.Stab)
	for _, instr := range b.Instr {
		d.visitInstr(instr)
	}
	d.pop()
}

func declType(mod *ir.Module) (*ir.Module, []api.CompilerError) {
	d := &decl{
		mod,
		[]api.CompilerError{},
		nil,
	}

	d.push(mod.Root)

	for _, name := range mod.FunctionOrder {
		fn, _ := mod.Functions[name.Value]
		d.push(fn.Stab)
		for _, instr := range fn.Body.Instr {
			d.visitInstr(instr)
		}
		d.pop()
	}

	return d.mod, d.errors
}
