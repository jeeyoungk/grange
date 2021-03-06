package grange

import (
	"fmt"
	"math"
	"sort"
	"strconv"
)

const end_symbol rune = 4

/* The rule types inferred from the grammar are below. */
type pegRule uint8

const (
	ruleUnknown pegRule = iota
	ruleexpression
	rulerangeexpr
	rulecombinators
	ruleintersect
	ruleexclude
	ruleunion
	rulebraces
	rulebrackets
	rulegroupq
	rulecluster
	rulegroup
	rulekey
	rulelocalkey
	rulefunction
	rulefuncargs
	ruleregex
	ruleliteral
	rulevalue
	ruleleaderChar
	rulespace
	ruleconst
	ruleq
	rulequoted
	ruleAction0
	rulenull
	ruleAction1
	ruleAction2
	ruleAction3
	ruleAction4
	ruleAction5
	ruleAction6
	ruleAction7
	ruleAction8
	ruleAction9
	ruleAction10
	ruleAction11
	ruleAction12
	ruleAction13
	rulePegText
	ruleAction14
	ruleAction15
	ruleAction16
	ruleAction17

	rulePre_
	rule_In_
	rule_Suf
)

var rul3s = [...]string{
	"Unknown",
	"expression",
	"rangeexpr",
	"combinators",
	"intersect",
	"exclude",
	"union",
	"braces",
	"brackets",
	"groupq",
	"cluster",
	"group",
	"key",
	"localkey",
	"function",
	"funcargs",
	"regex",
	"literal",
	"value",
	"leaderChar",
	"space",
	"const",
	"q",
	"quoted",
	"Action0",
	"null",
	"Action1",
	"Action2",
	"Action3",
	"Action4",
	"Action5",
	"Action6",
	"Action7",
	"Action8",
	"Action9",
	"Action10",
	"Action11",
	"Action12",
	"Action13",
	"PegText",
	"Action14",
	"Action15",
	"Action16",
	"Action17",

	"Pre_",
	"_In_",
	"_Suf",
}

type tokenTree interface {
	Print()
	PrintSyntax()
	PrintSyntaxTree(buffer string)
	Add(rule pegRule, begin, end, next, depth int)
	Expand(index int) tokenTree
	Tokens() <-chan token32
	AST() *node32
	Error() []token32
	trim(length int)
}

type node32 struct {
	token32
	up, next *node32
}

func (node *node32) print(depth int, buffer string) {
	for node != nil {
		for c := 0; c < depth; c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[node.pegRule], strconv.Quote(buffer[node.begin:node.end]))
		if node.up != nil {
			node.up.print(depth+1, buffer)
		}
		node = node.next
	}
}

func (ast *node32) Print(buffer string) {
	ast.print(0, buffer)
}

type element struct {
	node *node32
	down *element
}

/* ${@} bit structure for abstract syntax tree */
type token16 struct {
	pegRule
	begin, end, next int16
}

func (t *token16) isZero() bool {
	return t.pegRule == ruleUnknown && t.begin == 0 && t.end == 0 && t.next == 0
}

func (t *token16) isParentOf(u token16) bool {
	return t.begin <= u.begin && t.end >= u.end && t.next > u.next
}

func (t *token16) getToken32() token32 {
	return token32{pegRule: t.pegRule, begin: int32(t.begin), end: int32(t.end), next: int32(t.next)}
}

func (t *token16) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v %v", rul3s[t.pegRule], t.begin, t.end, t.next)
}

type tokens16 struct {
	tree    []token16
	ordered [][]token16
}

func (t *tokens16) trim(length int) {
	t.tree = t.tree[0:length]
}

func (t *tokens16) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens16) Order() [][]token16 {
	if t.ordered != nil {
		return t.ordered
	}

	depths := make([]int16, 1, math.MaxInt16)
	for i, token := range t.tree {
		if token.pegRule == ruleUnknown {
			t.tree = t.tree[:i]
			break
		}
		depth := int(token.next)
		if length := len(depths); depth >= length {
			depths = depths[:depth+1]
		}
		depths[depth]++
	}
	depths = append(depths, 0)

	ordered, pool := make([][]token16, len(depths)), make([]token16, len(t.tree)+len(depths))
	for i, depth := range depths {
		depth++
		ordered[i], pool, depths[i] = pool[:depth], pool[depth:], 0
	}

	for i, token := range t.tree {
		depth := token.next
		token.next = int16(i)
		ordered[depth][depths[depth]] = token
		depths[depth]++
	}
	t.ordered = ordered
	return ordered
}

type state16 struct {
	token16
	depths []int16
	leaf   bool
}

func (t *tokens16) AST() *node32 {
	tokens := t.Tokens()
	stack := &element{node: &node32{token32: <-tokens}}
	for token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	return stack.node
}

func (t *tokens16) PreOrder() (<-chan state16, [][]token16) {
	s, ordered := make(chan state16, 6), t.Order()
	go func() {
		var states [8]state16
		for i, _ := range states {
			states[i].depths = make([]int16, len(ordered))
		}
		depths, state, depth := make([]int16, len(ordered)), 0, 1
		write := func(t token16, leaf bool) {
			S := states[state]
			state, S.pegRule, S.begin, S.end, S.next, S.leaf = (state+1)%8, t.pegRule, t.begin, t.end, int16(depth), leaf
			copy(S.depths, depths)
			s <- S
		}

		states[state].token16 = ordered[0][0]
		depths[0]++
		state++
		a, b := ordered[depth-1][depths[depth-1]-1], ordered[depth][depths[depth]]
	depthFirstSearch:
		for {
			for {
				if i := depths[depth]; i > 0 {
					if c, j := ordered[depth][i-1], depths[depth-1]; a.isParentOf(c) &&
						(j < 2 || !ordered[depth-1][j-2].isParentOf(c)) {
						if c.end != b.begin {
							write(token16{pegRule: rule_In_, begin: c.end, end: b.begin}, true)
						}
						break
					}
				}

				if a.begin < b.begin {
					write(token16{pegRule: rulePre_, begin: a.begin, end: b.begin}, true)
				}
				break
			}

			next := depth + 1
			if c := ordered[next][depths[next]]; c.pegRule != ruleUnknown && b.isParentOf(c) {
				write(b, false)
				depths[depth]++
				depth, a, b = next, b, c
				continue
			}

			write(b, true)
			depths[depth]++
			c, parent := ordered[depth][depths[depth]], true
			for {
				if c.pegRule != ruleUnknown && a.isParentOf(c) {
					b = c
					continue depthFirstSearch
				} else if parent && b.end != a.end {
					write(token16{pegRule: rule_Suf, begin: b.end, end: a.end}, true)
				}

				depth--
				if depth > 0 {
					a, b, c = ordered[depth-1][depths[depth-1]-1], a, ordered[depth][depths[depth]]
					parent = a.isParentOf(b)
					continue
				}

				break depthFirstSearch
			}
		}

		close(s)
	}()
	return s, ordered
}

func (t *tokens16) PrintSyntax() {
	tokens, ordered := t.PreOrder()
	max := -1
	for token := range tokens {
		if !token.leaf {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[36m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[36m%v\x1B[m\n", rul3s[token.pegRule])
		} else if token.begin == token.end {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[31m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[31m%v\x1B[m\n", rul3s[token.pegRule])
		} else {
			for c, end := token.begin, token.end; c < end; c++ {
				if i := int(c); max+1 < i {
					for j := max; j < i; j++ {
						fmt.Printf("skip %v %v\n", j, token.String())
					}
					max = i
				} else if i := int(c); i <= max {
					for j := i; j <= max; j++ {
						fmt.Printf("dupe %v %v\n", j, token.String())
					}
				} else {
					max = int(c)
				}
				fmt.Printf("%v", c)
				for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
					fmt.Printf(" \x1B[34m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
				}
				fmt.Printf(" \x1B[34m%v\x1B[m\n", rul3s[token.pegRule])
			}
			fmt.Printf("\n")
		}
	}
}

func (t *tokens16) PrintSyntaxTree(buffer string) {
	tokens, _ := t.PreOrder()
	for token := range tokens {
		for c := 0; c < int(token.next); c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[token.pegRule], strconv.Quote(buffer[token.begin:token.end]))
	}
}

func (t *tokens16) Add(rule pegRule, begin, end, depth, index int) {
	t.tree[index] = token16{pegRule: rule, begin: int16(begin), end: int16(end), next: int16(depth)}
}

func (t *tokens16) Tokens() <-chan token32 {
	s := make(chan token32, 16)
	go func() {
		for _, v := range t.tree {
			s <- v.getToken32()
		}
		close(s)
	}()
	return s
}

func (t *tokens16) Error() []token32 {
	ordered := t.Order()
	length := len(ordered)
	tokens, length := make([]token32, length), length-1
	for i, _ := range tokens {
		o := ordered[length-i]
		if len(o) > 1 {
			tokens[i] = o[len(o)-2].getToken32()
		}
	}
	return tokens
}

/* ${@} bit structure for abstract syntax tree */
type token32 struct {
	pegRule
	begin, end, next int32
}

func (t *token32) isZero() bool {
	return t.pegRule == ruleUnknown && t.begin == 0 && t.end == 0 && t.next == 0
}

func (t *token32) isParentOf(u token32) bool {
	return t.begin <= u.begin && t.end >= u.end && t.next > u.next
}

func (t *token32) getToken32() token32 {
	return token32{pegRule: t.pegRule, begin: int32(t.begin), end: int32(t.end), next: int32(t.next)}
}

func (t *token32) String() string {
	return fmt.Sprintf("\x1B[34m%v\x1B[m %v %v %v", rul3s[t.pegRule], t.begin, t.end, t.next)
}

type tokens32 struct {
	tree    []token32
	ordered [][]token32
}

func (t *tokens32) trim(length int) {
	t.tree = t.tree[0:length]
}

func (t *tokens32) Print() {
	for _, token := range t.tree {
		fmt.Println(token.String())
	}
}

func (t *tokens32) Order() [][]token32 {
	if t.ordered != nil {
		return t.ordered
	}

	depths := make([]int32, 1, math.MaxInt16)
	for i, token := range t.tree {
		if token.pegRule == ruleUnknown {
			t.tree = t.tree[:i]
			break
		}
		depth := int(token.next)
		if length := len(depths); depth >= length {
			depths = depths[:depth+1]
		}
		depths[depth]++
	}
	depths = append(depths, 0)

	ordered, pool := make([][]token32, len(depths)), make([]token32, len(t.tree)+len(depths))
	for i, depth := range depths {
		depth++
		ordered[i], pool, depths[i] = pool[:depth], pool[depth:], 0
	}

	for i, token := range t.tree {
		depth := token.next
		token.next = int32(i)
		ordered[depth][depths[depth]] = token
		depths[depth]++
	}
	t.ordered = ordered
	return ordered
}

type state32 struct {
	token32
	depths []int32
	leaf   bool
}

func (t *tokens32) AST() *node32 {
	tokens := t.Tokens()
	stack := &element{node: &node32{token32: <-tokens}}
	for token := range tokens {
		if token.begin == token.end {
			continue
		}
		node := &node32{token32: token}
		for stack != nil && stack.node.begin >= token.begin && stack.node.end <= token.end {
			stack.node.next = node.up
			node.up = stack.node
			stack = stack.down
		}
		stack = &element{node: node, down: stack}
	}
	return stack.node
}

func (t *tokens32) PreOrder() (<-chan state32, [][]token32) {
	s, ordered := make(chan state32, 6), t.Order()
	go func() {
		var states [8]state32
		for i, _ := range states {
			states[i].depths = make([]int32, len(ordered))
		}
		depths, state, depth := make([]int32, len(ordered)), 0, 1
		write := func(t token32, leaf bool) {
			S := states[state]
			state, S.pegRule, S.begin, S.end, S.next, S.leaf = (state+1)%8, t.pegRule, t.begin, t.end, int32(depth), leaf
			copy(S.depths, depths)
			s <- S
		}

		states[state].token32 = ordered[0][0]
		depths[0]++
		state++
		a, b := ordered[depth-1][depths[depth-1]-1], ordered[depth][depths[depth]]
	depthFirstSearch:
		for {
			for {
				if i := depths[depth]; i > 0 {
					if c, j := ordered[depth][i-1], depths[depth-1]; a.isParentOf(c) &&
						(j < 2 || !ordered[depth-1][j-2].isParentOf(c)) {
						if c.end != b.begin {
							write(token32{pegRule: rule_In_, begin: c.end, end: b.begin}, true)
						}
						break
					}
				}

				if a.begin < b.begin {
					write(token32{pegRule: rulePre_, begin: a.begin, end: b.begin}, true)
				}
				break
			}

			next := depth + 1
			if c := ordered[next][depths[next]]; c.pegRule != ruleUnknown && b.isParentOf(c) {
				write(b, false)
				depths[depth]++
				depth, a, b = next, b, c
				continue
			}

			write(b, true)
			depths[depth]++
			c, parent := ordered[depth][depths[depth]], true
			for {
				if c.pegRule != ruleUnknown && a.isParentOf(c) {
					b = c
					continue depthFirstSearch
				} else if parent && b.end != a.end {
					write(token32{pegRule: rule_Suf, begin: b.end, end: a.end}, true)
				}

				depth--
				if depth > 0 {
					a, b, c = ordered[depth-1][depths[depth-1]-1], a, ordered[depth][depths[depth]]
					parent = a.isParentOf(b)
					continue
				}

				break depthFirstSearch
			}
		}

		close(s)
	}()
	return s, ordered
}

func (t *tokens32) PrintSyntax() {
	tokens, ordered := t.PreOrder()
	max := -1
	for token := range tokens {
		if !token.leaf {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[36m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[36m%v\x1B[m\n", rul3s[token.pegRule])
		} else if token.begin == token.end {
			fmt.Printf("%v", token.begin)
			for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
				fmt.Printf(" \x1B[31m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
			}
			fmt.Printf(" \x1B[31m%v\x1B[m\n", rul3s[token.pegRule])
		} else {
			for c, end := token.begin, token.end; c < end; c++ {
				if i := int(c); max+1 < i {
					for j := max; j < i; j++ {
						fmt.Printf("skip %v %v\n", j, token.String())
					}
					max = i
				} else if i := int(c); i <= max {
					for j := i; j <= max; j++ {
						fmt.Printf("dupe %v %v\n", j, token.String())
					}
				} else {
					max = int(c)
				}
				fmt.Printf("%v", c)
				for i, leaf, depths := 0, int(token.next), token.depths; i < leaf; i++ {
					fmt.Printf(" \x1B[34m%v\x1B[m", rul3s[ordered[i][depths[i]-1].pegRule])
				}
				fmt.Printf(" \x1B[34m%v\x1B[m\n", rul3s[token.pegRule])
			}
			fmt.Printf("\n")
		}
	}
}

func (t *tokens32) PrintSyntaxTree(buffer string) {
	tokens, _ := t.PreOrder()
	for token := range tokens {
		for c := 0; c < int(token.next); c++ {
			fmt.Printf(" ")
		}
		fmt.Printf("\x1B[34m%v\x1B[m %v\n", rul3s[token.pegRule], strconv.Quote(buffer[token.begin:token.end]))
	}
}

func (t *tokens32) Add(rule pegRule, begin, end, depth, index int) {
	t.tree[index] = token32{pegRule: rule, begin: int32(begin), end: int32(end), next: int32(depth)}
}

func (t *tokens32) Tokens() <-chan token32 {
	s := make(chan token32, 16)
	go func() {
		for _, v := range t.tree {
			s <- v.getToken32()
		}
		close(s)
	}()
	return s
}

func (t *tokens32) Error() []token32 {
	ordered := t.Order()
	length := len(ordered)
	tokens, length := make([]token32, length), length-1
	for i, _ := range tokens {
		o := ordered[length-i]
		if len(o) > 1 {
			tokens[i] = o[len(o)-2].getToken32()
		}
	}
	return tokens
}

func (t *tokens16) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		for i, v := range tree {
			expanded[i] = v.getToken32()
		}
		return &tokens32{tree: expanded}
	}
	return nil
}

func (t *tokens32) Expand(index int) tokenTree {
	tree := t.tree
	if index >= len(tree) {
		expanded := make([]token32, 2*len(tree))
		copy(expanded, tree)
		t.tree = expanded
	}
	return nil
}

type rangeQuery struct {
	currentLiteral string
	nodeStack      []parserNode

	Buffer string
	buffer []rune
	rules  [44]func() bool
	Parse  func(rule ...int) error
	Reset  func()
	tokenTree
}

type textPosition struct {
	line, symbol int
}

type textPositionMap map[int]textPosition

func translatePositions(buffer string, positions []int) textPositionMap {
	length, translations, j, line, symbol := len(positions), make(textPositionMap, len(positions)), 0, 1, 0
	sort.Ints(positions)

search:
	for i, c := range buffer[0:] {
		if c == '\n' {
			line, symbol = line+1, 0
		} else {
			symbol++
		}
		if i == positions[j] {
			translations[positions[j]] = textPosition{line, symbol}
			for j++; j < length; j++ {
				if i != positions[j] {
					continue search
				}
			}
			break search
		}
	}

	return translations
}

type parseError struct {
	p *rangeQuery
}

func (e *parseError) Error() string {
	tokens, error := e.p.tokenTree.Error(), "\n"
	positions, p := make([]int, 2*len(tokens)), 0
	for _, token := range tokens {
		positions[p], p = int(token.begin), p+1
		positions[p], p = int(token.end), p+1
	}
	translations := translatePositions(e.p.Buffer, positions)
	for _, token := range tokens {
		begin, end := int(token.begin), int(token.end)
		error += fmt.Sprintf("parse error near \x1B[34m%v\x1B[m (line %v symbol %v - line %v symbol %v):\n%v\n",
			rul3s[token.pegRule],
			translations[begin].line, translations[begin].symbol,
			translations[end].line, translations[end].symbol,
			/*strconv.Quote(*/ e.p.Buffer[begin:end] /*)*/)
	}

	return error
}

func (p *rangeQuery) PrintSyntaxTree() {
	p.tokenTree.PrintSyntaxTree(p.Buffer)
}

func (p *rangeQuery) Highlighter() {
	p.tokenTree.PrintSyntax()
}

func (p *rangeQuery) Execute() {
	buffer, begin, end := p.Buffer, 0, 0
	for token := range p.tokenTree.Tokens() {
		switch token.pegRule {
		case rulePegText:
			begin, end = int(token.begin), int(token.end)
		case ruleAction0:
			p.addBraceStart()
		case ruleAction1:
			p.addOperator(operatorIntersect)
		case ruleAction2:
			p.addOperator(operatorSubtract)
		case ruleAction3:
			p.addOperator(operatorUnion)
		case ruleAction4:
			p.addBraces()
		case ruleAction5:
			p.addGroupQuery()
		case ruleAction6:
			p.addValue(buffer[begin:end])
			p.addClusterLookup()
		case ruleAction7:
			p.addClusterLookup()
		case ruleAction8:
			p.addGroupLookup()
		case ruleAction9:
			p.addKeyLookup()
		case ruleAction10:
			p.addLocalClusterLookup(buffer[begin:end])
		case ruleAction11:
			p.addFunction(buffer[begin:end])
		case ruleAction12:
			p.addFuncArg()
		case ruleAction13:
			p.addFuncArg()
		case ruleAction14:
			p.addRegex(buffer[begin:end])
		case ruleAction15:
			p.addValue(buffer[begin:end])
		case ruleAction16:
			p.addConstant(buffer[begin:end])
		case ruleAction17:
			p.addConstant(buffer[begin:end])

		}
	}
}

func (p *rangeQuery) Init() {
	p.buffer = []rune(p.Buffer)
	if len(p.buffer) == 0 || p.buffer[len(p.buffer)-1] != end_symbol {
		p.buffer = append(p.buffer, end_symbol)
	}

	var tree tokenTree = &tokens16{tree: make([]token16, math.MaxInt16)}
	position, depth, tokenIndex, buffer, rules := 0, 0, 0, p.buffer, p.rules

	p.Parse = func(rule ...int) error {
		r := 1
		if len(rule) > 0 {
			r = rule[0]
		}
		matches := p.rules[r]()
		p.tokenTree = tree
		if matches {
			p.tokenTree.trim(tokenIndex)
			return nil
		}
		return &parseError{p}
	}

	p.Reset = func() {
		position, tokenIndex, depth = 0, 0, 0
	}

	add := func(rule pegRule, begin int) {
		if t := tree.Expand(tokenIndex); t != nil {
			tree = t
		}
		tree.Add(rule, begin, position, depth, tokenIndex)
		tokenIndex++
	}

	matchDot := func() bool {
		if buffer[position] != end_symbol {
			position++
			return true
		}
		return false
	}

	/*matchChar := func(c byte) bool {
		if buffer[position] == c {
			position++
			return true
		}
		return false
	}*/

	/*matchRange := func(lower byte, upper byte) bool {
		if c := buffer[position]; c >= lower && c <= upper {
			position++
			return true
		}
		return false
	}*/

	rules = [...]func() bool{
		nil,
		/* 0 expression <- <(rangeexpr combinators? !.)> */
		func() bool {
			position0, tokenIndex0, depth0 := position, tokenIndex, depth
			{
				position1 := position
				depth++
				if !rules[rulerangeexpr]() {
					goto l0
				}
				{
					position2, tokenIndex2, depth2 := position, tokenIndex, depth
					if !rules[rulecombinators]() {
						goto l2
					}
					goto l3
				l2:
					position, tokenIndex, depth = position2, tokenIndex2, depth2
				}
			l3:
				{
					position4, tokenIndex4, depth4 := position, tokenIndex, depth
					if !matchDot() {
						goto l4
					}
					goto l0
				l4:
					position, tokenIndex, depth = position4, tokenIndex4, depth4
				}
				depth--
				add(ruleexpression, position1)
			}
			return true
		l0:
			position, tokenIndex, depth = position0, tokenIndex0, depth0
			return false
		},
		/* 1 rangeexpr <- <(space (const / function / cluster / group / groupq / localkey / regex / value / brackets / (Action0 braces) / null) space)> */
		func() bool {
			position5, tokenIndex5, depth5 := position, tokenIndex, depth
			{
				position6 := position
				depth++
				if !rules[rulespace]() {
					goto l5
				}
				{
					position7, tokenIndex7, depth7 := position, tokenIndex, depth
					if !rules[ruleconst]() {
						goto l8
					}
					goto l7
				l8:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[rulefunction]() {
						goto l9
					}
					goto l7
				l9:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[rulecluster]() {
						goto l10
					}
					goto l7
				l10:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[rulegroup]() {
						goto l11
					}
					goto l7
				l11:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[rulegroupq]() {
						goto l12
					}
					goto l7
				l12:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[rulelocalkey]() {
						goto l13
					}
					goto l7
				l13:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[ruleregex]() {
						goto l14
					}
					goto l7
				l14:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[rulevalue]() {
						goto l15
					}
					goto l7
				l15:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[rulebrackets]() {
						goto l16
					}
					goto l7
				l16:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[ruleAction0]() {
						goto l17
					}
					if !rules[rulebraces]() {
						goto l17
					}
					goto l7
				l17:
					position, tokenIndex, depth = position7, tokenIndex7, depth7
					if !rules[rulenull]() {
						goto l5
					}
				}
			l7:
				if !rules[rulespace]() {
					goto l5
				}
				depth--
				add(rulerangeexpr, position6)
			}
			return true
		l5:
			position, tokenIndex, depth = position5, tokenIndex5, depth5
			return false
		},
		/* 2 combinators <- <(space (union / intersect / exclude / braces))> */
		func() bool {
			position18, tokenIndex18, depth18 := position, tokenIndex, depth
			{
				position19 := position
				depth++
				if !rules[rulespace]() {
					goto l18
				}
				{
					position20, tokenIndex20, depth20 := position, tokenIndex, depth
					if !rules[ruleunion]() {
						goto l21
					}
					goto l20
				l21:
					position, tokenIndex, depth = position20, tokenIndex20, depth20
					if !rules[ruleintersect]() {
						goto l22
					}
					goto l20
				l22:
					position, tokenIndex, depth = position20, tokenIndex20, depth20
					if !rules[ruleexclude]() {
						goto l23
					}
					goto l20
				l23:
					position, tokenIndex, depth = position20, tokenIndex20, depth20
					if !rules[rulebraces]() {
						goto l18
					}
				}
			l20:
				depth--
				add(rulecombinators, position19)
			}
			return true
		l18:
			position, tokenIndex, depth = position18, tokenIndex18, depth18
			return false
		},
		/* 3 intersect <- <('&' rangeexpr Action1 combinators?)> */
		func() bool {
			position24, tokenIndex24, depth24 := position, tokenIndex, depth
			{
				position25 := position
				depth++
				if buffer[position] != rune('&') {
					goto l24
				}
				position++
				if !rules[rulerangeexpr]() {
					goto l24
				}
				if !rules[ruleAction1]() {
					goto l24
				}
				{
					position26, tokenIndex26, depth26 := position, tokenIndex, depth
					if !rules[rulecombinators]() {
						goto l26
					}
					goto l27
				l26:
					position, tokenIndex, depth = position26, tokenIndex26, depth26
				}
			l27:
				depth--
				add(ruleintersect, position25)
			}
			return true
		l24:
			position, tokenIndex, depth = position24, tokenIndex24, depth24
			return false
		},
		/* 4 exclude <- <('-' rangeexpr Action2 combinators?)> */
		func() bool {
			position28, tokenIndex28, depth28 := position, tokenIndex, depth
			{
				position29 := position
				depth++
				if buffer[position] != rune('-') {
					goto l28
				}
				position++
				if !rules[rulerangeexpr]() {
					goto l28
				}
				if !rules[ruleAction2]() {
					goto l28
				}
				{
					position30, tokenIndex30, depth30 := position, tokenIndex, depth
					if !rules[rulecombinators]() {
						goto l30
					}
					goto l31
				l30:
					position, tokenIndex, depth = position30, tokenIndex30, depth30
				}
			l31:
				depth--
				add(ruleexclude, position29)
			}
			return true
		l28:
			position, tokenIndex, depth = position28, tokenIndex28, depth28
			return false
		},
		/* 5 union <- <(',' rangeexpr Action3 combinators?)> */
		func() bool {
			position32, tokenIndex32, depth32 := position, tokenIndex, depth
			{
				position33 := position
				depth++
				if buffer[position] != rune(',') {
					goto l32
				}
				position++
				if !rules[rulerangeexpr]() {
					goto l32
				}
				if !rules[ruleAction3]() {
					goto l32
				}
				{
					position34, tokenIndex34, depth34 := position, tokenIndex, depth
					if !rules[rulecombinators]() {
						goto l34
					}
					goto l35
				l34:
					position, tokenIndex, depth = position34, tokenIndex34, depth34
				}
			l35:
				depth--
				add(ruleunion, position33)
			}
			return true
		l32:
			position, tokenIndex, depth = position32, tokenIndex32, depth32
			return false
		},
		/* 6 braces <- <('{' rangeexpr combinators? '}' rangeexpr? Action4)> */
		func() bool {
			position36, tokenIndex36, depth36 := position, tokenIndex, depth
			{
				position37 := position
				depth++
				if buffer[position] != rune('{') {
					goto l36
				}
				position++
				if !rules[rulerangeexpr]() {
					goto l36
				}
				{
					position38, tokenIndex38, depth38 := position, tokenIndex, depth
					if !rules[rulecombinators]() {
						goto l38
					}
					goto l39
				l38:
					position, tokenIndex, depth = position38, tokenIndex38, depth38
				}
			l39:
				if buffer[position] != rune('}') {
					goto l36
				}
				position++
				{
					position40, tokenIndex40, depth40 := position, tokenIndex, depth
					if !rules[rulerangeexpr]() {
						goto l40
					}
					goto l41
				l40:
					position, tokenIndex, depth = position40, tokenIndex40, depth40
				}
			l41:
				if !rules[ruleAction4]() {
					goto l36
				}
				depth--
				add(rulebraces, position37)
			}
			return true
		l36:
			position, tokenIndex, depth = position36, tokenIndex36, depth36
			return false
		},
		/* 7 brackets <- <('(' rangeexpr combinators? ')')> */
		func() bool {
			position42, tokenIndex42, depth42 := position, tokenIndex, depth
			{
				position43 := position
				depth++
				if buffer[position] != rune('(') {
					goto l42
				}
				position++
				if !rules[rulerangeexpr]() {
					goto l42
				}
				{
					position44, tokenIndex44, depth44 := position, tokenIndex, depth
					if !rules[rulecombinators]() {
						goto l44
					}
					goto l45
				l44:
					position, tokenIndex, depth = position44, tokenIndex44, depth44
				}
			l45:
				if buffer[position] != rune(')') {
					goto l42
				}
				position++
				depth--
				add(rulebrackets, position43)
			}
			return true
		l42:
			position, tokenIndex, depth = position42, tokenIndex42, depth42
			return false
		},
		/* 8 groupq <- <('?' rangeexpr Action5)> */
		func() bool {
			position46, tokenIndex46, depth46 := position, tokenIndex, depth
			{
				position47 := position
				depth++
				if buffer[position] != rune('?') {
					goto l46
				}
				position++
				if !rules[rulerangeexpr]() {
					goto l46
				}
				if !rules[ruleAction5]() {
					goto l46
				}
				depth--
				add(rulegroupq, position47)
			}
			return true
		l46:
			position, tokenIndex, depth = position46, tokenIndex46, depth46
			return false
		},
		/* 9 cluster <- <(('%' literal Action6 key?) / ('%' '{' rangeexpr Action7 '}' key?))> */
		func() bool {
			position48, tokenIndex48, depth48 := position, tokenIndex, depth
			{
				position49 := position
				depth++
				{
					position50, tokenIndex50, depth50 := position, tokenIndex, depth
					if buffer[position] != rune('%') {
						goto l51
					}
					position++
					if !rules[ruleliteral]() {
						goto l51
					}
					if !rules[ruleAction6]() {
						goto l51
					}
					{
						position52, tokenIndex52, depth52 := position, tokenIndex, depth
						if !rules[rulekey]() {
							goto l52
						}
						goto l53
					l52:
						position, tokenIndex, depth = position52, tokenIndex52, depth52
					}
				l53:
					goto l50
				l51:
					position, tokenIndex, depth = position50, tokenIndex50, depth50
					if buffer[position] != rune('%') {
						goto l48
					}
					position++
					if buffer[position] != rune('{') {
						goto l48
					}
					position++
					if !rules[rulerangeexpr]() {
						goto l48
					}
					if !rules[ruleAction7]() {
						goto l48
					}
					if buffer[position] != rune('}') {
						goto l48
					}
					position++
					{
						position54, tokenIndex54, depth54 := position, tokenIndex, depth
						if !rules[rulekey]() {
							goto l54
						}
						goto l55
					l54:
						position, tokenIndex, depth = position54, tokenIndex54, depth54
					}
				l55:
				}
			l50:
				depth--
				add(rulecluster, position49)
			}
			return true
		l48:
			position, tokenIndex, depth = position48, tokenIndex48, depth48
			return false
		},
		/* 10 group <- <('@' rangeexpr Action8)> */
		func() bool {
			position56, tokenIndex56, depth56 := position, tokenIndex, depth
			{
				position57 := position
				depth++
				if buffer[position] != rune('@') {
					goto l56
				}
				position++
				if !rules[rulerangeexpr]() {
					goto l56
				}
				if !rules[ruleAction8]() {
					goto l56
				}
				depth--
				add(rulegroup, position57)
			}
			return true
		l56:
			position, tokenIndex, depth = position56, tokenIndex56, depth56
			return false
		},
		/* 11 key <- <(':' rangeexpr Action9)> */
		func() bool {
			position58, tokenIndex58, depth58 := position, tokenIndex, depth
			{
				position59 := position
				depth++
				if buffer[position] != rune(':') {
					goto l58
				}
				position++
				if !rules[rulerangeexpr]() {
					goto l58
				}
				if !rules[ruleAction9]() {
					goto l58
				}
				depth--
				add(rulekey, position59)
			}
			return true
		l58:
			position, tokenIndex, depth = position58, tokenIndex58, depth58
			return false
		},
		/* 12 localkey <- <('$' literal Action10)> */
		func() bool {
			position60, tokenIndex60, depth60 := position, tokenIndex, depth
			{
				position61 := position
				depth++
				if buffer[position] != rune('$') {
					goto l60
				}
				position++
				if !rules[ruleliteral]() {
					goto l60
				}
				if !rules[ruleAction10]() {
					goto l60
				}
				depth--
				add(rulelocalkey, position61)
			}
			return true
		l60:
			position, tokenIndex, depth = position60, tokenIndex60, depth60
			return false
		},
		/* 13 function <- <(literal Action11 '(' funcargs ')')> */
		func() bool {
			position62, tokenIndex62, depth62 := position, tokenIndex, depth
			{
				position63 := position
				depth++
				if !rules[ruleliteral]() {
					goto l62
				}
				if !rules[ruleAction11]() {
					goto l62
				}
				if buffer[position] != rune('(') {
					goto l62
				}
				position++
				if !rules[rulefuncargs]() {
					goto l62
				}
				if buffer[position] != rune(')') {
					goto l62
				}
				position++
				depth--
				add(rulefunction, position63)
			}
			return true
		l62:
			position, tokenIndex, depth = position62, tokenIndex62, depth62
			return false
		},
		/* 14 funcargs <- <((rangeexpr combinators? Action12 ';' funcargs) / (rangeexpr combinators? Action13))> */
		func() bool {
			position64, tokenIndex64, depth64 := position, tokenIndex, depth
			{
				position65 := position
				depth++
				{
					position66, tokenIndex66, depth66 := position, tokenIndex, depth
					if !rules[rulerangeexpr]() {
						goto l67
					}
					{
						position68, tokenIndex68, depth68 := position, tokenIndex, depth
						if !rules[rulecombinators]() {
							goto l68
						}
						goto l69
					l68:
						position, tokenIndex, depth = position68, tokenIndex68, depth68
					}
				l69:
					if !rules[ruleAction12]() {
						goto l67
					}
					if buffer[position] != rune(';') {
						goto l67
					}
					position++
					if !rules[rulefuncargs]() {
						goto l67
					}
					goto l66
				l67:
					position, tokenIndex, depth = position66, tokenIndex66, depth66
					if !rules[rulerangeexpr]() {
						goto l64
					}
					{
						position70, tokenIndex70, depth70 := position, tokenIndex, depth
						if !rules[rulecombinators]() {
							goto l70
						}
						goto l71
					l70:
						position, tokenIndex, depth = position70, tokenIndex70, depth70
					}
				l71:
					if !rules[ruleAction13]() {
						goto l64
					}
				}
			l66:
				depth--
				add(rulefuncargs, position65)
			}
			return true
		l64:
			position, tokenIndex, depth = position64, tokenIndex64, depth64
			return false
		},
		/* 15 regex <- <('/' <(!'/' .)*> '/' Action14)> */
		func() bool {
			position72, tokenIndex72, depth72 := position, tokenIndex, depth
			{
				position73 := position
				depth++
				if buffer[position] != rune('/') {
					goto l72
				}
				position++
				{
					position74 := position
					depth++
				l75:
					{
						position76, tokenIndex76, depth76 := position, tokenIndex, depth
						{
							position77, tokenIndex77, depth77 := position, tokenIndex, depth
							if buffer[position] != rune('/') {
								goto l77
							}
							position++
							goto l76
						l77:
							position, tokenIndex, depth = position77, tokenIndex77, depth77
						}
						if !matchDot() {
							goto l76
						}
						goto l75
					l76:
						position, tokenIndex, depth = position76, tokenIndex76, depth76
					}
					depth--
					add(rulePegText, position74)
				}
				if buffer[position] != rune('/') {
					goto l72
				}
				position++
				if !rules[ruleAction14]() {
					goto l72
				}
				depth--
				add(ruleregex, position73)
			}
			return true
		l72:
			position, tokenIndex, depth = position72, tokenIndex72, depth72
			return false
		},
		/* 16 literal <- <<(leaderChar ([a-z] / [A-Z] / ([0-9] / [0-9]) / '-' / '_')*)>> */
		func() bool {
			position78, tokenIndex78, depth78 := position, tokenIndex, depth
			{
				position79 := position
				depth++
				{
					position80 := position
					depth++
					if !rules[ruleleaderChar]() {
						goto l78
					}
				l81:
					{
						position82, tokenIndex82, depth82 := position, tokenIndex, depth
						{
							position83, tokenIndex83, depth83 := position, tokenIndex, depth
							if c := buffer[position]; c < rune('a') || c > rune('z') {
								goto l84
							}
							position++
							goto l83
						l84:
							position, tokenIndex, depth = position83, tokenIndex83, depth83
							if c := buffer[position]; c < rune('A') || c > rune('Z') {
								goto l85
							}
							position++
							goto l83
						l85:
							position, tokenIndex, depth = position83, tokenIndex83, depth83
							{
								position87, tokenIndex87, depth87 := position, tokenIndex, depth
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l88
								}
								position++
								goto l87
							l88:
								position, tokenIndex, depth = position87, tokenIndex87, depth87
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l86
								}
								position++
							}
						l87:
							goto l83
						l86:
							position, tokenIndex, depth = position83, tokenIndex83, depth83
							if buffer[position] != rune('-') {
								goto l89
							}
							position++
							goto l83
						l89:
							position, tokenIndex, depth = position83, tokenIndex83, depth83
							if buffer[position] != rune('_') {
								goto l82
							}
							position++
						}
					l83:
						goto l81
					l82:
						position, tokenIndex, depth = position82, tokenIndex82, depth82
					}
					depth--
					add(rulePegText, position80)
				}
				depth--
				add(ruleliteral, position79)
			}
			return true
		l78:
			position, tokenIndex, depth = position78, tokenIndex78, depth78
			return false
		},
		/* 17 value <- <(<(leaderChar (':' / ([a-z] / [A-Z]) / ([0-9] / [0-9]) / '-' / '_' / '.')*)> Action15)> */
		func() bool {
			position90, tokenIndex90, depth90 := position, tokenIndex, depth
			{
				position91 := position
				depth++
				{
					position92 := position
					depth++
					if !rules[ruleleaderChar]() {
						goto l90
					}
				l93:
					{
						position94, tokenIndex94, depth94 := position, tokenIndex, depth
						{
							position95, tokenIndex95, depth95 := position, tokenIndex, depth
							if buffer[position] != rune(':') {
								goto l96
							}
							position++
							goto l95
						l96:
							position, tokenIndex, depth = position95, tokenIndex95, depth95
							{
								position98, tokenIndex98, depth98 := position, tokenIndex, depth
								if c := buffer[position]; c < rune('a') || c > rune('z') {
									goto l99
								}
								position++
								goto l98
							l99:
								position, tokenIndex, depth = position98, tokenIndex98, depth98
								if c := buffer[position]; c < rune('A') || c > rune('Z') {
									goto l97
								}
								position++
							}
						l98:
							goto l95
						l97:
							position, tokenIndex, depth = position95, tokenIndex95, depth95
							{
								position101, tokenIndex101, depth101 := position, tokenIndex, depth
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l102
								}
								position++
								goto l101
							l102:
								position, tokenIndex, depth = position101, tokenIndex101, depth101
								if c := buffer[position]; c < rune('0') || c > rune('9') {
									goto l100
								}
								position++
							}
						l101:
							goto l95
						l100:
							position, tokenIndex, depth = position95, tokenIndex95, depth95
							if buffer[position] != rune('-') {
								goto l103
							}
							position++
							goto l95
						l103:
							position, tokenIndex, depth = position95, tokenIndex95, depth95
							if buffer[position] != rune('_') {
								goto l104
							}
							position++
							goto l95
						l104:
							position, tokenIndex, depth = position95, tokenIndex95, depth95
							if buffer[position] != rune('.') {
								goto l94
							}
							position++
						}
					l95:
						goto l93
					l94:
						position, tokenIndex, depth = position94, tokenIndex94, depth94
					}
					depth--
					add(rulePegText, position92)
				}
				if !rules[ruleAction15]() {
					goto l90
				}
				depth--
				add(rulevalue, position91)
			}
			return true
		l90:
			position, tokenIndex, depth = position90, tokenIndex90, depth90
			return false
		},
		/* 18 leaderChar <- <([a-z] / [A-Z] / ([0-9] / [0-9]) / '.' / '_')> */
		func() bool {
			position105, tokenIndex105, depth105 := position, tokenIndex, depth
			{
				position106 := position
				depth++
				{
					position107, tokenIndex107, depth107 := position, tokenIndex, depth
					if c := buffer[position]; c < rune('a') || c > rune('z') {
						goto l108
					}
					position++
					goto l107
				l108:
					position, tokenIndex, depth = position107, tokenIndex107, depth107
					if c := buffer[position]; c < rune('A') || c > rune('Z') {
						goto l109
					}
					position++
					goto l107
				l109:
					position, tokenIndex, depth = position107, tokenIndex107, depth107
					{
						position111, tokenIndex111, depth111 := position, tokenIndex, depth
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l112
						}
						position++
						goto l111
					l112:
						position, tokenIndex, depth = position111, tokenIndex111, depth111
						if c := buffer[position]; c < rune('0') || c > rune('9') {
							goto l110
						}
						position++
					}
				l111:
					goto l107
				l110:
					position, tokenIndex, depth = position107, tokenIndex107, depth107
					if buffer[position] != rune('.') {
						goto l113
					}
					position++
					goto l107
				l113:
					position, tokenIndex, depth = position107, tokenIndex107, depth107
					if buffer[position] != rune('_') {
						goto l105
					}
					position++
				}
			l107:
				depth--
				add(ruleleaderChar, position106)
			}
			return true
		l105:
			position, tokenIndex, depth = position105, tokenIndex105, depth105
			return false
		},
		/* 19 space <- <' '*> */
		func() bool {
			{
				position115 := position
				depth++
			l116:
				{
					position117, tokenIndex117, depth117 := position, tokenIndex, depth
					if buffer[position] != rune(' ') {
						goto l117
					}
					position++
					goto l116
				l117:
					position, tokenIndex, depth = position117, tokenIndex117, depth117
				}
				depth--
				add(rulespace, position115)
			}
			return true
		},
		/* 20 const <- <(q / quoted)> */
		func() bool {
			position118, tokenIndex118, depth118 := position, tokenIndex, depth
			{
				position119 := position
				depth++
				{
					position120, tokenIndex120, depth120 := position, tokenIndex, depth
					if !rules[ruleq]() {
						goto l121
					}
					goto l120
				l121:
					position, tokenIndex, depth = position120, tokenIndex120, depth120
					if !rules[rulequoted]() {
						goto l118
					}
				}
			l120:
				depth--
				add(ruleconst, position119)
			}
			return true
		l118:
			position, tokenIndex, depth = position118, tokenIndex118, depth118
			return false
		},
		/* 21 q <- <('q' '(' <(!')' .)*> ')' Action16)> */
		func() bool {
			position122, tokenIndex122, depth122 := position, tokenIndex, depth
			{
				position123 := position
				depth++
				if buffer[position] != rune('q') {
					goto l122
				}
				position++
				if buffer[position] != rune('(') {
					goto l122
				}
				position++
				{
					position124 := position
					depth++
				l125:
					{
						position126, tokenIndex126, depth126 := position, tokenIndex, depth
						{
							position127, tokenIndex127, depth127 := position, tokenIndex, depth
							if buffer[position] != rune(')') {
								goto l127
							}
							position++
							goto l126
						l127:
							position, tokenIndex, depth = position127, tokenIndex127, depth127
						}
						if !matchDot() {
							goto l126
						}
						goto l125
					l126:
						position, tokenIndex, depth = position126, tokenIndex126, depth126
					}
					depth--
					add(rulePegText, position124)
				}
				if buffer[position] != rune(')') {
					goto l122
				}
				position++
				if !rules[ruleAction16]() {
					goto l122
				}
				depth--
				add(ruleq, position123)
			}
			return true
		l122:
			position, tokenIndex, depth = position122, tokenIndex122, depth122
			return false
		},
		/* 22 quoted <- <('"' <(!'"' .)*> '"' Action17)> */
		func() bool {
			position128, tokenIndex128, depth128 := position, tokenIndex, depth
			{
				position129 := position
				depth++
				if buffer[position] != rune('"') {
					goto l128
				}
				position++
				{
					position130 := position
					depth++
				l131:
					{
						position132, tokenIndex132, depth132 := position, tokenIndex, depth
						{
							position133, tokenIndex133, depth133 := position, tokenIndex, depth
							if buffer[position] != rune('"') {
								goto l133
							}
							position++
							goto l132
						l133:
							position, tokenIndex, depth = position133, tokenIndex133, depth133
						}
						if !matchDot() {
							goto l132
						}
						goto l131
					l132:
						position, tokenIndex, depth = position132, tokenIndex132, depth132
					}
					depth--
					add(rulePegText, position130)
				}
				if buffer[position] != rune('"') {
					goto l128
				}
				position++
				if !rules[ruleAction17]() {
					goto l128
				}
				depth--
				add(rulequoted, position129)
			}
			return true
		l128:
			position, tokenIndex, depth = position128, tokenIndex128, depth128
			return false
		},
		/* 24 Action0 <- <{ p.addBraceStart() }> */
		func() bool {
			{
				add(ruleAction0, position)
			}
			return true
		},
		/* 25 null <- <> */
		func() bool {
			{
				position137 := position
				depth++
				depth--
				add(rulenull, position137)
			}
			return true
		},
		/* 26 Action1 <- <{ p.addOperator(operatorIntersect) }> */
		func() bool {
			{
				add(ruleAction1, position)
			}
			return true
		},
		/* 27 Action2 <- <{ p.addOperator(operatorSubtract) }> */
		func() bool {
			{
				add(ruleAction2, position)
			}
			return true
		},
		/* 28 Action3 <- <{ p.addOperator(operatorUnion) }> */
		func() bool {
			{
				add(ruleAction3, position)
			}
			return true
		},
		/* 29 Action4 <- <{ p.addBraces() }> */
		func() bool {
			{
				add(ruleAction4, position)
			}
			return true
		},
		/* 30 Action5 <- <{ p.addGroupQuery() }> */
		func() bool {
			{
				add(ruleAction5, position)
			}
			return true
		},
		/* 31 Action6 <- <{ p.addValue(buffer[begin:end]); p.addClusterLookup() }> */
		func() bool {
			{
				add(ruleAction6, position)
			}
			return true
		},
		/* 32 Action7 <- <{ p.addClusterLookup() }> */
		func() bool {
			{
				add(ruleAction7, position)
			}
			return true
		},
		/* 33 Action8 <- <{ p.addGroupLookup() }> */
		func() bool {
			{
				add(ruleAction8, position)
			}
			return true
		},
		/* 34 Action9 <- <{ p.addKeyLookup() }> */
		func() bool {
			{
				add(ruleAction9, position)
			}
			return true
		},
		/* 35 Action10 <- <{ p.addLocalClusterLookup(buffer[begin:end]) }> */
		func() bool {
			{
				add(ruleAction10, position)
			}
			return true
		},
		/* 36 Action11 <- <{ p.addFunction(buffer[begin:end]) }> */
		func() bool {
			{
				add(ruleAction11, position)
			}
			return true
		},
		/* 37 Action12 <- <{ p.addFuncArg() }> */
		func() bool {
			{
				add(ruleAction12, position)
			}
			return true
		},
		/* 38 Action13 <- <{ p.addFuncArg() }> */
		func() bool {
			{
				add(ruleAction13, position)
			}
			return true
		},
		nil,
		/* 40 Action14 <- <{ p.addRegex(buffer[begin:end]) }> */
		func() bool {
			{
				add(ruleAction14, position)
			}
			return true
		},
		/* 41 Action15 <- <{ p.addValue(buffer[begin:end]) }> */
		func() bool {
			{
				add(ruleAction15, position)
			}
			return true
		},
		/* 42 Action16 <- <{ p.addConstant(buffer[begin:end]) }> */
		func() bool {
			{
				add(ruleAction16, position)
			}
			return true
		},
		/* 43 Action17 <- <{ p.addConstant(buffer[begin:end]) }> */
		func() bool {
			{
				add(ruleAction17, position)
			}
			return true
		},
	}
	p.rules = rules
}
