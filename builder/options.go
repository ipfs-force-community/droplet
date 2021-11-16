package builder

import (
	"context"
	"golang.org/x/xerrors"
	"reflect"
	"sort"

	"go.uber.org/fx"
)

type Special struct{ ID int }

type Invoke int

var MaxInvoke Invoke

func NextInvoke() Invoke {
	MaxInvoke++
	return MaxInvoke
}

// Option is a functional option which can be used with the New function to
// change how the node is constructed
//
// Options are applied in sequence
type Option func(*Settings) error

// Options groups multiple options into one
func Options(opts ...Option) Option {
	return func(s *Settings) error {
		for _, opt := range opts {
			if err := opt(s); err != nil {
				return err
			}
		}
		return nil
	}
}

// Error is a special option which returns an error when applied
func Error(err error) Option {
	return func(_ *Settings) error {
		return err
	}
}

func ApplyIf(check func(s *Settings) bool, opts ...Option) Option {
	return func(s *Settings) error {
		if check(s) {
			return Options(opts...)(s)
		}
		return nil
	}
}

func ApplyIfElse(check func(s *Settings) bool, ifOpt Option, elseOpt Option) Option {
	return func(s *Settings) error {
		if check(s) {
			return Options(ifOpt)(s)
		} else {
			return Options(elseOpt)(s)
		}
		return nil
	}
}

func If(b bool, opts ...Option) Option {
	return ApplyIf(func(s *Settings) bool {
		return b
	}, opts...)
}

// Override option changes constructor for a given type
func Override(typ, constructor interface{}) Option {
	return func(s *Settings) error {
		if key, ok := typ.(Invoke); ok {
			s.Invokes[key] = InvokeOption{
				Priority: key,
				Option:   fx.Invoke(constructor),
			}
			return nil
		}

		if c, ok := typ.(Special); ok {
			s.Modules[c] = fx.Provide(constructor)
			return nil
		}
		ctor := as(constructor, typ)
		rt := reflect.TypeOf(typ).Elem()

		s.Modules[rt] = fx.Provide(ctor)
		return nil
	}
}

func Unset(typ interface{}) Option {
	return func(s *Settings) error {
		if i, ok := typ.(Invoke); ok {
			delete(s.Invokes, i)
			return nil
		}

		if c, ok := typ.(Special); ok {
			delete(s.Modules, c)
			return nil
		}
		rt := reflect.TypeOf(typ).Elem()

		delete(s.Modules, rt)
		return nil
	}
}

// From(*T) -> func(t T) T {return t}
func From(typ interface{}) interface{} {
	rt := []reflect.Type{reflect.TypeOf(typ).Elem()}
	ft := reflect.FuncOf(rt, rt, false)
	return reflect.MakeFunc(ft, func(args []reflect.Value) (results []reflect.Value) {
		return args
	}).Interface()
}

// from go-ipfs
// as casts input constructor to a given interface (if a value is given, it
// wraps it into a constructor).
//
// Note: this method may look like a hack, and in fact it is one.
// This is here only because https://github.com/uber-go/fx/issues/673 wasn't
// released yet
//
// Note 2: when making changes here, make sure this method stays at
// 100% coverage. This makes it less likely it will be terribly broken
func as(in interface{}, as interface{}) interface{} {
	outType := reflect.TypeOf(as)

	if outType.Kind() != reflect.Ptr {
		panic("outType is not a pointer " + outType.String())
	}

	if reflect.TypeOf(in).Kind() != reflect.Func {
		ctype := reflect.FuncOf(nil, []reflect.Type{outType.Elem()}, false)

		return reflect.MakeFunc(ctype, func(args []reflect.Value) (results []reflect.Value) {
			out := reflect.New(outType.Elem())
			out.Elem().Set(reflect.ValueOf(in))

			return []reflect.Value{out.Elem()}
		}).Interface()
	}

	inType := reflect.TypeOf(in)

	ins := make([]reflect.Type, inType.NumIn())
	outs := make([]reflect.Type, inType.NumOut())

	for i := range ins {
		ins[i] = inType.In(i)
	}
	outs[0] = outType.Elem()
	for i := range outs[1:] {
		outs[i+1] = inType.Out(i + 1)
	}

	ctype := reflect.FuncOf(ins, outs, false)

	return reflect.MakeFunc(ctype, func(args []reflect.Value) (results []reflect.Value) {
		outs := reflect.ValueOf(in).Call(args)

		out := reflect.New(outType.Elem())
		if outs[0].Type().AssignableTo(outType.Elem()) {
			// Out: Iface = In: *Struct; Out: Iface = In: OtherIface
			out.Elem().Set(outs[0])
		} else {
			// Out: Iface = &(In: Struct)
			t := reflect.New(outs[0].Type())
			t.Elem().Set(outs[0])
			out.Elem().Set(t)
		}
		outs[0] = out.Elem()

		return outs
	}).Interface()
}

type StopFunc func(context.Context) error

// New builds and starts new Filecoin node
func New(ctx context.Context, opts ...Option) (StopFunc, error) {
	settings := Settings{
		Modules: map[interface{}]fx.Option{},
		Invokes: map[Invoke]InvokeOption{},
	}

	// apply module options in the right order
	if err := Options(opts...)(&settings); err != nil {
		return nil, xerrors.Errorf("applying node options failed: %w", err)
	}

	// gather constructors for fx.Options
	ctors := make([]fx.Option, 0, len(settings.Modules))
	for _, opt := range settings.Modules {
		ctors = append(ctors, opt)
	}

	// fill holes in invokes for use in fx.Options
	invokeOpts := []InvokeOption{}
	for _, opt := range settings.Invokes {
		invokeOpts = append(invokeOpts, opt)
	}

	sort.Slice(invokeOpts, func(i, j int) bool {
		return invokeOpts[i].Priority < invokeOpts[j].Priority
	})

	invokes := []fx.Option{}
	for _, opt := range invokeOpts {
		invokes = append(invokes, opt.Option)
	}

	app := fx.New(
		fx.Options(ctors...),
		fx.Options(invokes...),

		fx.Logger(&debugPrinter{}),
	)

	// TODO: we probably should have a 'firewall' for Closing signal
	//  on this context, and implement closing logic through lifecycles
	//  correctly
	if err := app.Start(ctx); err != nil {
		// comment fx.NopLogger few lines above for easier debugging
		return nil, xerrors.Errorf("starting node: %w", err)
	}

	return app.Stop, nil
}

type InvokeOption struct {
	Priority Invoke
	Option   fx.Option
}

type Settings struct {
	// modules is a map of constructors for DI
	//
	// In most cases the index will be a reflect. Type of element returned by
	// the constructor, but for some 'constructors' it's hard to specify what's
	// the return type should be (or the constructor returns fx group)
	Modules map[interface{}]fx.Option

	// invokes are separate from modules as they can't be referenced by return
	// type, and must be applied in correct order
	Invokes map[Invoke]InvokeOption

	Base   bool // Base option applied
	Config bool // Config option applied
}
