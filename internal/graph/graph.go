package graph

import (
	"fmt"

	"github.com/HybridMind1337/DataMask/internal/discovery"
)

// DependencyGraph models table relationships for computing safe processing order.
type DependencyGraph struct {
	Tables    map[string]*discovery.Table
	Adjacency map[string][]string // table -> tables that depend on it
	InDegree  map[string]int      // number of incoming dependencies (parents to be processed before this table)
}

// NewDependencyGraph builds a graph from schema table definitions and foreign keys.
func NewDependencyGraph(schema *discovery.Schema) *DependencyGraph {
	g := &DependencyGraph{
		Tables:    schema.Tables,
		Adjacency: make(map[string][]string),
		InDegree:  make(map[string]int),
	}

	for name := range schema.Tables {
		g.Adjacency[name] = make([]string, 0)
		g.InDegree[name] = 0
	}

	for tableName, table := range schema.Tables {
		for _, fk := range table.ForeignKeys {
			refTable := fk.ReferencedTable
			// Ignore self-references or references outside schema
			if refTable == tableName || schema.Tables[refTable] == nil {
				continue
			}
			// refTable must be masked before tableName
			g.Adjacency[refTable] = append(g.Adjacency[refTable], tableName)
			g.InDegree[tableName]++
		}
	}

	return g
}

// TopologicalSort computes an execution order so parents are processed before children.
// If circular dependencies are present, returns the best-effort partial order along with cycle tables.
func (g *DependencyGraph) TopologicalSort() (order []string, cycles []string, err error) {
	inDegree := make(map[string]int)
	for k, v := range g.InDegree {
		inDegree[k] = v
	}

	queue := make([]string, 0)
	for name, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, name)
		}
	}

	visited := 0
	total := len(g.Tables)

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]
		order = append(order, curr)
		visited++

		for _, neighbor := range g.Adjacency[curr] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if visited < total {
		for name, deg := range inDegree {
			if deg > 0 {
				cycles = append(cycles, name)
			}
		}
		return order, cycles, fmt.Errorf("circular dependencies detected among %d tables", len(cycles))
	}

	return order, nil, nil
}
