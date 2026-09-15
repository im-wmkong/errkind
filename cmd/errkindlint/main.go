// Command errkindlint 静态扫描 errkind.Define 调用, 检查 (code, name) 冲突。
//
// 用法:
//
//	errkindlint [-exclude=glob] [path ...]
//
// 不传路径时扫描当前目录; 发现冲突时退出码为 1。
// -exclude 可重复, 用 filepath.Match 风格匹配文件路径 (例: "*/examples/*")。
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/im-wmkong/errkind/internal/lint"
	"github.com/im-wmkong/errkind/internal/scan"
)

type stringSlice []string

func (s *stringSlice) String() string     { return strings.Join(*s, ",") }
func (s *stringSlice) Set(v string) error { *s = append(*s, v); return nil }

func main() {
	var excludes stringSlice
	flag.Var(&excludes, "exclude", "glob to skip files (filepath.Match; repeatable)")
	flag.Parse()

	dirs := flag.Args()
	if len(dirs) == 0 {
		dirs = []string{"."}
	}
	dirs = expandEllipsis(dirs)

	defs, scanErrs := scan.ScanDirs(dirs, excludes...)
	for _, e := range scanErrs {
		fmt.Fprintln(os.Stderr, "errkindlint:", e)
	}
	issues := lint.Check(defs)
	for _, is := range issues {
		fmt.Printf("%s: %s\n", is.Pos, is.Message)
	}
	if len(issues) > 0 || len(scanErrs) > 0 {
		os.Exit(1)
	}
}

// expandEllipsis 把 Go 风格的 "dir/..." 归一为 "dir"; 扫描器本身递归, 直接去掉后缀即可。
func expandEllipsis(in []string) []string {
	out := make([]string, 0, len(in))
	for _, p := range in {
		switch {
		case p == "./..." || p == "...":
			out = append(out, ".")
		case strings.HasSuffix(p, "/..."):
			out = append(out, strings.TrimSuffix(p, "/..."))
		default:
			out = append(out, p)
		}
	}
	return out
}
