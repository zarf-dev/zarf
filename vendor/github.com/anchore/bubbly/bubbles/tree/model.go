package tree

import (
	"errors"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/scylladb/go-set/strset"

	"github.com/anchore/bubbly"
)

var _ tea.Model = (*Model)(nil)

type Model struct {
	roots    []string
	nodes    map[string]bubbly.VisibleModel
	children map[string][]string
	parents  map[string]string
	lock     *sync.RWMutex

	// formatting options

	Margin                    string
	Indent                    string
	Fork                      string
	Branch                    string
	Leaf                      string
	Padding                   string
	VerticalPadMultilineNodes bool
	RootsWithoutPrefix        bool
}

func NewModel() Model {
	return Model{
		nodes:    make(map[string]bubbly.VisibleModel),
		children: make(map[string][]string),
		parents:  make(map[string]string),
		lock:     &sync.RWMutex{},

		// formatting options

		Margin:                    "",
		Indent:                    "   ",
		Branch:                    "│  ",
		Fork:                      "├──",
		Leaf:                      "└──",
		Padding:                   "",
		VerticalPadMultilineNodes: false,
		RootsWithoutPrefix:        false,
	}
}

func (m *Model) Add(parent string, id string, model bubbly.VisibleModel) error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if id == "" {
		return errors.New("id cannot be empty")
	}

	m.nodes[id] = model
	if parent != "" {
		m.children[parent] = append(m.children[parent], id)
		m.parents[id] = parent
	} else {
		m.roots = append(m.roots, id)
	}

	return nil
}

func (m *Model) Remove(id string) {
	m.lock.Lock()
	defer m.lock.Unlock()

	delete(m.nodes, id)
	delete(m.children, id)
	delete(m.parents, id)
	for _, children := range m.children {
		for i, child := range children {
			if child == id {
				m.children[child] = append(children[:i], children[i+1:]...)
			}
		}
	}

	for i, node := range m.roots {
		if node == id {
			m.roots = append(m.roots[:i], m.roots[i+1:]...)
		}
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds tea.Cmd
	for id := range m.nodes {
		model, cmd := m.nodes[id].Update(msg)
		if cmd != nil {
			cmds = tea.Batch(cmds, cmd)
		}
		m.nodes[id] = model.(bubbly.VisibleModel)
	}

	return m, cmds
}

func (m Model) View() string {
	sb := strings.Builder{}

	observed := strset.New()

	for i, id := range m.roots {
		ret := m.renderNode(i, id, observed, 0, []bool{m.isLastElement(i, m.roots)})
		if len(ret) > 0 {
			sb.WriteString(ret)
		}
	}

	// optionally add a margin to the left of the entire tree
	if m.Margin != "" {
		lines := strings.Split(strings.TrimRight(sb.String(), "\n"), "\n")
		sb = strings.Builder{}
		for i, line := range lines {
			sb.WriteString(m.Margin)
			sb.WriteString(line)
			if i != len(lines)-1 {
				sb.WriteString("\n")
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

func (m Model) renderNode(siblingIdx int, id string, observed *strset.Set, depth int, path []bool) string {
	if observed.Has(id) {
		return ""
	}

	observed.Add(id)

	node := m.nodes[id]

	if !node.IsVisible() {
		return ""
	}

	prefix := strings.Builder{}

	// handle indentation and prefixes for each level

	for i, isIndent := range path[:depth] {
		if m.RootsWithoutPrefix && i == 0 {
			prefix.WriteString(m.Padding)
			continue
		}
		if isIndent {
			prefix.WriteString(m.Indent)
		} else {
			prefix.WriteString(m.Branch)
		}
		prefix.WriteString(m.Padding)
	}

	// determine the correct prefix (fork or leaf)
	if m.RootsWithoutPrefix && depth > 0 || !m.RootsWithoutPrefix {
		prefix.WriteString(m.forkOrLeaf(siblingIdx, id))
	}

	sb := strings.Builder{}

	// add the node's view
	current := node.View()
	if len(current) > 0 {
		sb.WriteString(m.prefixLines(current, prefix.String(), m.hasChildren(id)))
		sb.WriteString("\n")
	}

	// process all children
	for i, childID := range m.children[id] {
		_, ok := m.nodes[childID]
		if ok && !observed.Has(childID) {
			newPath := append([]bool(nil), path...)
			newPath = append(newPath, m.isLastElement(i, m.children[id]))
			sb.WriteString(m.renderNode(i, childID, observed, depth+1, newPath))
		}
	}

	return sb.String()
}

func (m Model) isLastElement(idx int, siblings []string) bool {
	// check if this is the last visible element in the list of siblings
	for i := idx + 1; i < len(siblings); i++ {
		if m.nodes[siblings[i]].IsVisible() {
			return false
		}
	}
	return true
}

func (m Model) hasChildren(id string) bool {
	// check if there are any children that are visible
	for _, childID := range m.children[id] {
		if m.nodes[childID].IsVisible() {
			return true
		}
	}
	return false
}

func (m Model) forkOrLeaf(siblingIdx int, id string) string {
	if parent, exists := m.parents[id]; exists {
		// index relative to the parent's "children" list
		if m.isLastElement(siblingIdx, m.children[parent]) {
			return m.Leaf
		}
		return m.Fork
	}

	// index relative to the root nodes
	if m.isLastElement(siblingIdx, m.roots) {
		return m.Leaf
	}
	return m.Fork
}

func (m Model) prefixLines(input, prefix string, hasChildren bool) string {
	lines := strings.Split(strings.TrimRight(input, "\n"), "\n")
	sb := strings.Builder{}
	nextPrefix := strings.ReplaceAll(prefix, m.Fork, m.Branch)
	nextPrefix = strings.ReplaceAll(nextPrefix, m.Leaf, m.Indent)

	doPadding := m.VerticalPadMultilineNodes && len(lines) > 1

	for i, line := range lines {
		if i == 0 {
			sb.WriteString(prefix)
		} else {
			sb.WriteString(nextPrefix)
		}
		sb.WriteString(line)
		if doPadding || i != len(lines)-1 {
			sb.WriteString("\n")
		}
	}

	if doPadding {
		sb.WriteString(nextPrefix)
		if hasChildren {
			sb.WriteString(m.Branch)
		}
	}

	return sb.String()
}
