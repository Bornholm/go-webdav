package expr

import (
	"reflect"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/builtin"
	"github.com/expr-lang/expr/conf"
)

var functions = []builtin.Function{
	{
		Name: "isRead",
		Func: func(args ...any) (any, error) {
			return nil, nil
		},
		Types: []reflect.Type{
			reflect.TypeFor[int](),
		},
	},
}

func WithRuleAPI() expr.Option {
	return func(c *conf.Config) {
		for _, fn := range functions {
			c.Functions[fn.Name] = &fn
		}
	}
}
