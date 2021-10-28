package clients

import (
	"fmt"
	"sync"

	"github.com/NHAS/reverse_ssh/internal"
	"golang.org/x/crypto/ssh"
)

var lock sync.RWMutex
var clients = map[string]*ssh.ServerConn{}

var uniqueIdToAllAliases = map[string][]string{}
var aliases = map[string]map[string]bool{}

func Add(conn *ssh.ServerConn) (string, error) {
	lock.Lock()
	defer lock.Unlock()

	idString, err := internal.RandomString(20)
	if err != nil {
		return "", err
	}

	if _, ok := aliases[conn.User()]; !ok {
		aliases[conn.User()] = make(map[string]bool)
	}

	uniqueIdToAllAliases[idString] = append(uniqueIdToAllAliases[idString], conn.User())
	aliases[conn.User()][idString] = true

	if _, ok := aliases[conn.RemoteAddr().String()]; !ok {
		aliases[conn.RemoteAddr().String()] = make(map[string]bool)
	}

	uniqueIdToAllAliases[idString] = append(uniqueIdToAllAliases[idString], conn.RemoteAddr().String())
	aliases[conn.RemoteAddr().String()][idString] = true

	clients[idString] = conn
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
		matchingHosts = matchingHosts[:len(matchingHosts)-1]

		return nil, fmt.Errorf("%d connections match alias '%s'\n%s", matches, identifier, matchingHosts)

	}

	return nil, fmt.Errorf("Not found. It could be that you are using the 'user@hostname' format, unfortunately due to limitations of ssh we cant do that. Try using 'user.host' instead!")
}

func Remove(uniqueId string) {
	lock.Lock()
	defer lock.Unlock()

	if _, ok := clients[uniqueId]; !ok {
		//	panic("Somehow a unqiue ID is being removed without being in the set, this is a programming issue guy")
	}

	delete(clients, uniqueId)

	if currentAliases, ok := uniqueIdToAllAliases[uniqueId]; ok {
		for _, alias := range currentAliases {
			if len(alias) == 1 {
				delete(aliases, alias)
				continue
			}

			delete(aliases[alias], uniqueId)
		}
		delete(uniqueIdToAllAliases, uniqueId)
	}

}
