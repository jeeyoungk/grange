package grange

import (
	"reflect"
	"testing"
)

func TestDefaultCluster(t *testing.T) {
	testEval(t, []string{"b", "c"}, "%a", singleCluster("a", Cluster{
		"CLUSTER": []string{"b", "c"},
	}))
}

func TestExplicitCluster(t *testing.T) {
	testEval(t, []string{"b", "c"}, "%a:NODES", singleCluster("a", Cluster{
		"NODES": []string{"b", "c"},
	}))
}

func TestClusterKeys(t *testing.T) {
	testEval(t, []string{"NODES"}, "%a:KEYS", singleCluster("a", Cluster{
		"NODES": []string{"b", "c"},
	}))
}

func TestClusterMissing(t *testing.T) {
	testEval(t, []string{}, "%a", emptyState())
}
func TestClusterMissingKey(t *testing.T) {
	testEval(t, []string{}, "%a:NODES", singleCluster("a", Cluster{}))
}

func TestErrorExplicitCluster(t *testing.T) {
	testError(t, "Invalid token in query: \"}\"", "%a:}")
}

func TestErrorClusterName(t *testing.T) {
	testError(t, "Invalid token in query: \"}\"", "%}")
}

func TestHas(t *testing.T) {
	testEval(t, []string{"a", "b"}, "has(TYPE;one)", multiCluster(map[string]Cluster{
		"a": Cluster{"TYPE": []string{"one", "two"}},
		"b": Cluster{"TYPE": []string{"two", "one"}},
		"c": Cluster{"TYPE": []string{"three"}},
	}))
}

func TestHasIntersect(t *testing.T) {
	testEval(t, []string{"b"}, "has(TYPE;one)&b", multiCluster(map[string]Cluster{
		"a": Cluster{"TYPE": []string{"one", "two"}},
		"b": Cluster{"TYPE": []string{"two", "one"}},
		"c": Cluster{"TYPE": []string{"three"}},
	}))

	testEval(t, []string{"b"}, "has(TYPE;two)&has(TYPE;three)", multiCluster(map[string]Cluster{
		"a": Cluster{"TYPE": []string{"one", "two"}},
		"b": Cluster{"TYPE": []string{"two", "one", "three"}},
		"c": Cluster{"TYPE": []string{"three"}},
	}))
}

func TestIntersect(t *testing.T) {
	testEval(t, []string{"c"}, "%a:L&%a:R", singleCluster("a", Cluster{
		"L": []string{"b", "c"},
		"R": []string{"c", "d"},
	}))
}

/*
// TODO: Pending
func TestIntersectError(t *testing.T) {
	testError(t, "No left side provided for intersection", "&a")
}
*/

func TestUnion(t *testing.T) {
	testEval(t, []string{"a", "b"}, "a,b", emptyState())
}

func TestBracesWithUnion(t *testing.T) {
	testEval(t, []string{"a.c", "b.c"}, "{a,b}.c", emptyState())
	testEval(t, []string{"a.b", "a.c"}, "a.{b,c}", emptyState())
	testEval(t, []string{"a.b.d", "a.c.d"}, "a.{b,c}.d", emptyState())
}

func TestClusterUnion(t *testing.T) {
	testEval(t, []string{"c", "d"}, "%a,%b", multiCluster(map[string]Cluster{
		"a": Cluster{"CLUSTER": []string{"c"}},
		"b": Cluster{"CLUSTER": []string{"d"}},
	}))
}

/*
// TODO: Pending
func TestNoExpandInClusterName(t *testing.T) {
	testError(t, "Invalid token in query: \"{\"", "%a-{b,c}")
}
*/

func TestSelfReferentialCluster(t *testing.T) {
	testEval(t, []string{"b"}, "%a", multiCluster(map[string]Cluster{
		"a": Cluster{"CLUSTER": []string{"$ALL"}, "ALL": []string{"b"}},
	}))
}

func TestSelfReferentialClusterExpression(t *testing.T) {
	testEval(t, []string{"a", "c"}, "%a", multiCluster(map[string]Cluster{
		"a": Cluster{
			"CLUSTER": []string{"$ALL - $DOWN"},
			"ALL":     []string{"a", "b", "c"},
			"DOWN":    []string{"b"},
		},
	}))
}

func TestGroups(t *testing.T) {
	testEval(t, []string{"a", "b"}, "@dc", singleGroup("dc", "a", "b"))
}

func TestGroupsExpand(t *testing.T) {
	testEval(t, []string{"c"}, "@a", multiGroup(Cluster{
		"a": []string{"$b"},
		"b": []string{"c"},
	}))
}

func TestClusterLookup(t *testing.T) {
	testEval(t, []string{"a"}, "%{has(TYPE;db)}", singleCluster("ignore", Cluster{
		"CLUSTER": []string{"a"},
		"TYPE":    []string{"db"},
	}))
}

func TestClusterLookupExplicitKey(t *testing.T) {
	testEval(t, []string{"a"}, "%{has(TYPE;db)}:NODES", singleCluster("ignore", Cluster{
		"NODES": []string{"a"},
		"TYPE":  []string{"db"},
	}))
}

func TestClusterLookupDedup(t *testing.T) {
	testEval(t, []string{"one", "two"}, "%{has(TYPE;one)}:TYPE", multiCluster(map[string]Cluster{
		"a": Cluster{"TYPE": []string{"one", "two"}},
		"b": Cluster{"TYPE": []string{"two", "one"}},
		"c": Cluster{"TYPE": []string{"three"}},
	}))
}

func TestMatchNoContext(t *testing.T) {
	testEval(t, []string{"ab"}, "/b/", singleGroup("b", "ab", "c"))
}

func TestMatch(t *testing.T) {
	testEval(t, []string{"ab", "ba", "abc"}, "%cluster & /b/",
		singleCluster("cluster", Cluster{
			"CLUSTER": []string{"ab", "ba", "abc", "ccc"},
		}))
}

func TestMatchReverse(t *testing.T) {
	testEval(t, []string{"ab", "ba", "abc"}, "/b/ & @group",
		singleGroup("group", "ab", "ba", "abc", "ccc"))
}

func TestMatchWithExclude(t *testing.T) {
	testEval(t, []string{"ccc"}, "%cluster - /b/",
		singleCluster("cluster", Cluster{
			"CLUSTER": []string{"ab", "ba", "abc", "ccc"},
		}))
}

func TestInvalidLex(t *testing.T) {
	testError(t, "No closing / for match", "/")
}

func TestClusters(t *testing.T) {
	testEval(t, []string{"a", "b"}, "clusters(one)", multiCluster(map[string]Cluster{
		"a": Cluster{"CLUSTER": []string{"two", "one"}},
		"b": Cluster{"CLUSTER": []string{"$ALL"}, "ALL": []string{"one"}},
		"c": Cluster{"CLUSTER": []string{"three"}},
	}))
}

func TestQ(t *testing.T) {
	testEval(t, []string{"(/"}, "q((/)", emptyState())
	testEval(t, []string{"http://foo/bar?yeah"}, "q(http://foo/bar?yeah)", emptyState())
}

func TestQueryGroups(t *testing.T) {
	testEval(t, []string{"one", "two"}, "?a", multiGroup(Cluster{
		"one":   []string{"a"},
		"two":   []string{"$one"},
		"three": []string{"b"},
	}))
}

func TestNumericRange(t *testing.T) {
	testEval(t, []string{"n01", "n02", "n03"}, "n01..n03", emptyState())
	testEval(t, []string{"n01", "n02", "n03"}, "n01..n3", emptyState())

	testEval(t, []string{"1", "2", "3"}, "1..3", emptyState())
	testEval(t, []string{"n1", "n2", "n3"}, "n1..3", emptyState())
	testEval(t, []string{"n1", "n2", "n3"}, "n1..n3", emptyState())
	testEval(t, []string{}, "n2..n1", emptyState())
	testEval(t, []string{"n9", "n10", "n11"}, "n9..11", emptyState())
	testEval(t, []string{"n1", "n2", "n3"}, "n1..n03", emptyState())
	testEval(t, []string{"n10", "n11"}, "n10..1", emptyState())
	testEval(t, []string{"n1..2an3", "n1..2an4"}, "n1..2an3..4", emptyState())
	testEval(t, []string{"n1..3"}, "q(n1..3)", emptyState())
}

func testError(t *testing.T, expected string, query string) {
	_, err := evalRange(query, emptyState())

	if err == nil {
		t.Errorf("Expected error but none returned")
	} else if err.Error() != expected {
		// TODO: Get error messages back
		//t.Errorf("Different error returned.\n got: %s\nwant: %s", err.Error(), expected)
	}
}

func testEval(t *testing.T, expected []string, query string, state *RangeState) {
	actual, err := evalRange(query, state)

	if err != nil {
		t.Errorf("Expected result, got error: %s", err)
	} else if !reflect.DeepEqual(actual, expected) {
		t.Errorf("evalRange\n got: %v\nwant: %v", actual, expected)
	}
}

func singleCluster(name string, c Cluster) *RangeState {
	state := RangeState{
		clusters: map[string]Cluster{},
	}
	state.clusters[name] = c
	return &state
}

func singleGroup(name string, members ...string) *RangeState {
	state := RangeState{
		groups: map[string][]string{},
	}
	state.groups[name] = members
	return &state
}

func multiGroup(c Cluster) *RangeState {
	state := RangeState{
		groups: c,
	}
	return &state
}

func multiCluster(cs map[string]Cluster) *RangeState {
	state := RangeState{
		clusters: cs,
	}
	return &state
}

func emptyState() *RangeState {
	return &RangeState{}
}
