package grange

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/deckarep/golang-set"
)

// --------- STATE

type State struct {
	clusters map[string]Cluster
	groups   Cluster

	// Populated lazily as groups are evaluated. They won't change unless state
	// changes.
	groupCache   map[string]*mapset.Set
	clusterCache map[string]map[string]*mapset.Set
}

type Cluster map[string][]string

var (
  // Maximum number of characters that grange will try to parse in a query.
  // Queries longer than this will be rejected. This limit also applies to
  // cluster and group names and values. Combined with MaxResults, this limits
  // result sizes to approximately 1MB.
	MaxQuerySize = 1000

	// The maximum number of results a query can return. Execution will be
	// short-circuited once this many results have been gathered. No error will
	// be returned.
	MaxResults = 10000
)

type tooManyResults struct {}

func (state *State) PrimeCache() {
	// traverse and expand every cluster, adding them to cache.
	EvalRange("clusters(a)", state)

	// traverse and expand every group
	EvalRange("//", state)
}

// --------- CONTEXTS

type evalContext struct {
	currentClusterName string
	currentResult      mapset.Set
	workingResult      *mapset.Set
}

func newContext() evalContext {
	return evalContext{currentResult: mapset.NewSet()}
}

func newClusterContext(clusterName string) evalContext {
	return evalContext{
		currentClusterName: clusterName,
		currentResult:      mapset.NewSet(),
	}
}

func SetGroups(state *State, c Cluster) {
	state.groups = c
	state.ResetCache()
}

func AddCluster(state *State, name string, c Cluster) {
	state.clusters[name] = c
	state.ResetCache()
}

func (state *State) ResetCache() {
	state.groupCache = map[string]*mapset.Set{}
	state.clusterCache = map[string]map[string]*mapset.Set{}
}

func NewState() State {
	state := State{
		clusters: map[string]Cluster{},
		groups:   Cluster{},
	}
	state.ResetCache()
	return state
}

func NewResult(args ...interface{}) mapset.Set {
	return mapset.NewSetFromSlice(args)
}

func parseRange(input string) (Node, error) {
	r := &RangeQuery{Buffer: input}
	r.Init()
	if err := r.Parse(); err != nil {
		return nil, err
	}
	r.Execute()
	return r.nodeStack[0], nil
}

func EvalRange(input string, state *State) (result mapset.Set, err error) {
	if len(input) > MaxQuerySize {
		return mapset.NewSet(),
      errors.New(fmt.Sprintf("Query is too long, max length is %d", MaxQuerySize))
	}
	return evalRange(input, state)
}

func evalRange(input string, state *State) (result mapset.Set, err error) {
	context := newContext()
	return evalRangeWithContext(input, state, &context)
}

func evalRangeWithContext(input string, state *State, context *evalContext) (mapset.Set, error) {
	err := evalRangeInplace(input, state, context)

	return context.currentResult, err
}

// Useful internally so that results do not need to be copied all over the place
func evalRangeInplace(input string, state *State, context *evalContext) (err error) {
	node, parseError := parseRange(input)
	if parseError != nil {
		return errors.New("Could not parse query")
	}

  defer func() {
    if r := recover(); r != nil {
      switch r.(type) {
      case tooManyResults:
        // No error returned, we just chop off the results
        err = nil
      case error:
        err = r.(error)
      default:
        panic(r)
      }
    }
  }()

	return node.(EvalNode).visit(state, context)
}

func (c evalContext) hasResults() bool {
	return c.currentResult.Cardinality() == 0
}

func (n BracesNode) visit(state *State, context *evalContext) error {
	leftContext := newContext()
	rightContext := newContext()
	middleContext := newContext()
	// TODO: Handle errors
	n.left.(EvalNode).visit(state, &leftContext)
	n.node.(EvalNode).visit(state, &middleContext)
	n.right.(EvalNode).visit(state, &rightContext)

	if leftContext.hasResults() {
		leftContext.addResult("")
	}
	if middleContext.hasResults() {
		middleContext.addResult("")
	}
	if rightContext.hasResults() {
		rightContext.addResult("")
	}

	for l := range leftContext.resultIter() {
		for m := range middleContext.resultIter() {
			for r := range rightContext.resultIter() {
				context.addResult(fmt.Sprintf("%s%s%s", l, m, r))
			}
		}
	}

	return nil
}

func (n LocalClusterLookupNode) visit(state *State, context *evalContext) error {
	if context.currentClusterName == "" {
		return groupLookup(state, context, n.key)
	}

	return clusterLookup(state, context, n.key)
}

func (n ClusterLookupNode) visit(state *State, context *evalContext) error {
	var evalErr error

	subContext := newContext()
	evalErr = n.node.(EvalNode).visit(state, &subContext)
	if evalErr != nil {
		return evalErr
	}

	keyContext := newContext()
	evalErr = n.key.(EvalNode).visit(state, &keyContext)
	if evalErr != nil {
		return evalErr
	}

	for clusterName := range subContext.resultIter() {
		context.currentClusterName = clusterName.(string)
		for key := range keyContext.resultIter() {
			evalErr = clusterLookup(state, context, key.(string))
			if evalErr != nil {
				return evalErr
			}
		}
	}

	return nil
}

func (n GroupLookupNode) visit(state *State, context *evalContext) error {
	subContext := context.sub()
	n.node.(EvalNode).visit(state, &subContext) // TODO: Error handle

	for key := range subContext.resultIter() {
		groupLookup(state, context, key.(string))
	}

	return nil
}

func (c evalContext) sub() evalContext {
	return newClusterContext(c.currentClusterName)
}

func (n OperatorNode) visit(state *State, context *evalContext) error {
	switch n.op {
	case operatorIntersect:

		leftContext := context.sub()
		n.left.(EvalNode).visit(state, &leftContext) // TODO: Error handle

		if leftContext.currentResult.Cardinality() == 0 {
			// Optimization: no need to compute right side if left side is empty
			return nil
		}

		rightContext := context.sub()
		// RegexNode needs to know about LHS to filter correctly
		rightContext.workingResult = &leftContext.currentResult
		n.right.(EvalNode).visit(state, &rightContext) // TODO: Error handle

		for x := range leftContext.currentResult.Intersect(rightContext.currentResult).Iter() {
			context.addResult(x.(string))
		}
	case operatorSubtract:
		leftContext := context.sub()
		n.left.(EvalNode).visit(state, &leftContext) // TODO: Error handle

		if leftContext.currentResult.Cardinality() == 0 {
			// Optimization: no need to compute right side if left side is empty
			return nil
		}

		rightContext := context.sub()
		// RegexNode needs to know about LHS to filter correctly
		rightContext.workingResult = &leftContext.currentResult
		n.right.(EvalNode).visit(state, &rightContext) // TODO: Error handle

		for x := range leftContext.currentResult.Difference(rightContext.currentResult).Iter() {
			context.addResult(x.(string))
		}
	case operatorUnion:
		// TODO: Handle errors
		n.left.(EvalNode).visit(state, context)
		n.right.(EvalNode).visit(state, context)
	}
	return nil
}

func (n ConstantNode) visit(state *State, context *evalContext) error {
	context.addResult(n.val)
	return nil
}

var (
	numericRangeRegexp = regexp.MustCompile("^(.*?)(\\d+)\\.\\.([^\\d]*?)?(\\d+)(.*)$")
)

func (n TextNode) visit(state *State, context *evalContext) error {
	match := numericRangeRegexp.FindStringSubmatch(n.val)

	if len(match) == 0 {
		context.addResult(n.val)
		return nil
	}

	leftStr := match[1]
	leftStrToMatch := match[1]
	leftN := match[2]
	rightStr := match[3]
	rightN := match[4]
	trailing := match[5]

	for {
		if len(leftN) <= len(rightN) {
			break
		}

		leftStr += leftN[0:1]
		leftN = leftN[1:]
	}

	// a1..a4 is valid, a1..b4 is invalid
	if len(rightStr) != 0 && leftStrToMatch != rightStr {
		context.addResult(n.val)
	}

	width := strconv.Itoa(len(leftN))
	low, _ := strconv.Atoi(leftN)
	high, _ := strconv.Atoi(rightN)

	for x := low; x <= high; x++ {
	 context.addResult(fmt.Sprintf("%s%0"+width+"d%s", leftStr, x, trailing))
	}

	return nil
}

func (n GroupQueryNode) visit(state *State, context *evalContext) error {
	subContext := newContext()
	// TODO: Handle errors
	n.node.(EvalNode).visit(state, &subContext)
	lookingFor := subContext.currentResult

	for groupName, group := range state.groups {
		groupContext := newContext()
		for _, value := range group {
			// TODO: Handle errors
			evalRangeInplace(value, state, &groupContext)
		}

		for x := range lookingFor {
			if groupContext.currentResult.Contains(x) {
				context.addResult(groupName)
				break
			}
		}
	}
	return nil
}

func (n FunctionNode) visit(state *State, context *evalContext) error {
	switch n.name {
	case "has":
		// TODO: Error handling when no or multiple results
		keyContext := newContext()
		valueContext := newContext()
		n.params[0].(EvalNode).visit(state, &keyContext)
		n.params[1].(EvalNode).visit(state, &valueContext)

		key := (<-keyContext.resultIter()).(string)
		toMatch := (<-valueContext.resultIter()).(string)

		for clusterName, cluster := range state.clusters {
			for _, value := range cluster[key] {
				// TODO: Need to eval value?
				if value == toMatch {
					context.addResult(clusterName)
				}
			}
		}
	case "clusters":
		// TODO: Error handling
		subContext := newContext()
		n.params[0].(EvalNode).visit(state, &subContext)

		lookingFor := subContext.currentResult

		for clusterName, _ := range state.clusters {
			subContext = newClusterContext(clusterName)
			clusterLookup(state, &subContext, "CLUSTER")

			for value := range subContext.resultIter() {
				if lookingFor.Contains(value) {
					context.addResult(clusterName)
				}
			}
		}
	}
	return nil
}

func (n RegexNode) visit(state *State, context *evalContext) error {
	if context.workingResult == nil {
		subContext := context.sub()
		state.allValues(&subContext)
		context.workingResult = &subContext.currentResult
	}

	for x := range context.workingResult.Iter() {
		if strings.Contains(x.(string), n.val) {
			context.addResult(x.(string))
		}
	}

	return nil
}

func (n NullNode) visit(state *State, context *evalContext) error {
	return nil
}

func (state *State) allValues(context *evalContext) error {
	// Expand everything into the set
	for _, v := range state.groups {
		for _, subv := range v {
			// TODO: Handle errors
			evalRangeInplace(subv, state, context)
		}
	}

	return nil
}

func groupLookup(state *State, context *evalContext, key string) error {
	if state.groupCache[key] != nil {
		for x := range state.groupCache[key].Iter() {
			context.addResult(x.(string))
		}
		return nil
	}

	clusterExp := state.groups[key]

	for _, value := range clusterExp {
		// TODO: Return errors correctly
		evalRangeInplace(value, state, context)
	}
	state.groupCache[key] = &context.currentResult
	return nil
}

func clusterLookup(state *State, context *evalContext, key string) error {
	var evalErr error
	clusterName := context.currentClusterName
	cluster := state.clusters[clusterName]

	if key == "KEYS" {
		for k, _ := range cluster {
			context.currentResult.Add(k) // TODO: addResult
		}
		return nil
	}

	if state.clusterCache[clusterName] == nil {
		state.clusterCache[clusterName] = map[string]*mapset.Set{}
	}

	if state.clusterCache[clusterName][key] == nil {
		clusterExp := cluster[key] // TODO: Error handling

		subContext := newClusterContext(context.currentClusterName)

		for _, value := range clusterExp {
			evalErr = evalRangeInplace(value, state, &subContext)
			if evalErr != nil {
				return evalErr
			}
		}

		state.clusterCache[clusterName][key] = &subContext.currentResult
	}

	for x := range state.clusterCache[clusterName][key].Iter() {
		context.addResult(x.(string))
	}
	return nil
}

func (c *evalContext) addResult(value string) {
	if c.currentResult.Cardinality() >= MaxResults {
    panic(tooManyResults{})
	}

  if len(value) > MaxQuerySize {
    panic(errors.New(
      fmt.Sprintf("Value would exceed max query size: %s...", value[0:20])))
  }

	c.currentResult.Add(value)
}

func (c *evalContext) resultIter() <-chan interface{} {
	return c.currentResult.Iter()
}

type EvalNode interface {
	visit(*State, *evalContext) error
}
