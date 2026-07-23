package devslog

import (
	"fmt"
	"reflect"
	"runtime"
)

func (h *developHandler) getFileLineFromPC(pcs []uintptr) (fileLines []string) {
	if len(pcs) == 0 {
		return nil
	}

	frames := runtime.CallersFrames(pcs[:])
	for {
		fr, more := frames.Next()
		fileLines = append(fileLines, fmt.Sprintf("%v:%v", fr.File, fr.Line))
		if !more {
			break
		}
	}

	return fileLines
}

// extractPCFromError tries to extract StackTrace PC frames from errors created by
// - github.com/pkg/errors
// - golang.org/x/xerrors
// - golang.org/x/exp/errors
func (h *developHandler) extractPCFromError(err error) (pc []uintptr) {
	if h.opts.MaxErrorStackTrace == 0 {
		return nil
	}

	v := reflect.ValueOf(err)
	pc = h.extractPCFromPkgErrors(v)
	if len(pc) > 0 {
		return pc
	}

	pc = h.extractPCFromExpErrors(v)
	if len(pc) > 0 {
		return pc
	}

	return pc
}

func (h *developHandler) extractPCFromPkgErrors(v reflect.Value) (pc []uintptr) {
	// https://github.com/pkg/errors/blob/master/stack.go#L155
	//
	// type stackTracer interface {
	//   StackTrace() StackTrace
	// }
	// type StackTrace []Frame
	// type Frame uintptr

	v = v.MethodByName("StackTrace")
	if !v.IsValid() {
		return nil
	}

	v = v.Call(nil)[0]
	if v.Kind() != reflect.Slice {
		return nil
	}

	// Get up to two frames from github.com/pkg/errors StackTrace.
	for i := 0; i < min(v.Len(), int(h.opts.MaxErrorStackTrace)); i++ {
		index := v.Index(i)
		if !index.CanUint() {
			return pc
		}

		pc = append(pc, uintptr(index.Uint()))
	}

	return pc
}

func (h *developHandler) extractPCFromExpErrors(v reflect.Value) (pc []uintptr) {
	// https://cs.opensource.google/go/x/exp/+/92128663:errors/fmt/errors.go;l=24
	// https://cs.opensource.google/go/x/exp/+/92128663:errors/errors.go;l=25
	//
	// type noWrapError struct {
	// 	 msg   string
	// 	 err   error
	// 	 frame errors.Frame
	// }
	//
	// type wrapError struct {
	//   msg   string
	//   err   error
	//   frame errors.Frame
	// }
	//
	// type errorString struct {
	//   s     string
	//   frame Frame
	// }
	if v.Kind() == reflect.Ptr && !v.IsNil() {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	v = v.FieldByName("frame")
	if v.Kind() != reflect.Struct {
		return nil
	}

	// https://cs.opensource.google/go/x/exp/+/92128663:errors/frame.go;l=12
	//
	// type Frame struct {
	// 	 frames [3]uintptr
	// }
	v = v.FieldByName("frames")
	if v.Kind() != reflect.Array {
		return nil
	}

	// Skip first frame pointing at fmt.Errorf() or errors.New().
	skip := 1
	for i := skip; i < min(v.Len(), skip+int(h.opts.MaxErrorStackTrace)); i++ {
		index := v.Index(i)
		if !index.CanUint() {
			return nil
		}

		pc = append(pc, uintptr(index.Uint()))
	}

	return pc
}
