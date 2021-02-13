package trie

type Trie struct {
	root     bool
	c        byte
	children map[byte]*Trie
}

func (t *Trie) Add(s string) {
	if len(s) == 0 {
		return
	}

	if child, ok := t.children[s[0]]; ok {
		child.Add(s[1:])
		return
	}

	newChild := &Trie{children: make(map[byte]*Trie)}
	newChild.c = s[0]
	t.children[s[0]] = newChild
	newChild.Add(s[1:])

}

func (t *Trie) getAll() (result []string) {
	if len(t.children) == 0 {
		return []string{string(t.c)}
	}
	for _, c := range t.children {
		for _, n := range c.getAll() {
			result = append(result, string(t.c)+n)
		}
	}

	return result
}

func (t *Trie) PrefixMatch(prefix string) (result []string) {
	if len(prefix) == 0 {
		for _, child := range t.children {
			result = append(result, child.getAll()...)
		}
		return result
	}

	if child, ok := t.children[prefix[0]]; ok {
		return child.PrefixMatch(prefix[1:])
	}

	return []string{} // No matches

}

func (t *Trie) Remove(s string) {

}

func NewTrie() *Trie {
	t := &Trie{
		children: make(map[byte]*Trie),
		root:     true,
	}

	return t
}
