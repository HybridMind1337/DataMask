package graph_test

import (
	"testing"

	"github.com/HybridMind1337/DataMask/internal/discovery"
	"github.com/HybridMind1337/DataMask/internal/graph"
)

func TestTopologicalSort(t *testing.T) {
	schema := &discovery.Schema{
		Name: "test",
		Tables: map[string]*discovery.Table{
			"users": {
				Name:        "users",
				ForeignKeys: nil,
			},
			"orders": {
				Name: "orders",
				ForeignKeys: []*discovery.ForeignKey{
					{ReferencedTable: "users"},
				},
			},
			"order_items": {
				Name: "order_items",
				ForeignKeys: []*discovery.ForeignKey{
					{ReferencedTable: "orders"},
				},
			},
		},
	}

	g := graph.NewDependencyGraph(schema)
	order, cycles, err := g.TopologicalSort()

	if err != nil {
		t.Fatalf("unexpected sort error: %v", err)
	}
	if len(cycles) > 0 {
		t.Fatalf("unexpected cycles: %v", cycles)
	}

	// Verify order: users must come before orders, orders before order_items
	idxMap := make(map[string]int)
	for i, name := range order {
		idxMap[name] = i
	}

	if idxMap["users"] >= idxMap["orders"] {
		t.Errorf("expected users before orders")
	}
	if idxMap["orders"] >= idxMap["order_items"] {
		t.Errorf("expected orders before order_items")
	}
}
