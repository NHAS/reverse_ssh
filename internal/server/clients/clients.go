package clients

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/trie"
	"golang.org/x/crypto/ssh"
)

var (
	lock       sync.RWMutex
	allClients = map[string]*ssh.ServerConn{}

	clientIdToOwner = map[string]string{}
	// Owner username, like jim/admin/none
	ownerToClientIds = map[string][]string{}

	uniqueIdToAllAliases = map[string][]string{}
	aliases              = map[string]map[string]bool{}

	usernameRegex = regexp.MustCompile(`[^\w-]`)

	autocompletes = map[string]*trie.Trie{}
)

func Autocomplete(username string) *trie.Trie {
	lock.RLock()
	defer lock.RUnlock()

	if t, ok := autocompletes[username]; ok {
		return t
	}

	ret := trie.NewTrie()

	for _, idString := range ownerToClientIds[username] {

		ret.Add(idString)
		for _, v := range uniqueIdToAllAliases[idString] {
			ret.Add(v)
		}
	}

	autocompletes[username] = ret

	return ret
}

func NormaliseHostname(hostname string) string {
	hostname = strings.ToLower(hostname)

	hostname = usernameRegex.ReplaceAllString(hostname, ".")

	return hostname
}

func Add(conn *ssh.ServerConn) (string, string, error) {
	lock.Lock()
	defer lock.Unlock()

	idString, err := internal.RandomString(20)
	if err != nil {
		return "", "", err
	}

	username := NormaliseHostname(conn.User())

	addAlias(idString, username)
	addAlias(idString, conn.RemoteAddr().String())
	addAlias(idString, conn.Permissions.Extensions["pubkey-fp"])
	if conn.Permissions.Extensions["comment"] != "" {
		addAlias(idString, conn.Permissions.Extensions["comment"])
	}
	allClients[idString] = conn

	// If we cant unmarshal the owner, dont error, just mark it as none, an admin can then come along and assign it. So people dont lose a connection
	owners := strings.Split(conn.Permissions.Extensions["owners"], ",")

	for _, owner := range owners {

		ownerToClientIds[owner] = append(ownerToClientIds[owner], idString)
		clientIdToOwner[idString] = owner

		_, ok := autocompletes[owner]
		if !ok {
			autocompletes[owner] = trie.NewTrie()
		}

		autocompletes[owner].Add(idString)
		for _, v := range uniqueIdToAllAliases[idString] {
			autocompletes[owner].Add(v)
		}
	}

	return idString, username, nil

}

func addAlias(uniqueId, newAlias string) {
	if _, ok := aliases[newAlias]; !ok {
		aliases[newAlias] = make(map[string]bool)
	}

	uniqueIdToAllAliases[uniqueId] = append(uniqueIdToAllAliases[uniqueId], newAlias)
	aliases[newAlias][uniqueId] = true
}

func Search(filter string) (out map[string]*ssh.ServerConn, err error) {

	filter = filter + "*"
	_, err = filepath.Match(filter, "")
	if err != nil {
		return nil, fmt.Errorf("filter is not well formed")
	}

	out = make(map[string]*ssh.ServerConn)

	lock.RLock()
	defer lock.RUnlock()

	for id, conn := range allClients {
		if filter == "" {
			out[id] = conn
			continue
		}

		if _matches(filter, id, conn.RemoteAddr().String()) {
			out[id] = conn
			continue
		}

	}

	return
}

func _matches(filter, clientId, remoteAddr string) bool {
	match, _ := filepath.Match(filter, clientId)
	if match {
		return true
	}

	for _, alias := range uniqueIdToAllAliases[clientId] {
		match, _ = filepath.Match(filter, alias)
		if match {
			return true
		}
	}

	match, _ = filepath.Match(filter, remoteAddr)
	return match
}

func Matches(filter, clientId, remoteAddr string) bool {
	lock.RLock()
	defer lock.RUnlock()

	return _matches(filter, clientId, remoteAddr)
}

func Get(identifier string) (ssh.Conn, error) {
	lock.RLock()
	defer lock.RUnlock()

	if m, ok := allClients[identifier]; ok {
		return m, nil
	}

	if m, ok := aliases[identifier]; ok {
		if len(m) == 1 {
			for k := range m {
				return allClients[k], nil
			}
		}

		matches := 0
		matchingHosts := ""
		for k := range m {
			matches++
			client := allClients[k]
			matchingHosts += fmt.Sprintf("%s (%s %s)\n", k, client.User(), client.RemoteAddr().String())
		}

		if len(matchingHosts) > 0 {
			matchingHosts = matchingHosts[:len(matchingHosts)-1]
		}
		return nil, fmt.Errorf("%d connections match alias '%s'\n%s", matches, identifier, matchingHosts)

	}

	return nil, fmt.Errorf("%s not found", identifier)
}

func Remove(uniqueId string) {
	lock.Lock()
	defer lock.Unlock()

	if _, ok := allClients[uniqueId]; !ok {
		//If this is already removed then we dont need to remove it again.
		return
	}

	delete(allClients, uniqueId)

	owner, ok := clientIdToOwner[uniqueId]
	autocomplete := autocompletes[owner]

	autocomplete.Remove(uniqueId)

	if ok {
		ids := ownerToClientIds[owner]

		ownerToClientIds[owner] = slices.DeleteFunc(ids, func(s string) bool {
			return s == uniqueId
		})
	}

	delete(clientIdToOwner, uniqueId)

	if currentAliases, ok := uniqueIdToAllAliases[uniqueId]; ok {

		for _, alias := range currentAliases {
			if len(aliases[alias]) <= 1 {
				autocomplete.Remove(alias)
				delete(aliases, alias)
			}

			delete(aliases[alias], uniqueId)
		}
		delete(uniqueIdToAllAliases, uniqueId)
	}

}
