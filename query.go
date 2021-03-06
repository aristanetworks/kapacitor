package kapacitor

import (
	"fmt"
	"time"

	"github.com/influxdata/influxdb/influxql"
	"github.com/influxdata/kapacitor/tick/ast"
)

type Query struct {
	startTL *influxql.TimeLiteral
	stopTL  *influxql.TimeLiteral
	stmt    *influxql.SelectStatement
}

func NewQuery(queryString string) (*Query, error) {
	query := &Query{}
	// Parse and validate query
	q, err := influxql.ParseQuery(queryString)
	if err != nil {
		return nil, err
	}
	if l := len(q.Statements); l != 1 {
		return nil, fmt.Errorf("query must be a single select statement, got %d statements", l)
	}
	var ok bool
	query.stmt, ok = q.Statements[0].(*influxql.SelectStatement)
	if !ok {
		return nil, fmt.Errorf("query is not a select statement %q", q)
	}

	// Add in time condition nodes
	query.startTL = &influxql.TimeLiteral{}
	startExpr := &influxql.BinaryExpr{
		Op:  influxql.GTE,
		LHS: &influxql.VarRef{Val: "time"},
		RHS: query.startTL,
	}

	query.stopTL = &influxql.TimeLiteral{}
	stopExpr := &influxql.BinaryExpr{
		Op:  influxql.LT,
		LHS: &influxql.VarRef{Val: "time"},
		RHS: query.stopTL,
	}

	if query.stmt.Condition != nil {
		query.stmt.Condition = &influxql.BinaryExpr{
			Op:  influxql.AND,
			LHS: query.stmt.Condition,
			RHS: &influxql.BinaryExpr{
				Op:  influxql.AND,
				LHS: startExpr,
				RHS: stopExpr,
			},
		}
	} else {
		query.stmt.Condition = &influxql.BinaryExpr{
			Op:  influxql.AND,
			LHS: startExpr,
			RHS: stopExpr,
		}
	}
	return query, nil
}

// Return the db rp pairs of the query
func (q *Query) DBRPs() ([]DBRP, error) {
	dbrps := make([]DBRP, len(q.stmt.Sources))
	for i, s := range q.stmt.Sources {
		m, ok := s.(*influxql.Measurement)
		if !ok {
			return nil, fmt.Errorf("unknown query source %T", s)
		}
		dbrps[i] = DBRP{
			Database:        m.Database,
			RetentionPolicy: m.RetentionPolicy,
		}
	}
	return dbrps, nil
}

// Set the start time of the query
func (q *Query) Start(s time.Time) {
	q.startTL.Val = s
}

// Set the stop time of the query
func (q *Query) Stop(s time.Time) {
	q.stopTL.Val = s
}

// Set the dimensions on the query
func (q *Query) Dimensions(dims []interface{}) error {
	q.stmt.Dimensions = q.stmt.Dimensions[:0]
	// Add in dimensions
	hasTime := false
	for _, d := range dims {
		switch dim := d.(type) {
		case time.Duration:
			if hasTime {
				return fmt.Errorf("groupBy cannot have more than one time dimension")
			}
			// Add time dimension
			hasTime = true
			q.stmt.Dimensions = append(q.stmt.Dimensions,
				&influxql.Dimension{
					Expr: &influxql.Call{
						Name: "time",
						Args: []influxql.Expr{
							&influxql.DurationLiteral{
								Val: dim,
							},
						},
					},
				})
		case string:
			q.stmt.Dimensions = append(q.stmt.Dimensions,
				&influxql.Dimension{
					Expr: &influxql.VarRef{
						Val: dim,
					},
				})
		case *ast.StarNode:
			q.stmt.Dimensions = append(q.stmt.Dimensions,
				&influxql.Dimension{
					Expr: &influxql.Wildcard{},
				})
		case TimeDimension:
			q.stmt.Dimensions = append(q.stmt.Dimensions,
				&influxql.Dimension{
					Expr: &influxql.Call{
						Name: "time",
						Args: []influxql.Expr{
							&influxql.DurationLiteral{
								Val: dim.Length,
							},
							&influxql.DurationLiteral{
								Val: dim.Offset,
							},
						},
					},
				})

		default:
			return fmt.Errorf("invalid dimension type:%T, must be string or time.Duration", d)
		}
	}

	return nil
}

func (q *Query) Fill(option influxql.FillOption, value interface{}) {
	q.stmt.Fill = option
	q.stmt.FillValue = value
}

func (q *Query) String() string {
	return q.stmt.String()
}

type TimeDimension struct {
	Length time.Duration
	Offset time.Duration
}

func groupByTime(length time.Duration, offset ...time.Duration) (TimeDimension, error) {
	var o time.Duration
	if l := len(offset); l == 1 {
		o = offset[0]

	} else if l != 0 {
		return TimeDimension{}, fmt.Errorf("time() function expects 1 or 2 args, got %d", l+1)
	}
	return TimeDimension{
		Length: length,
		Offset: o,
	}, nil
}
