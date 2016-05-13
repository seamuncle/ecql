package ecql

type PredicateType int

const (
	EqPredicate PredicateType = iota
	GtPredicate
	GePredicate
	LtPredicate
	LePredicate
	InPredicate
)

type Condition struct {
	Column    string
	Predicate PredicateType
	Value     interface{}
	Values    []interface{}
}

func Eq(col string, v interface{}) Condition {
	return Condition{col, EqPredicate, v, nil}
}

func Gt(col string, v interface{}) Condition {
	return Condition{col, GtPredicate, v, nil}
}

func Ge(col string, v interface{}) Condition {
	return Condition{col, GePredicate, v, nil}
}

func Lt(col string, v interface{}) Condition {
	return Condition{col, LtPredicate, v, nil}
}

func Le(col string, v interface{}) Condition {
	return Condition{col, LePredicate, v, nil}
}

func In(col string, v ...interface{}) Condition {
	return Condition{col, EqPredicate, nil, v}
}
