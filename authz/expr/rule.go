package expr

import (
	"os"

	"github.com/bornholm/go-webdav/authz"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/pkg/errors"
)

type Rule struct {
	script string
}

// Exec implements authz.Rule.
func (r *Rule) Exec(env map[string]any) (bool, error) {
	program, err := r.getProgram()
	if err != nil {
		return false, errors.WithStack(err)
	}

	env["O_APPEND"] = os.O_APPEND
	env["O_RDONLY"] = os.O_RDONLY
	env["O_WRONLY"] = os.O_WRONLY
	env["O_RDWR"] = os.O_RDWR
	env["O_CREATE"] = os.O_CREATE
	env["O_EXCL"] = os.O_EXCL
	env["O_SYNC"] = os.O_SYNC
	env["O_TRUNC"] = os.O_TRUNC

	// Meta
	env["O_WRITE"] = os.O_WRONLY | os.O_APPEND | os.O_RDWR | os.O_TRUNC | os.O_CREATE

	result, err := expr.Run(program, env)
	if err != nil {
		return false, errors.WithStack(err)
	}

	allowed, ok := result.(bool)
	if !ok {
		return false, errors.Errorf("unexpected rule '%s' result type '%T', expected boolean", r.script, result)
	}

	return allowed, nil
}

func (r *Rule) getProgram() (*vm.Program, error) {
	program, err := defaultCache.Get(r.script)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return program, nil
}

func (r *Rule) String() string {
	return r.script
}

func NewRule(script string) *Rule {
	return &Rule{script: script}
}

var _ authz.Rule = &Rule{}
