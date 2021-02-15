package trie

import (
	"sync"
)

type Trie struct {
	root     bool
	c        byte
	children map[byte]*Trie
	mut      sync.RWMutex
}

func (t *Trie) Add(s string) {
	if t.root {
		t.mut.Lock()
		defer t.mut.Unlock()
	}

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

	prefix := string(t.c)
	if t.root {
		prefix = ""
	}
	for _, c := range t.children {
		for _, n := range c.getAll() {
			result = append(result, prefix+n)
		}
	}

	return result
}

func (t *Trie) PrefixMatch(prefix string) (result []string) {
	if t.root {
		t.mut.RLock()
		defer t.mut.RUnlock()
	}

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

func (t *Trie) Remove(s string) bool {
	if t.root {
		t.mut.Lock()
		defer t.mut.Unlock()
	}

	if len(s) == 0 {
		return len(t.children) == 0
	}

	if len(t.children) == 0 {
		return true
	}

	if child, ok := t.children[s[0]]; ok && child.Remove(s[1:]) {
		delete(t.children, s[0])
		return len(t.children) == 0
	}

	return false
}

func NewTrie() *Trie {
	t := &Trie{
		children: make(map[byte]*Trie),
		root:     true,
	}

	return t
}
