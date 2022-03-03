package clients

import (
	"fmt"
	"strings"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/trie"
	"golang.org/x/crypto/ssh"
)

var lock sync.RWMutex
var clients = map[string]*ssh.ServerConn{}

var Autocomplete = trie.NewTrie()

var uniqueIdToAllAliases = map[string][]string{}
var aliases = map[string]map[string]bool{}

func Add(conn *ssh.ServerConn) (string, error) {
	lock.Lock()
	defer lock.Unlock()

	idString, err := internal.RandomString(20)
	if err != nil {
		return "", err
	}

	username := strings.ToLower(conn.User())

	if _, ok := aliases[username]; !ok {
		aliases[username] = make(map[string]bool)
	}

	uniqueIdToAllAliases[idString] = append(uniqueIdToAllAliases[idString], username)
	aliases[username][idString] = true

	if _, ok := aliases[conn.RemoteAddr().String()]; !ok {
		aliases[conn.RemoteAddr().String()] = make(map[string]bool)
	}

	uniqueIdToAllAliases[idString] = append(uniqueIdToAllAliases[idString], conn.RemoteAddr().String())
	aliases[conn.RemoteAddr().String()][idString] = true

	clients[idString] = conn

	Autocomplete.Add(idString)
	for _, v := range uniqueIdToAllAliases[idString] {
		Autocomplete.Add(v)
	}

	return idString, nil

}

func GetAll() map[string]ssh.Conn {
	lock.RLock()
	defer lock.RUnlock()

	//Defensive copy
	out := map[string]ssh.Conn{}
	for k, v := range clients {
		out[k] = v
	}

	return out
}

func Get(identifier string) (ssh.Conn, error) {
	lock.RLock()
	defer lock.RUnlock()

	if m, ok := clients[identifier]; ok {
		return m, nil
	}

	if m, ok := aliases[identifier]; ok {
		if len(m) == 1 {
			for k := range m {
				return clients[k], nil
			}
		}

		matches := 0
		matchingHosts := ""
		for k := range m {
			matches++
			client := clients[k]
			matchingHosts += fmt.Sprintf("%s (%s %s)\n", k, client.User(), client.RemoteAddr().String())
		}

		if len(matchingHosts) > 0 {
			matchingHosts = matchingHosts[:len(matchingHosts)-1]
		}
		return nil, fmt.Errorf("%d connections match alias '%s'\n%s", matches, identifier, matchingHosts)

	}

	return nil, fmt.Errorf("%s Not found.", identifier)
}

func Remove(uniqueId string) {
	lock.Lock()
	defer lock.Unlock()

	if _, ok := clients[uniqueId]; !ok {
		panic("Somehow a unqiue ID is being removed without being in the set, this is a programming issue guy")
	}

	Autocomplete.Remove(uniqueId)
	delete(clients, uniqueId)

	if currentAliases, ok := uniqueIdToAllAliases[uniqueId]; ok {

		for _, alias := range currentAliases {
			delete(aliases[alias], uniqueId)

			if len(aliases[alias]) <= 1 {
				Autocomplete.Remove(alias)
				delete(aliases, alias)
			}
		}
		delete(uniqueIdToAllAliases, uniqueId)
	}

}
