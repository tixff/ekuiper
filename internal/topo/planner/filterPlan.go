package planner

import "github.com/lf-edge/ekuiper/pkg/ast"

type FilterPlan struct {
	baseLogicalPlan
	condition ast.Expr
}

func (p FilterPlan) Init() *FilterPlan {
	p.baseLogicalPlan.self = &p
	return &p
}

func (p *FilterPlan) PushDownPredicate(condition ast.Expr) (ast.Expr, LogicalPlan) {
	// if no child, swallow all conditions
	a := combine(condition, p.condition)
	if len(p.children) == 0 {
		p.condition = a
		return nil, p
	}

	rest, _ := p.baseLogicalPlan.PushDownPredicate(a)

	if rest != nil {
		p.condition = rest
		return nil, p
	} else if len(p.children) == 1 {
		// eliminate this filter
		return nil, p.children[0]
	} else {
		return nil, p
	}
}

func (p *FilterPlan) PruneColumns(fields []ast.Expr) error {
	f := getFields(p.condition)
	return p.baseLogicalPlan.PruneColumns(append(fields, f...))
}
