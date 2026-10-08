package docx

import (
	"strconv"

	"github.com/askrejans/crowdoc/v2/ast"
)

type numLevel struct {
	format string // w:numFmt
	text   string // w:lvlText
	start  int
	pStyle string
}

type abstractNum struct {
	levels       map[int]*numLevel
	numStyleLink string
}

type numOverride struct {
	start    int
	hasStart bool
	level    *numLevel
}

type numInstance struct {
	abstractID string
	overrides  map[int]*numOverride
}

// numCounter tracks running numbers of one abstract numbering definition.
// Word continues numbering across w:num instances sharing an abstract
// definition unless an instance restarts a level with w:startOverride.
type numCounter struct {
	counts  [9]int
	started [9]bool
	seen    map[string]bool
}

type numbering struct {
	abstracts map[string]*abstractNum
	nums      map[string]*numInstance
	styles    *styleSheet
	counters  map[string]*numCounter
}

func parseNumbering(root *node, styles *styleSheet) *numbering {
	nb := &numbering{
		abstracts: map[string]*abstractNum{},
		nums:      map[string]*numInstance{},
		styles:    styles,
		counters:  map[string]*numCounter{},
	}
	if root == nil {
		return nb
	}
	for _, k := range root.kids {
		switch k.name {
		case "abstractNum":
			a := &abstractNum{levels: map[int]*numLevel{}, numStyleLink: k.val("numStyleLink")}
			for _, l := range k.childrenNamed("lvl") {
				if ilvl, err := strconv.Atoi(l.attr("ilvl")); err == nil && ilvl >= 0 && ilvl < 9 {
					a.levels[ilvl] = parseLevel(l)
				}
			}
			nb.abstracts[k.attr("abstractNumId")] = a
		case "num":
			n := &numInstance{abstractID: k.val("abstractNumId"), overrides: map[int]*numOverride{}}
			for _, o := range k.childrenNamed("lvlOverride") {
				ilvl, err := strconv.Atoi(o.attr("ilvl"))
				if err != nil || ilvl < 0 || ilvl >= 9 {
					continue
				}
				ov := &numOverride{}
				if so := o.child("startOverride"); so != nil {
					if v, err := strconv.Atoi(so.attr("val")); err == nil {
						ov.start, ov.hasStart = v, true
					}
				}
				if l := o.child("lvl"); l != nil {
					ov.level = parseLevel(l)
				}
				n.overrides[ilvl] = ov
			}
			nb.nums[k.attr("numId")] = n
		}
	}
	return nb
}

func parseLevel(l *node) *numLevel {
	lv := &numLevel{format: l.val("numFmt"), text: l.val("lvlText"), start: 1, pStyle: l.val("pStyle")}
	if s := l.child("start"); s != nil {
		if v, err := strconv.Atoi(s.attr("val")); err == nil {
			lv.start = v
		}
	}
	if lv.format == "" {
		// Producers that omit numFmt get Word's default.
		lv.format = "decimal"
	}
	return lv
}

// abstract resolves the abstract definition of num numID, following
// numbering-style links, and returns it with its identity key.
func (nb *numbering) abstract(numID string) (*abstractNum, string) {
	n := nb.nums[numID]
	if n == nil {
		return nil, ""
	}
	key := n.abstractID
	a := nb.abstracts[key]
	for i := 0; a != nil && a.numStyleLink != "" && i < 8; i++ {
		var linked string
		if s := nb.styles.byID[a.numStyleLink]; s != nil {
			linked = s.ppr.numID
		}
		ln := nb.nums[linked]
		if ln == nil || nb.abstracts[ln.abstractID] == nil || ln.abstractID == key {
			break
		}
		key = ln.abstractID
		a = nb.abstracts[key]
	}
	return a, key
}

// level returns the effective level definition of numID at ilvl.
func (nb *numbering) level(numID string, ilvl int) *numLevel {
	if n := nb.nums[numID]; n != nil {
		if ov := n.overrides[ilvl]; ov != nil && ov.level != nil {
			return ov.level
		}
	}
	a, _ := nb.abstract(numID)
	if a == nil {
		return nil
	}
	return a.levels[ilvl]
}

// levelForStyle finds the level whose w:pStyle links to paragraph style id.
func (nb *numbering) levelForStyle(numID, styleID string) (int, bool) {
	a, _ := nb.abstract(numID)
	if a == nil || styleID == "" {
		return 0, false
	}
	for i := 0; i < 9; i++ {
		if l := a.levels[i]; l != nil && l.pStyle == styleID {
			return i, true
		}
	}
	return 0, false
}

// next advances the counter for numID at ilvl and returns the item number.
func (nb *numbering) next(numID string, ilvl int) int {
	_, key := nb.abstract(numID)
	if key == "" {
		key = "num:" + numID
	}
	c := nb.counters[key]
	if c == nil {
		c = &numCounter{seen: map[string]bool{}}
		nb.counters[key] = c
	}
	seenKey := numID + ":" + strconv.Itoa(ilvl)
	if !c.seen[seenKey] {
		c.seen[seenKey] = true
		if n := nb.nums[numID]; n != nil {
			if ov := n.overrides[ilvl]; ov != nil && ov.hasStart {
				c.counts[ilvl] = ov.start - 1
				c.started[ilvl] = true
			}
		}
	}
	if !c.started[ilvl] {
		start := 1
		if l := nb.level(numID, ilvl); l != nil {
			start = l.start
		}
		c.counts[ilvl] = start - 1
		c.started[ilvl] = true
	}
	c.counts[ilvl]++
	for k := ilvl + 1; k < 9; k++ {
		c.started[k] = false
	}
	return c.counts[ilvl]
}

// listRef describes a numbered paragraph.
type listRef struct {
	numID   string
	ilvl    int
	ordered bool
	style   ast.NumberStyle
	number  int
	task    ast.TaskState
}

// listFormat maps a w:numFmt to list kind and marker style. ok is false
// for levels that draw no marker at all.
func listFormat(l *numLevel) (ordered bool, style ast.NumberStyle, ok bool) {
	if l == nil {
		return false, ast.NumberDecimal, true
	}
	switch l.format {
	case "bullet":
		return false, ast.NumberDecimal, true
	case "none":
		return false, ast.NumberDecimal, l.text != ""
	case "lowerLetter":
		return true, ast.NumberLowerAlpha, true
	case "upperLetter":
		return true, ast.NumberUpperAlpha, true
	case "lowerRoman":
		return true, ast.NumberLowerRoman, true
	case "upperRoman":
		return true, ast.NumberUpperRoman, true
	}
	return true, ast.NumberDecimal, true
}
