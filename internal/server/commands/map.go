package commands

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/NHAS/reverse_ssh/internal/server/users"
	"github.com/NHAS/reverse_ssh/internal/terminal"
	"github.com/fatih/color"
	"golang.org/x/crypto/ssh"
)

type mapCommand struct{}

func (m *mapCommand) ValidArgs() map[string]string {
	return map[string]string{
		"h": "Print help",
	}
}

type mapNode struct {
	id       string
	conn     *ssh.ServerConn
	children []string
}

func (m *mapCommand) Run(user *users.User, tty io.ReadWriter, line terminal.ParsedLine) error {
	allClients, err := user.SearchClients("")
	if err != nil {
		return err
	}

	if len(allClients) == 0 {
		return fmt.Errorf("No RSSH clients connected")
	}

	// Build node map and collect parent relationships.
	nodes := make(map[string]*mapNode, len(allClients))
	for id, conn := range allClients {
		nodes[id] = &mapNode{id: id, conn: conn}
	}

	// childOf maps child-id -> parent-id (only when parent is itself connected).
	childOf := make(map[string]string)
	for id, conn := range allClients {
		parent := conn.Permissions.Extensions["pivot-parent"]
		if parent == "" {
			continue
		}
		if _, parentConnected := nodes[parent]; parentConnected {
			childOf[id] = parent
			nodes[parent].children = append(nodes[parent].children, id)
		}
	}

	// Sort children lists for stable output.
	for _, n := range nodes {
		sort.Strings(n.children)
	}

	// Find roots: clients with no connected parent (direct or orphaned pivot).
	roots := []string{}
	for id := range nodes {
		if _, hasParent := childOf[id]; !hasParent {
			roots = append(roots, id)
		}
	}
	sort.Strings(roots)

	fmt.Fprintln(tty, color.HiWhiteString("RSSH"))

	for i, rootID := range roots {
		isLastRoot := i == len(roots)-1
		printNode(tty, nodes, rootID, "", isLastRoot)
	}

	return nil
}

func printNode(tty io.ReadWriter, nodes map[string]*mapNode, id, prefix string, isLast bool) {
	n := nodes[id]

	branch := "├── "
	childPrefix := prefix + "│   "
	if isLast {
		branch = "└── "
		childPrefix = prefix + "    "
	}

	label := formatLabel(n.conn, id)
	fmt.Fprintf(tty, "%s%s%s\n", prefix, branch, label)

	for i, childID := range n.children {
		isLastChild := i == len(n.children)-1
		printNode(tty, nodes, childID, childPrefix, isLastChild)
	}
}

func formatLabel(conn *ssh.ServerConn, id string) string {
	keyID := conn.Permissions.Extensions["pubkey-fp"]
	if conn.Permissions.Extensions["comment"] != "" {
		keyID = conn.Permissions.Extensions["comment"]
	}

	hostname := users.NormaliseHostname(conn.User())
	addr := conn.RemoteAddr().String()
	version := string(conn.ClientVersion())

	// Trim SSH version prefix for brevity ("SSH-_guess-windows_amd64" -> "windows_amd64")
	version = strings.TrimPrefix(version, "SSH-_guess-")
	version = strings.TrimPrefix(version, "SSH-2.0-")

	pivot := ""
	if conn.Permissions.Extensions["pivot-parent"] != "" {
		pivot = color.YellowString(" [pivoted]")
	}

	shortKeyID := keyID
	if len(keyID) > 4 {
		shortKeyID = keyID[:4]
	}

	return fmt.Sprintf("%s %s %s (%s) [%s]%s",
		color.YellowString(id),
		color.CyanString(shortKeyID),
		color.BlueString(hostname),
		addr,
		version,
		pivot,
	)
}

func (m *mapCommand) Expect(line terminal.ParsedLine) []string {
	return nil
}

func (m *mapCommand) Help(explain bool) string {
	const description = "Display a topology map of connected clients and their pivot chains"
	if explain {
		return description
	}

	return terminal.MakeHelpText(m.ValidArgs(),
		"map",
		description,
	)
}
