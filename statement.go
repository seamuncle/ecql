package ecql

import (
	"fmt"
	"strings"

	"github.com/gocql/gocql"
)

type Command int

const (
	SelectCmd Command = iota
	InsertCmd
	DeleteCmd
	UpdateCmd
	CountCmd
)

type Statement struct {
	session    *Session
	Command    Command
	Table      Table
	Conditions []Condition
	Orders     []OrderBy
	LimitValue int
	TTLValue   int
	mapping    map[string]interface{}
	values     []interface{}
}

func NewStatement(sess *Session) *Statement {
	return &Statement{session: sess}
}

func (s *Statement) TypeScan() error {
	if query, err := s.query(); err != nil {
		return err
	} else {
		return query.MapScan(s.mapping)
	}
}

func (s *Statement) Scan(i ...interface{}) error {
	if query, err := s.query(); err != nil {
		return err
	} else {
		return query.Scan(i...)
	}
}

func (s *Statement) Exec() error {
	if query, err := s.query(); err != nil {
		return err
	} else {
		return query.Exec()
	}
}

func (s *Statement) Iter() *Iter {
	return &Iter{
		statement: s,
	}
}

func (s *Statement) query() (*gocql.Query, error) {
	var cql []string
	supportsTTL := false

	switch s.Command {
	case SelectCmd:
		cql = append(cql, fmt.Sprintf("SELECT * FROM %s", s.Table.Name))
	case InsertCmd:
		supportsTTL = true
		cql = append(cql, fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", s.Table.Name, s.Table.getCols(), s.Table.getQms()))
	case DeleteCmd:
		cql = append(cql, fmt.Sprintf("DELETE FROM %s", s.Table.Name))
	// case UpdateCmd:
	//  supportsTTL = true
	// 	cql = append(cql, fmt.Sprintf("UPDATE %s", s.Table))
	case CountCmd:
		cql = append(cql, fmt.Sprintf("SELECT COUNT(1) FROM %s", s.Table.Name))
	default:
		return nil, ErrInvalidCommand
	}

	var args []interface{}

	if len(s.Conditions) > 0 {
		cql = append(cql, "WHERE")
		for _, cond := range s.Conditions {
			args = append(args, cond.Value)
			switch cond.Predicate {
			case EqPredicate:
				cql = append(cql, fmt.Sprintf("%s = ?", cond.Column))
			case GtPredicate:
				cql = append(cql, fmt.Sprintf("%s > ?", cond.Column))
			case GePredicate:
				cql = append(cql, fmt.Sprintf("%s >= ?", cond.Column))
			case LtPredicate:
				cql = append(cql, fmt.Sprintf("%s < ?", cond.Column))
			case LePredicate:
				cql = append(cql, fmt.Sprintf("%s <= ?", cond.Column))
			case InPredicate:
				cql = append(cql, fmt.Sprintf("%s IN (%s)", qms(len(cond.Values)), cond.Column))
			}
		}
	}

	if len(s.values) > 0 {
		for i := range s.values {
			args = append(args, s.values[i])
		}
	}

	if s.Command == SelectCmd {
		if len(s.Orders) > 0 {
			cql = append(cql, "ORDER BY")
			orders := make([]string, len(s.Orders))
			for i, o := range s.Orders {
				orders[i] = fmt.Sprintf("%s %s", o.Column, o.OrderType)
			}
			cql = append(cql, strings.Join(orders, ", "))
		}

		if s.LimitValue > 0 {
			cql = append(cql, fmt.Sprintf("LIMIT %d", s.LimitValue))
		}
	}

	if supportsTTL && s.TTLValue > 0 {
		cql = append(cql, fmt.Sprintf("USING TTL %d", s.TTLValue))
	}

	return s.session.Query(strings.Join(cql, " "), args...), nil
}

func (s *Statement) Do(cmd Command) *Statement {
	s.Command = cmd
	return s
}

func (s *Statement) From(table string) *Statement {
	s.Table = Table{Name: table}
	return s
}

func (s *Statement) FromType(i interface{}) *Statement {
	table := GetTable(i)
	return s.From(table.Name)
}

func (s *Statement) Where(cond ...Condition) *Statement {
	s.Conditions = cond
	return s
}

func (s *Statement) OrderBy(order ...OrderBy) *Statement {
	s.Orders = order
	return s
}

func (s *Statement) Bind(i interface{}) *Statement {
	s.values, s.Table = BindTable(i)
	return s
}

func (s *Statement) Map(i interface{}) *Statement {
	s.mapping, s.Table = MapTable(i)
	return s
}

func (s *Statement) Limit(n int) *Statement {
	s.LimitValue = n
	return s
}

func (s *Statement) TTL(seconds int) *Statement {
	s.TTLValue = seconds
	return s
}

func qms(l int) string {
	switch l {
	case 0:
		return ""
	case 1:
		return "?"
	default:
		return strings.Repeat("?,", l-1) + "?"
	}
}
