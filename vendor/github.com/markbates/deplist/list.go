package deplist

import (
	"go/build"
	"os"

	"github.com/markbates/oncer"
	"github.com/pkg/errors"
)

func List(skips ...string) (map[string]string, error) {
	oncer.Deprecate(0, "deplist.List", "Use deplist.FindImports instead")
	deps := map[string]string{}
	pwd, err := os.Getwd()
	if err != nil {
		return deps, errors.WithStack(err)
	}
	pkgs, err := FindImports(pwd, build.IgnoreVendor)
	if err != nil {
		return deps, errors.WithStack(err)
	}
	for _, p := range pkgs {
		deps[p] = p
	}

	return deps, nil
}
