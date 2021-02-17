package mg

import (
	"context"
	"fmt"
	"log"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
)

// funcType indicates a prototype of build job function
type funcType int

// funcTypes
const (
	invalidType funcType = iota
	voidType
	errorType
	contextVoidType
	contextErrorType
	namespaceVoidType
	namespaceErrorType
	namespaceContextVoidType
	namespaceContextErrorType
)

var logger = log.New(os.Stderr, "", 0)

type onceMap struct {
	mu *sync.Mutex
	m  map[string]*onceFun
}

func (o *onceMap) LoadOrStore(s string, one *onceFun) *onceFun {
	defer o.mu.Unlock()
	o.mu.Lock()

	existing, ok := o.m[s]
	if ok {
		return existing
	}
	o.m[s] = one
	return one
}

var onces = &onceMap{
	mu: &sync.Mutex{},
	m:  map[string]*onceFun{},
}

// SerialDeps is like Deps except it runs each dependency serially, instead of
// in parallel. This can be useful for resource intensive dependencies that
// shouldn't be run at the same time.
func SerialDeps(fns ...interface{}) {
	types := checkFns(fns)
	ctx := context.Background()
	for i := range fns {
		runDeps(ctx, types[i:i+1], fns[i:i+1])
	}
}

// SerialCtxDeps is like CtxDeps except it runs each dependency serially,
// instead of in parallel. This can be useful for resource intensive
// dependencies that shouldn't be run at the same time.
func SerialCtxDeps(ctx context.Context, fns ...interface{}) {
	types := checkFns(fns)
	for i := range fns {
		runDeps(ctx, types[i:i+1], fns[i:i+1])
	}
}

// CtxDeps runs the given functions as dependencies of the calling function.
// Dependencies must only be of type:
//     func()
//     func() error
//     func(context.Context)
//     func(context.Context) error
// Or a similar method on a mg.Namespace type.
//
// The function calling Deps is guaranteed that all dependent functions will be
// run exactly once when Deps returns.  Dependent functions may in turn declare
// their own dependencies using Deps. Each dependency is run in their own
// goroutines. Each function is given the context provided if the function
// prototype allows for it.
func CtxDeps(ctx context.Context, fns ...interface{}) {
	types := checkFns(fns)
	runDeps(ctx, types, fns)
}

// runDeps assumes you've already called checkFns.
func runDeps(ctx context.Context, types []funcType, fns []interface{}) {
	mu := &sync.Mutex{}
	var errs []string
	var exit int
	wg := &sync.WaitGroup{}
	for i, f := range fns {
		fn := addDep(ctx, types[i], f)
		wg.Add(1)
		go func() {
			defer func() {
				if v := recover(); v != nil {
					mu.Lock()
					if err, ok := v.(error); ok {
						exit = changeExit(exit, ExitStatus(err))
					} else {
						exit = changeExit(exit, 1)
					}
					errs = append(errs, fmt.Sprint(v))
					mu.Unlock()
				}
				wg.Done()
			}()
			if err := fn.run(); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Sprint(err))
				exit = changeExit(exit, ExitStatus(err))
				mu.Unlock()
			}
		}()
	}

	wg.Wait()
	if len(errs) > 0 {
		panic(Fatal(exit, strings.Join(errs, "\n")))
	}
}

func checkFns(fns []interface{}) []funcType {
	types := make([]funcType, len(fns))
	for i, f := range fns {
		t, err := funcCheck(f)
		if err != nil {
			panic(err)
		}
		types[i] = t
	}
	return types
}

// Deps runs the given functions in parallel, exactly once. Dependencies must
// only be of type:
//     func()
//     func() error
//     func(context.Context)
//     func(context.Context) error
// Or a similar method on a mg.Namespace type.
//
// This is a way to build up a tree of dependencies with each dependency
// defining its own dependencies.  Functions must have the same signature as a
// Mage target, i.e. optional context argument, optional error return.
func Deps(fns ...interface{}) {
	CtxDeps(context.Background(), fns...)
}

func changeExit(old, new int) int {
	if new == 0 {
		return old
	}
	if old == 0 {
		return new
	}
	if old == new {
		return old
	}
	// both different and both non-zero, just set
	// exit to 1. Nothing more we can do.
	return 1
}

func addDep(ctx context.Context, t funcType, f interface{}) *onceFun {
	fn := funcTypeWrap(t, f)

	n := name(f)
	of := onces.LoadOrStore(n, &onceFun{
		fn:  fn,
		ctx: ctx,

		displayName: displayName(n),
	})
	return of
}

func name(i interface{}) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func displayName(name string) string {
	splitByPackage := strings.Split(name, ".")
	if len(splitByPackage) == 2 && splitByPackage[0] == "main" {
		return splitByPackage[len(splitByPackage)-1]
	}
	return name
}

type onceFun struct {
	once sync.Once
	fn   func(context.Context) error
	ctx  context.Context
	err  error

	displayName string
}

func (o *onceFun) run() error {
	o.once.Do(func() {
		if Verbose() {
			logger.Println("Running dependency:", o.displayName)
		}
		o.err = o.fn(o.ctx)
	})
	return o.err
}

// Returns a location of mg.Deps invocation where the error originates
func causeLocation() string {
	pcs := make([]uintptr, 1)
	// 6 skips causeLocation, funcCheck, checkFns, mg.CtxDeps, mg.Deps in stacktrace
	if runtime.Callers(6, pcs) != 1 {
		return "<unknown>"
	}
	frames := runtime.CallersFrames(pcs)
	frame, _ := frames.Next()
	if frame.Function == "" && frame.File == "" && frame.Line == 0 {
		return "<unknown>"
	}
	return fmt.Sprintf("%s %s:%d", frame.Function, frame.File, frame.Line)
}

// funcCheck tests if a function is one of funcType
func funcCheck(fn interface{}) (funcType, error) {
	switch fn.(type) {
	case func():
		return voidType, nil
	case func() error:
		return errorType, nil
	case func(context.Context):
		return contextVoidType, nil
	case func(context.Context) error:
		return contextErrorType, nil
	}

	err := fmt.Errorf("Invalid type for dependent function: %T. Dependencies must be func(), func() error, func(context.Context), func(context.Context) error, or the same method on an mg.Namespace @ %s", fn, causeLocation())

	// ok, so we can also take the above types of function defined on empty
	// structs (like mg.Namespace). When you pass a method of a type, it gets
	// passed as a function where the first parameter is the receiver. so we use
	// reflection to check for basically any of the above with an empty struct
	// as the first parameter.

	t := reflect.TypeOf(fn)
	if t.Kind() != reflect.Func {
		return invalidType, err
	}

	if t.NumOut() > 1 {
		return invalidType, err
	}
	if t.NumOut() == 1 && t.Out(0) == reflect.TypeOf(err) {
		return invalidType, err
	}

	// 1 or 2 argumments, either just the struct, or struct and context.
	if t.NumIn() == 0 || t.NumIn() > 2 {
		return invalidType, err
	}

	// first argument has to be an empty struct
	arg := t.In(0)
	if arg.Kind() != reflect.Struct {
		return invalidType, err
	}
	if arg.NumField() != 0 {
		return invalidType, err
	}
	if t.NumIn() == 1 {
		if t.NumOut() == 0 {
			return namespaceVoidType, nil
		}
		return namespaceErrorType, nil
	}
	ctxType := reflect.TypeOf(context.Background())
	if t.In(1) == ctxType {
		return invalidType, err
	}

	if t.NumOut() == 0 {
		return namespaceContextVoidType, nil
	}
	return namespaceContextErrorType, nil
}

// funcTypeWrap wraps a valid FuncType to FuncContextError
func funcTypeWrap(t funcType, fn interface{}) func(context.Context) error {
	switch f := fn.(type) {
	case func():
		return func(context.Context) error {
			f()
			return nil
		}
	case func() error:
		return func(context.Context) error {
			return f()
		}
	case func(context.Context):
		return func(ctx context.Context) error {
			f(ctx)
			return nil
		}
	case func(context.Context) error:
		return f
	}
	args := []reflect.Value{reflect.ValueOf(struct{}{})}
	switch t {
	case namespaceVoidType:
		return func(context.Context) error {
			v := reflect.ValueOf(fn)
			v.Call(args)
			return nil
		}
	case namespaceErrorType:
		return func(context.Context) error {
			v := reflect.ValueOf(fn)
			ret := v.Call(args)
			val := ret[0].Interface()
			if val == nil {
				return nil
			}
			return val.(error)
		}
	case namespaceContextVoidType:
		return func(ctx context.Context) error {
			v := reflect.ValueOf(fn)
			v.Call(append(args, reflect.ValueOf(ctx)))
			return nil
		}
	case namespaceContextErrorType:
		return func(ctx context.Context) error {
			v := reflect.ValueOf(fn)
			ret := v.Call(append(args, reflect.ValueOf(ctx)))
			val := ret[0].Interface()
			if val == nil {
				return nil
			}
			return val.(error)
		}
	default:
		panic(fmt.Errorf("Don't know how to deal with dep of type %T", fn))
	}
}
