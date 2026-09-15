package errkind

import (
	"runtime"
	"strconv"
)

// Frame 是调用栈一帧。
type Frame struct {
	Function string
	File     string
	Line     int
}

// String 输出 "package.Func\n\tfile:line"。
func (f Frame) String() string {
	return f.Function + "\n\t" + f.File + ":" + strconv.Itoa(f.Line)
}

// Tracer 表示能提供调用栈的错误。
type Tracer interface {
	StackTrace() []Frame
}

// capturePCs 抓取当前调用栈的 PC 列表; skip 表示需要跳过的栈帧数 (含 capturePCs 自身)。
func capturePCs(skip int) []uintptr {
	const depth = 32
	var pcs [depth]uintptr
	n := runtime.Callers(skip, pcs[:])
	if n == 0 {
		return nil
	}
	out := make([]uintptr, n)
	copy(out, pcs[:n])
	return out
}

// resolveFrames 把 PC 列表解析成可读 Frame; 用延迟解析是因为大多数错误从不被打印。
func resolveFrames(pcs []uintptr) []Frame {
	if len(pcs) == 0 {
		return nil
	}
	frames := runtime.CallersFrames(pcs)
	out := make([]Frame, 0, len(pcs))
	for {
		f, more := frames.Next()
		out = append(out, Frame{Function: f.Function, File: f.File, Line: f.Line})
		if !more {
			break
		}
	}
	return out
}

// StackOf 返回错误树中第一个非空调用栈。
func StackOf(err error) []Frame {
	var frames []Frame
	walk(err, func(cur error) bool {
		if e, ok := cur.(*kerr); ok {
			frames = resolveFrames(e.pcs)
		} else if tracer, ok := cur.(Tracer); ok {
			frames = tracer.StackTrace()
		}
		return len(frames) > 0
	})
	return frames
}

func hasStack(err error) bool {
	return walk(err, func(cur error) bool {
		if e, ok := cur.(*kerr); ok {
			return len(e.pcs) > 0
		}
		if tracer, ok := cur.(Tracer); ok {
			return len(tracer.StackTrace()) > 0
		}
		return false
	})
}
