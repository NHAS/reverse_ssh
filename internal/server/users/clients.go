package users

import (
	"regexp"
	"strings"

	"github.com/NHAS/reverse_ssh/internal"
	"github.com/NHAS/reverse_ssh/pkg/trie"
	"golang.org/x/crypto/ssh"
)

var (
	allClients = map[string]*ssh.ServerConn{}

	ownedByAll = map[string]*ssh.ServerConn{}

	uniqueIdToAllAliases = map[string][]string{}

	// alias to uniqueID
	aliases = map[string]map[string]bool{}

	usernameRegex = regexp.MustCompile(`[^\w-]`)

	globalAutoComplete = trie.NewTrie()

	PublicClientsAutoComplete = trie.NewTrie()
)

func NormaliseHostname(hostname string) string {
	hostname = strings.ToLower(hostname)

	hostname = usernameRegex.ReplaceAllString(hostname, ".")

	return hostname
}

func AssociateClient(conn *ssh.ServerConn) (string, string, error) {
	lck.Lock()
	defer lck.Unlock()

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

	globalAutoComplete.AddMultiple(idString, username, conn.RemoteAddr().String(), conn.Permissions.Extensions["pubkey-fp"])
	if conn.Permissions.Extensions["comment"] != "" {
		globalAutoComplete.Add(conn.Permissions.Extensions["comment"])
	}

	_associateToOwners(idString, conn.Permissions.Extensions["owners"], conn)

	return idString, username, nil

}

func _associateToOwners(idString, owners string, conn *ssh.ServerConn) {
	username := NormaliseHostname(conn.User())
	ownersParts := strings.Split(owners, ",")

	if len(ownersParts) == 1 && ownersParts[0] == "" {
		// Owners is empty, so add it to the public list
		ownedByAll[idString] = conn

		PublicClientsAutoComplete.AddMultiple(idString, username, conn.RemoteAddr().String(), conn.Permissions.Extensions["pubkey-fp"])
		if conn.Permissions.Extensions["comment"] != "" {
			PublicClientsAutoComplete.Add(conn.Permissions.Extensions["comment"])
		}

	} else {
		for _, owner := range ownersParts {
			// Cant error if we're not adding a connection
			u, _, _ := _createOrGetUser(owner, nil)
			u.clients[idString] = conn

			u.autocomplete.AddMultiple(idString, username, conn.RemoteAddr().String(), conn.Permissions.Extensions["pubkey-fp"])
			if conn.Permissions.Extensions["comment"] != "" {
				u.autocomplete.Add(conn.Permissions.Extensions["comment"])
			}
		}
	}

}

func addAlias(uniqueId, newAlias string) {
	if _, ok := aliases[newAlias]; !ok {
		aliases[newAlias] = make(map[string]bool)
	}

	uniqueIdToAllAliases[uniqueId] = append(uniqueIdToAllAliases[uniqueId], newAlias)
	aliases[newAlias][uniqueId] = true
}

func DisassociateClient(uniqueId string, conn *ssh.ServerConn) {
	lck.Lock()
	defer lck.Unlock()

	if _, ok := allClients[uniqueId]; !ok {
		//If this is already removed then we dont need to remove it again.
		return
	}

	globalAutoComplete.Remove(uniqueId)
	currentAliases, ok := uniqueIdToAllAliases[uniqueId]
	if ok {
		// Remove the global references of the aliases and auto completes
		for _, alias := range currentAliases {
			if len(aliases[alias]) <= 1 {
				globalAutoComplete.Remove(alias)
				delete(aliases, alias)
			}

			delete(aliases[alias], uniqueId)
		}
	}

	_disassociateFromOwners(uniqueId, conn.Permissions.Extensions["owners"])

	delete(allClients, uniqueId)
	delete(uniqueIdToAllAliases, uniqueId)

}

func _disassociateFromOwners(uniqueId, owners string) {
	ownersParts := strings.Split(owners, ",")

	currentAliases := uniqueIdToAllAliases[uniqueId]

	if len(ownersParts) == 1 && ownersParts[0] == "" {
		delete(ownedByAll, uniqueId)

		PublicClientsAutoComplete.Remove(uniqueId)
		PublicClientsAutoComplete.RemoveMultiple(currentAliases...)

	} else {
		for _, owner := range ownersParts {

			u, err := _getUser(owner)
			if err != nil {
				continue
			}

			delete(u.clients, uniqueId)

			u.autocomplete.Remove(uniqueId)
			u.autocomplete.RemoveMultiple(currentAliases...)

			// If the owner has no clients after we do the delete, then remove the construct from memory
			if len(u.clients) == 0 {
				delete(users, owner)
			}
		}
	}
}
