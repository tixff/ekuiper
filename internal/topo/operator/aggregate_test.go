package operator

import (
	"errors"
	"fmt"
	"github.com/lf-edge/ekuiper/internal/conf"
	"github.com/lf-edge/ekuiper/internal/topo/context"
	"github.com/lf-edge/ekuiper/internal/xsql"
	"github.com/lf-edge/ekuiper/pkg/cast"
	"reflect"
	"strings"
	"testing"
)

func TestAggregatePlan_Apply(t *testing.T) {
	var tests = []struct {
		sql    string
		data   interface{}
		result xsql.GroupedTuplesSet
	}{
		{
			sql: "SELECT abc FROM tbl group by abc",
			data: &xsql.Tuple{
				Emitter: "tbl",
				Message: xsql.Message{
					"abc": int64(6),
					"def": "hello",
				},
			},
			result: xsql.GroupedTuplesSet{
				{
					Content: []xsql.DataValuer{&xsql.Tuple{
						Emitter: "tbl",
						Message: xsql.Message{
							"abc": int64(6),
							"def": "hello",
						},
					},
					},
				},
			},
		},

		{
			sql: "SELECT abc FROM src1 GROUP BY TUMBLINGWINDOW(ss, 10), f1",
			data: xsql.WindowTuplesSet{
				Content: []xsql.WindowTuples{{
					Emitter: "src1",
					Tuples: []xsql.Tuple{
						{
							Emitter: "src1",
							Message: xsql.Message{"id1": 1, "f1": "v1"},
						}, {
							Emitter: "src1",
							Message: xsql.Message{"id1": 2, "f1": "v2"},
						}, {
							Emitter: "src1",
							Message: xsql.Message{"id1": 3, "f1": "v1"},
						},
					},
				},
				},
				WindowRange: &xsql.WindowRange{
					WindowStart: 1541152486013,
					WindowEnd:   1541152487013,
				},
			},
			result: xsql.GroupedTuplesSet{
				{
					Content: []xsql.DataValuer{
						&xsql.Tuple{
							Emitter: "src1",
							Message: xsql.Message{"id1": 1, "f1": "v1"},
						},
						&xsql.Tuple{
							Emitter: "src1",
							Message: xsql.Message{"id1": 3, "f1": "v1"},
						},
					},
					WindowRange: &xsql.WindowRange{
						WindowStart: 1541152486013,
						WindowEnd:   1541152487013,
					},
				},
				{
					Content: []xsql.DataValuer{
						&xsql.Tuple{
							Emitter: "src1",
							Message: xsql.Message{"id1": 2, "f1": "v2"},
						},
					},
					WindowRange: &xsql.WindowRange{
						WindowStart: 1541152486013,
						WindowEnd:   1541152487013,
					},
				},
			},
		},
		{
			sql: "SELECT abc FROM src1 GROUP BY id1, TUMBLINGWINDOW(ss, 10), f1",
			data: xsql.WindowTuplesSet{
				Content: []xsql.WindowTuples{{
					Emitter: "src1",
					Tuples: []xsql.Tuple{
						{
							Emitter: "src1",
							Message: xsql.Message{"id1": 1, "f1": "v1"},
						}, {
							Emitter: "src1",
							Message: xsql.Message{"id1": 2, "f1": "v2"},
						}, {
							Emitter: "src1",
							Message: xsql.Message{"id1": 3, "f1": "v1"},
						},
					},
				},
				},
			},
			result: xsql.GroupedTuplesSet{
				{
					Content: []xsql.DataValuer{
						&xsql.Tuple{
							Emitter: "src1",
							Message: xsql.Message{"id1": 1, "f1": "v1"},
						},
					},
				},
				{
					Content: []xsql.DataValuer{
						&xsql.Tuple{
							Emitter: "src1",
							Message: xsql.Message{"id1": 2, "f1": "v2"},
						},
					},
				},
				{
					Content: []xsql.DataValuer{
						&xsql.Tuple{
							Emitter: "src1",
							Message: xsql.Message{"id1": 3, "f1": "v1"},
						},
					},
				},
			},
		},
		{
			sql: "SELECT abc FROM src1 GROUP BY meta(topic), TUMBLINGWINDOW(ss, 10)",
			data: xsql.WindowTuplesSet{
				Content: []xsql.WindowTuples{{
					Emitter: "src1",
					Tuples: []xsql.Tuple{
						{
							Emitter:  "src1",
							Message:  xsql.Message{"id1": 1, "f1": "v1"},
							Metadata: xsql.Metadata{"topic": "topic1"},
						}, {
							Emitter:  "src1",
							Message:  xsql.Message{"id1": 2, "f1": "v2"},
							Metadata: xsql.Metadata{"topic": "topic2"},
						}, {
							Emitter:  "src1",
							Message:  xsql.Message{"id1": 3, "f1": "v1"},
							Metadata: xsql.Metadata{"topic": "topic1"},
						},
					},
				},
				},
			},
			result: xsql.GroupedTuplesSet{
				{
					Content: []xsql.DataValuer{
						&xsql.Tuple{
							Emitter:  "src1",
							Message:  xsql.Message{"id1": 1, "f1": "v1"},
							Metadata: xsql.Metadata{"topic": "topic1"},
						},
						&xsql.Tuple{
							Emitter:  "src1",
							Message:  xsql.Message{"id1": 3, "f1": "v1"},
							Metadata: xsql.Metadata{"topic": "topic1"},
						},
					},
				},
				{
					Content: []xsql.DataValuer{
						&xsql.Tuple{
							Emitter:  "src1",
							Message:  xsql.Message{"id1": 2, "f1": "v2"},
							Metadata: xsql.Metadata{"topic": "topic2"},
						},
					},
				},
			},
		},
		{
			sql: "SELECT id1 FROM src1 left join src2 on src1.id1 = src2.id2 GROUP BY src2.f2, TUMBLINGWINDOW(ss, 10)",
			data: &xsql.JoinTupleSets{
				Content: []xsql.JoinTuple{
					{
						Tuples: []xsql.Tuple{
							{Emitter: "src1", Message: xsql.Message{"id1": 1, "f1": "v1"}},
							{Emitter: "src2", Message: xsql.Message{"id2": 2, "f2": "w2"}},
						},
					},
					{
						Tuples: []xsql.Tuple{
							{Emitter: "src1", Message: xsql.Message{"id1": 2, "f1": "v2"}},
							{Emitter: "src2", Message: xsql.Message{"id2": 4, "f2": "w3"}},
						},
					},
					{
						Tuples: []xsql.Tuple{
							{Emitter: "src1", Message: xsql.Message{"id1": 3, "f1": "v1"}},
						},
					},
				},
				WindowRange: &xsql.WindowRange{
					WindowStart: 1541152486013,
					WindowEnd:   1541152487013,
				},
			},
			result: xsql.GroupedTuplesSet{
				{
					Content: []xsql.DataValuer{
						&xsql.JoinTuple{
							Tuples: []xsql.Tuple{
								{Emitter: "src1", Message: xsql.Message{"id1": 1, "f1": "v1"}},
								{Emitter: "src2", Message: xsql.Message{"id2": 2, "f2": "w2"}},
							},
						},
					},
					WindowRange: &xsql.WindowRange{
						WindowStart: 1541152486013,
						WindowEnd:   1541152487013,
					},
				},
				{
					Content: []xsql.DataValuer{
						&xsql.JoinTuple{
							Tuples: []xsql.Tuple{
								{Emitter: "src1", Message: xsql.Message{"id1": 2, "f1": "v2"}},
								{Emitter: "src2", Message: xsql.Message{"id2": 4, "f2": "w3"}},
							},
						},
					},
					WindowRange: &xsql.WindowRange{
						WindowStart: 1541152486013,
						WindowEnd:   1541152487013,
					},
				},
				{
					Content: []xsql.DataValuer{
						&xsql.JoinTuple{
							Tuples: []xsql.Tuple{
								{Emitter: "src1", Message: xsql.Message{"id1": 3, "f1": "v1"}},
							},
						},
					},
					WindowRange: &xsql.WindowRange{
						WindowStart: 1541152486013,
						WindowEnd:   1541152487013,
					},
				},
			},
		},
		{
			sql: "SELECT id1 FROM src1 left join src2 on src1.id1 = src2.id2 GROUP BY TUMBLINGWINDOW(ss, 10), src1.f1",
			data: &xsql.JoinTupleSets{
				Content: []xsql.JoinTuple{
					{
						Tuples: []xsql.Tuple{
							{Emitter: "src1", Message: xsql.Message{"id1": 1, "f1": "v1"}},
							{Emitter: "src2", Message: xsql.Message{"id2": 2, "f2": "w2"}},
						},
					},
					{
						Tuples: []xsql.Tuple{
							{Emitter: "src1", Message: xsql.Message{"id1": 2, "f1": "v2"}},
							{Emitter: "src2", Message: xsql.Message{"id2": 4, "f2": "w3"}},
						},
					},
					{
						Tuples: []xsql.Tuple{
							{Emitter: "src1", Message: xsql.Message{"id1": 3, "f1": "v1"}},
						},
					},
				},
			},
			result: xsql.GroupedTuplesSet{
				{
					Content: []xsql.DataValuer{
						&xsql.JoinTuple{
							Tuples: []xsql.Tuple{
								{Emitter: "src1", Message: xsql.Message{"id1": 1, "f1": "v1"}},
								{Emitter: "src2", Message: xsql.Message{"id2": 2, "f2": "w2"}},
							},
						},
						&xsql.JoinTuple{
							Tuples: []xsql.Tuple{
								{Emitter: "src1", Message: xsql.Message{"id1": 3, "f1": "v1"}},
							},
						},
					},
				},
				{
					Content: []xsql.DataValuer{
						&xsql.JoinTuple{
							Tuples: []xsql.Tuple{
								{Emitter: "src1", Message: xsql.Message{"id1": 2, "f1": "v2"}},
								{Emitter: "src2", Message: xsql.Message{"id2": 4, "f2": "w3"}},
							},
						},
					},
				},
			},
		},
		{
			sql: "SELECT id1 FROM src1 left join src2 on src1.id1 = src2.id2 GROUP BY TUMBLINGWINDOW(ss, 10), src1.ts",
			data: &xsql.JoinTupleSets{
				Content: []xsql.JoinTuple{
					{
						Tuples: []xsql.Tuple{
							{Emitter: "src1", Message: xsql.Message{"id1": 1, "f1": "v1", "ts": cast.TimeFromUnixMilli(1568854515000)}},
							{Emitter: "src2", Message: xsql.Message{"id2": 2, "f2": "w2"}},
						},
					},
					{
						Tuples: []xsql.Tuple{
							{Emitter: "src1", Message: xsql.Message{"id1": 2, "f1": "v2", "ts": cast.TimeFromUnixMilli(1568854573431)}},
							{Emitter: "src2", Message: xsql.Message{"id2": 4, "f2": "w3"}},
						},
					},
					{
						Tuples: []xsql.Tuple{
							{Emitter: "src1", Message: xsql.Message{"id1": 3, "f1": "v1", "ts": cast.TimeFromUnixMilli(1568854515000)}},
						},
					},
				},
			},
			result: xsql.GroupedTuplesSet{
				{
					Content: []xsql.DataValuer{
						&xsql.JoinTuple{
							Tuples: []xsql.Tuple{
								{Emitter: "src1", Message: xsql.Message{"id1": 1, "f1": "v1", "ts": cast.TimeFromUnixMilli(1568854515000)}},
								{Emitter: "src2", Message: xsql.Message{"id2": 2, "f2": "w2"}},
							},
						},
						&xsql.JoinTuple{
							Tuples: []xsql.Tuple{
								{Emitter: "src1", Message: xsql.Message{"id1": 3, "f1": "v1", "ts": cast.TimeFromUnixMilli(1568854515000)}},
							},
						},
					},
				},
				{
					Content: []xsql.DataValuer{
						&xsql.JoinTuple{
							Tuples: []xsql.Tuple{
								{Emitter: "src1", Message: xsql.Message{"id1": 2, "f1": "v2", "ts": cast.TimeFromUnixMilli(1568854573431)}},
								{Emitter: "src2", Message: xsql.Message{"id2": 4, "f2": "w3"}},
							},
						},
					},
				},
			},
		},
		{
			sql: "SELECT abc FROM src1 GROUP BY TUMBLINGWINDOW(ss, 10), CASE WHEN id1 > 1 THEN \"others\" ELSE \"one\" END",
			data: xsql.WindowTuplesSet{
				Content: []xsql.WindowTuples{{
					Emitter: "src1",
					Tuples: []xsql.Tuple{
						{
							Emitter: "src1",
							Message: xsql.Message{"id1": 1, "f1": "v1"},
						}, {
							Emitter: "src1",
							Message: xsql.Message{"id1": 2, "f1": "v2"},
						}, {
							Emitter: "src1",
							Message: xsql.Message{"id1": 3, "f1": "v1"},
						},
					},
				},
				},
			},
			result: xsql.GroupedTuplesSet{
				{
					Content: []xsql.DataValuer{
						&xsql.Tuple{
							Emitter: "src1",
							Message: xsql.Message{"id1": 1, "f1": "v1"},
						},
					},
				},
				{
					Content: []xsql.DataValuer{
						&xsql.Tuple{
							Emitter: "src1",
							Message: xsql.Message{"id1": 2, "f1": "v2"},
						},
						&xsql.Tuple{
							Emitter: "src1",
							Message: xsql.Message{"id1": 3, "f1": "v1"},
						},
					},
				},
			},
		},
	}

	fmt.Printf("The test bucket size is %d.\n\n", len(tests))
	contextLogger := conf.Log.WithField("rule", "TestFilterPlan_Apply")
	ctx := context.WithValue(context.Background(), context.LoggerKey, contextLogger)
	for i, tt := range tests {
		stmt, err := xsql.NewParser(strings.NewReader(tt.sql)).Parse()
		if err != nil {
			t.Errorf("statement parse error %s", err)
			break
		}
		fv, afv := xsql.NewFunctionValuersForOp(nil, xsql.FuncRegisters)
		pp := &AggregateOp{Dimensions: stmt.Dimensions.GetGroups()}
		result := pp.Apply(ctx, tt.data, fv, afv)
		gr, ok := result.(xsql.GroupedTuplesSet)
		if !ok {
			t.Errorf("result is not GroupedTuplesSet")
		}
		if len(tt.result) != len(gr) {
			t.Errorf("%d. %q\n\nresult mismatch:\n\nexp=%#v\n\ngot=%#v\n\n", i, tt.sql, tt.result, gr)
		}

		for _, r := range tt.result {
			matched := false
			for _, gre := range gr {
				if reflect.DeepEqual(r, gre) {
					matched = true
				}
			}
			if !matched {
				t.Errorf("%d. %q\n\nresult mismatch:\n\nexp=%#v\n\ngot=%#v\n\n", i, tt.sql, tt.result, gr)
			}
		}
	}
}

func TestAggregatePlanError(t *testing.T) {
	tests := []struct {
		sql    string
		data   interface{}
		result error
	}{
		{
			sql:    "SELECT abc FROM tbl group by abc",
			data:   errors.New("an error from upstream"),
			result: errors.New("an error from upstream"),
		},

		{
			sql: "SELECT abc FROM src1 GROUP BY TUMBLINGWINDOW(ss, 10), f1 * 2",
			data: xsql.WindowTuplesSet{
				Content: []xsql.WindowTuples{
					{
						Emitter: "src1",
						Tuples: []xsql.Tuple{
							{
								Emitter: "src1",
								Message: xsql.Message{"id1": 1, "f1": "v1"},
							}, {
								Emitter: "src1",
								Message: xsql.Message{"id1": 2, "f1": "v2"},
							}, {
								Emitter: "src1",
								Message: xsql.Message{"id1": 3, "f1": "v1"},
							},
						},
					},
				},
			},
			result: errors.New("run Group By error: invalid operation string(v1) * int64(2)"),
		},
	}

	fmt.Printf("The test bucket size is %d.\n\n", len(tests))
	contextLogger := conf.Log.WithField("rule", "TestFilterPlanError")
	ctx := context.WithValue(context.Background(), context.LoggerKey, contextLogger)
	for i, tt := range tests {
		stmt, err := xsql.NewParser(strings.NewReader(tt.sql)).Parse()
		if err != nil {
			t.Errorf("statement parse error %s", err)
			break
		}
		fv, afv := xsql.NewFunctionValuersForOp(nil, xsql.FuncRegisters)
		pp := &AggregateOp{Dimensions: stmt.Dimensions.GetGroups()}
		result := pp.Apply(ctx, tt.data, fv, afv)
		if !reflect.DeepEqual(tt.result, result) {
			t.Errorf("%d. %q\n\nresult mismatch:\n\nexp=%#v\n\ngot=%#v\n\n", i, tt.sql, tt.result, result)
		}
	}
}
