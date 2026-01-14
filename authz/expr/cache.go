package expr

import (
	"time"

	"github.com/bornholm/go-webdav/syncx"
	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/vm"
	"github.com/pkg/errors"
)

var defaultCache = NewCache(time.Hour)

type cachedProgram struct {
	Program *vm.Program
	Expires time.Time
}

type Cache struct {
	ttl      time.Duration
	programs syncx.Map[string, cachedProgram]
}

func (c *Cache) Get(script string) (*vm.Program, error) {
	now := time.Now()
	cached, ok := c.programs.Load(script)
	if ok && cached.Expires.After(now) {
		return cached.Program, nil
	}

	program, err := expr.Compile(script, expr.AsBool(), WithRuleAPI())
	if err != nil {
		return nil, errors.WithStack(err)
	}

	c.programs.Store(script, cachedProgram{
		Program: program,
		Expires: time.Now().Add(c.ttl),
	})

	return program, nil
}

func NewCache(ttl time.Duration) *Cache {
	return &Cache{ttl: ttl}
}
