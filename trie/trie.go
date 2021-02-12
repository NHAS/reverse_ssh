package trie

type Trie struct {
	root     bool
	c        byte
	children map[byte][]*Trie
}

func (t *Trie) GetAllUnderneath() (result []string) {
	if len(t.children) == 0 {

		return []string{string(t.c)}
	}

	for _, v := range t.children {
		for i := range v {
			r := v[i].GetAllUnderneath()
			for _, ii := range r {
				result = append(result, string(t.c)+ii)
			}
		}
	}

	return result
}

func (t *Trie) PrefixMatch(s string) (result []string) {

	if len(s) == 0 {
		for _, v := range t.children {
			for x := range v {
				n := v[x].GetAllUnderneath()

				result = append(result, n...)
			}
		}
		return result
	}

	for _, v := range t.children[s[0]] {

		n := v.PrefixMatch(s[1:])

		result = append(result, n...)
	}

	return result
}

func (t *Trie) Add(s string) {
	if len(s) == 0 {
		return
	}
	newChild := &Trie{children: make(map[byte][]*Trie)}
	if t.root {

		t.children[s[0]] = append(t.children[s[0]], newChild)
		newChild.Add(s)
		return
	}

	t.c = s[0]

	if len(s) > 1 {

		t.children[s[1]] = append(t.children[s[1]], newChild)
		newChild.Add(s[1:])
	}

}

func NewTrie() *Trie {
	t := &Trie{
		children: make(map[byte][]*Trie),
		root:     true,
	}

	return t
}
