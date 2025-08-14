package graph

import (
	"testing"

	"github.com/xhanio/framingo/pkg/utils/errors"
)

// testNode implements common.Named for testing
type testNode struct {
	name string
}

func (t testNode) Name() string {
	return t.name
}

func newTestNode(name string) testNode {
	return testNode{name: name}
}

func TestNew(t *testing.T) {
	graph := New[testNode]()
	if graph == nil {
		t.Error("New() should return a non-nil graph")
	}
	if graph.Count() != 0 {
		t.Errorf("New graph should have 0 nodes, got %d", graph.Count())
	}
	if len(graph.Nodes()) != 0 {
		t.Errorf("New graph should have empty nodes slice, got %v", graph.Nodes())
	}
}

func TestGraph_Add(t *testing.T) {
	tests := []struct {
		name          string
		setup         func() Graph[testNode]
		expectedCount int
		expectedNodes []string
	}{
		{
			name: "add single node without dependencies",
			setup: func() Graph[testNode] {
				g := New[testNode]()
				g.Add(newTestNode("A"))
				return g
			},
			expectedCount: 1,
			expectedNodes: []string{"A"},
		},
		{
			name: "add node with single dependency",
			setup: func() Graph[testNode] {
				g := New[testNode]()
				g.Add(newTestNode("A"), newTestNode("B"))
				return g
			},
			expectedCount: 2,
			expectedNodes: []string{"A", "B"},
		},
		{
			name: "add node with multiple dependencies",
			setup: func() Graph[testNode] {
				g := New[testNode]()
				g.Add(newTestNode("A"), newTestNode("B"), newTestNode("C"))
				return g
			},
			expectedCount: 3,
			expectedNodes: []string{"A", "B", "C"},
		},
		{
			name: "add duplicate nodes",
			setup: func() Graph[testNode] {
				g := New[testNode]()
				g.Add(newTestNode("A"))
				g.Add(newTestNode("A")) // duplicate
				return g
			},
			expectedCount: 1,
			expectedNodes: []string{"A"},
		},
		{
			name: "add multiple nodes with overlapping dependencies",
			setup: func() Graph[testNode] {
				g := New[testNode]()
				g.Add(newTestNode("A"), newTestNode("B"))
				g.Add(newTestNode("C"), newTestNode("B")) // B is duplicate dependency
				return g
			},
			expectedCount: 3,
			expectedNodes: []string{"A", "B", "C"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := tt.setup()

			if g.Count() != tt.expectedCount {
				t.Errorf("Count() = %d, want %d", g.Count(), tt.expectedCount)
			}

			nodes := g.Nodes()
			if len(nodes) != tt.expectedCount {
				t.Errorf("len(Nodes()) = %d, want %d", len(nodes), tt.expectedCount)
			}

			// Check all expected nodes are present
			nodeNames := make(map[string]bool)
			for _, node := range nodes {
				nodeNames[node.Name()] = true
			}

			for _, expectedName := range tt.expectedNodes {
				if !nodeNames[expectedName] {
					t.Errorf("Expected node %s not found in graph", expectedName)
				}
			}
		})
	}
}

func TestGraph_TopoSort(t *testing.T) {
	tests := []struct {
		name          string
		setup         func() Graph[testNode]
		expectError   bool
		expectedOrder []string // expected topological order
	}{
		{
			name: "empty graph",
			setup: func() Graph[testNode] {
				return New[testNode]()
			},
			expectError:   false,
			expectedOrder: []string{},
		},
		{
			name: "single node",
			setup: func() Graph[testNode] {
				g := New[testNode]()
				g.Add(newTestNode("A"))
				return g
			},
			expectError:   false,
			expectedOrder: []string{"A"},
		},
		{
			name: "two nodes with dependency",
			setup: func() Graph[testNode] {
				g := New[testNode]()
				g.Add(newTestNode("A"), newTestNode("B")) // A depends on B
				return g
			},
			expectError:   false,
			expectedOrder: []string{"B", "A"}, // B comes before A
		},
		{
			name: "linear dependency chain",
			setup: func() Graph[testNode] {
				g := New[testNode]()
				g.Add(newTestNode("C"), newTestNode("B")) // C depends on B
				g.Add(newTestNode("B"), newTestNode("A")) // B depends on A
				return g
			},
			expectError:   false,
			expectedOrder: []string{"A", "B", "C"}, // A -> B -> C
		},
		{
			name: "diamond dependency",
			setup: func() Graph[testNode] {
				g := New[testNode]()
				g.Add(newTestNode("D"), newTestNode("B"), newTestNode("C")) // D depends on B, C
				g.Add(newTestNode("B"), newTestNode("A"))                   // B depends on A
				g.Add(newTestNode("C"), newTestNode("A"))                   // C depends on A
				return g
			},
			expectError: false,
			// A should come first, then B and C (order between B and C doesn't matter), then D
			expectedOrder: []string{"A"}, // We'll check A is first and D is last
		},
		{
			name: "simple cycle",
			setup: func() Graph[testNode] {
				g := New[testNode]()
				g.Add(newTestNode("A"), newTestNode("B")) // A depends on B
				g.Add(newTestNode("B"), newTestNode("A")) // B depends on A (cycle!)
				return g
			},
			expectError: true,
		},
		{
			name: "self cycle",
			setup: func() Graph[testNode] {
				g := New[testNode]()
				g.Add(newTestNode("A"), newTestNode("A")) // A depends on itself
				return g
			},
			expectError: true,
		},
		{
			name: "complex cycle",
			setup: func() Graph[testNode] {
				g := New[testNode]()
				g.Add(newTestNode("A"), newTestNode("B")) // A depends on B
				g.Add(newTestNode("B"), newTestNode("C")) // B depends on C
				g.Add(newTestNode("C"), newTestNode("A")) // C depends on A (cycle!)
				return g
			},
			expectError: true,
		},
		{
			name: "multiple independent components",
			setup: func() Graph[testNode] {
				g := New[testNode]()
				g.Add(newTestNode("A"), newTestNode("B")) // A depends on B
				g.Add(newTestNode("C"), newTestNode("D")) // C depends on D
				g.Add(newTestNode("E"))                   // E has no dependencies
				return g
			},
			expectError:   false,
			expectedOrder: []string{"B", "D", "E"}, // These should all come before their dependents
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := tt.setup()
			err := g.TopoSort()

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				} else if !errors.Is(err, errors.Conflict) {
					t.Errorf("Expected conflict error, got %v", err)
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			nodes := g.Nodes()
			nodeNames := make([]string, len(nodes))
			for i, node := range nodes {
				nodeNames[i] = node.Name()
			}

			// For simple cases, check exact order
			if len(tt.expectedOrder) == len(nodeNames) {
				for i, expected := range tt.expectedOrder {
					if i < len(nodeNames) && nodeNames[i] != expected {
						t.Errorf("Position %d: got %s, want %s", i, nodeNames[i], expected)
					}
				}
			} else if tt.name == "diamond dependency" {
				// Special case: check A is first and D is last
				if len(nodeNames) != 4 {
					t.Errorf("Expected 4 nodes, got %d", len(nodeNames))
				}
				if nodeNames[0] != "A" {
					t.Errorf("Expected A to be first, got %s", nodeNames[0])
				}
				if nodeNames[3] != "D" {
					t.Errorf("Expected D to be last, got %s", nodeNames[3])
				}
			} else if tt.name == "multiple independent components" {
				// Check that dependencies come before dependents
				positions := make(map[string]int)
				for i, name := range nodeNames {
					positions[name] = i
				}

				// B should come before A
				if positions["B"] >= positions["A"] {
					t.Error("B should come before A")
				}
				// D should come before C
				if positions["D"] >= positions["C"] {
					t.Error("D should come before C")
				}
			}
		})
	}
}

func TestGraph_Count(t *testing.T) {
	g := New[testNode]()

	if g.Count() != 0 {
		t.Errorf("Empty graph Count() = %d, want 0", g.Count())
	}

	g.Add(newTestNode("A"))
	if g.Count() != 1 {
		t.Errorf("After adding one node Count() = %d, want 1", g.Count())
	}

	g.Add(newTestNode("B"))
	if g.Count() != 2 {
		t.Errorf("After adding two nodes Count() = %d, want 2", g.Count())
	}

	// Adding duplicate should not increase count
	g.Add(newTestNode("A"))
	if g.Count() != 2 {
		t.Errorf("After adding duplicate Count() = %d, want 2", g.Count())
	}
}

func TestGraph_Nodes(t *testing.T) {
	g := New[testNode]()

	// Empty graph
	nodes := g.Nodes()
	if len(nodes) != 0 {
		t.Errorf("Empty graph Nodes() length = %d, want 0", len(nodes))
	}

	// Add nodes
	g.Add(newTestNode("A"))
	g.Add(newTestNode("B"), newTestNode("C"))

	nodes = g.Nodes()
	if len(nodes) != 3 {
		t.Errorf("After adding nodes, Nodes() length = %d, want 3", len(nodes))
	}

	// Check all nodes are present
	nodeNames := make(map[string]bool)
	for _, node := range nodes {
		nodeNames[node.Name()] = true
	}

	expectedNames := []string{"A", "B", "C"}
	for _, expected := range expectedNames {
		if !nodeNames[expected] {
			t.Errorf("Expected node %s not found in Nodes()", expected)
		}
	}
}

func TestGraph_TopoSortPreservesOrder(t *testing.T) {
	// Test that TopoSort can be called multiple times and maintains consistency
	g := New[testNode]()
	g.Add(newTestNode("C"), newTestNode("B"))
	g.Add(newTestNode("B"), newTestNode("A"))

	// First sort
	err1 := g.TopoSort()
	if err1 != nil {
		t.Fatalf("First TopoSort failed: %v", err1)
	}
	firstOrder := make([]string, len(g.Nodes()))
	for i, node := range g.Nodes() {
		firstOrder[i] = node.Name()
	}

	// Second sort
	err2 := g.TopoSort()
	if err2 != nil {
		t.Fatalf("Second TopoSort failed: %v", err2)
	}
	secondOrder := make([]string, len(g.Nodes()))
	for i, node := range g.Nodes() {
		secondOrder[i] = node.Name()
	}

	// Orders should be the same
	if len(firstOrder) != len(secondOrder) {
		t.Errorf("Order lengths differ: %d vs %d", len(firstOrder), len(secondOrder))
	}

	for i := 0; i < len(firstOrder) && i < len(secondOrder); i++ {
		if firstOrder[i] != secondOrder[i] {
			t.Errorf("Position %d differs: %s vs %s", i, firstOrder[i], secondOrder[i])
		}
	}
}
