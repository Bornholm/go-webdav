package authz

type User interface {
	Attrs() map[string]any
	Rules() []Rule
	Groups() []*Group
}

type Group struct {
	name  string
	rules []Rule
}

func (g *Group) Name() string {
	return g.name
}

// Rules implements Rules.
func (g *Group) Rules() []Rule {
	return g.rules
}

func NewGroup(name string, rules ...Rule) *Group {
	return &Group{name, rules}
}

var _ Rules = &Group{}
