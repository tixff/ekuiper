package node

import (
	"github.com/lf-edge/ekuiper/internal/conf"
	"github.com/lf-edge/ekuiper/internal/topo/context"
	"github.com/lf-edge/ekuiper/internal/topo/state"
	"github.com/lf-edge/ekuiper/pkg/api"
	"github.com/lf-edge/ekuiper/pkg/ast"
	"testing"
)

func TestSourcePool(t *testing.T) {
	n := NewSourceNode("test", ast.TypeStream, &ast.Options{
		DATASOURCE: "demo",
		TYPE:       "mock",
		SHARED:     true,
	})
	n.concurrency = 2
	contextLogger := conf.Log.WithField("rule", "mockRule0")
	ctx := context.WithValue(context.Background(), context.LoggerKey, contextLogger)
	tempStore, _ := state.CreateStore("mockRule0", api.AtMostOnce)
	n.ctx = ctx.WithMeta("mockRule0", "test", tempStore)
	n1 := NewSourceNode("test", ast.TypeStream, &ast.Options{
		DATASOURCE: "demo1",
		TYPE:       "mock",
		SHARED:     true,
	})

	contextLogger = conf.Log.WithField("rule", "mockRule1")
	ctx = context.WithValue(context.Background(), context.LoggerKey, contextLogger)
	tempStore, _ = state.CreateStore("mockRule1", api.AtMostOnce)
	n1.ctx = ctx.WithMeta("mockRule1", "test1", tempStore)
	n2 := NewSourceNode("test2", ast.TypeStream, &ast.Options{
		DATASOURCE: "demo1",
		TYPE:       "mock",
	})
	contextLogger = conf.Log.WithField("rule", "mockRule2")
	ctx = context.WithValue(context.Background(), context.LoggerKey, contextLogger)
	tempStore, _ = state.CreateStore("mockRule2", api.AtMostOnce)
	n2.ctx = ctx.WithMeta("mockRule2", "test2", tempStore)

	// Test add source instance
	getSourceInstance(n, 0)
	getSourceInstance(n1, 0)
	getSourceInstance(n, 1)
	getSourceInstance(n2, 0)

	poolLen := len(pool.registry)
	if poolLen != 1 {
		t.Errorf("source instances length unmatch: expect %d but got %d", 1, poolLen)
		return
	}
	si, ok := pool.registry["mock.test"]
	if !ok {
		t.Errorf("source instances pool unmatch: can't find key %s", "mock.test")
		return
	}
	outputLen := len(si.outputs)
	if outputLen != 3 {
		t.Errorf("source instances length unmatch: expect %d but got %d", 3, outputLen)
		return
	}

	removeSourceInstance(n)
	poolLen = len(pool.registry)
	if poolLen != 1 {
		t.Errorf("source instances length unmatch: expect %d but got %d", 1, poolLen)
		return
	}
	si, ok = pool.registry["mock.test"]
	if !ok {
		t.Errorf("source instances pool unmatch: can't find key %s", "mock.test")
		return
	}
	outputLen = len(si.outputs)
	if outputLen != 1 {
		t.Errorf("source instances length unmatch: expect %d but got %d", 1, outputLen)
		return
	}

	removeSourceInstance(n1)
	poolLen = len(pool.registry)
	if poolLen != 0 {
		t.Errorf("source instances length unmatch: expect %d but got %d", 0, poolLen)
		return
	}

	removeSourceInstance(n2)
}
