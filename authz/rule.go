package authz

type Rules interface {
	Rules() []Rule
}

type Rule interface {
	Exec(env map[string]any) (bool, error)
}
