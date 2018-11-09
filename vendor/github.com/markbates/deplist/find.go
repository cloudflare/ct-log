package deplist

import (
	"go/build"
	"path/filepath"
	"sort"
	"strings"

	"github.com/markbates/oncer"
	"github.com/pkg/errors"
)

// // FindGoFiles *.go files for a given diretory
// func FindGoFiles(dir string) ([]string, error) {
// 	var err error
// 	var names []string
// 	oncer.Do("FindGoFiles"+dir, func() {
//
// 		callback := func(path string, do *godirwalk.Dirent) error {
// 			ext := filepath.Ext(path)
// 			if ext != ".go" {
// 				return nil
// 			}
// 			names = append(names, path)
// 			return nil
// 		}
// 		err = godirwalk.Walk(dir, &godirwalk.Options{
// 			FollowSymbolicLinks: true,
// 			Callback:            callback,
// 		})
// 	})
//
// 	return names, err
// }

// FindImports
func FindImports(dir string, mode build.ImportMode) ([]string, error) {
	var err error
	var names []string
	oncer.Do("FindImports"+dir, func() {
		ctx := build.Default

		if len(ctx.SrcDirs()) == 0 {
			err = errors.New("no src directories found")
			return
		}

		pkg, err := ctx.ImportDir(dir, mode)

		if err != nil {
			if !strings.Contains(err.Error(), "cannot find package") {
				if _, ok := errors.Cause(err).(*build.NoGoError); !ok {
					err = errors.WithStack(err)
					return
				}
			}
		}

		if pkg.ImportPath == "." {
			return
		}
		if pkg.Goroot {
			return
		}
		if len(pkg.GoFiles) <= 0 {
			return
		}

		nm := map[string]string{}
		nm[pkg.ImportPath] = pkg.ImportPath
		for _, imp := range pkg.Imports {
			if len(ctx.SrcDirs()) == 0 {
				continue
			}
			// nm[imp] = imp
			d := ctx.SrcDirs()[len(ctx.SrcDirs())-1]
			ip := filepath.Join(d, imp)
			n, err := FindImports(ip, mode)
			if err != nil {
				continue
			}
			for _, x := range n {
				nm[x] = x
			}
		}

		var ns []string
		for k := range nm {
			ns = append(ns, k)
		}
		sort.Strings(ns)
		names = ns
	})
	return names, err
}
