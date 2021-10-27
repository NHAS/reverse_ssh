package internal

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/crypto/ssh"
)

func constructRoutes() error {

	for i := 0; i < 10; i++ {

		var s ssh.Conn = ssh.ServerConn{}
		userId := fmt.Sprintf("user%d", i)
		u, err := AddUser(userId, s)
		if err != nil {
			return err
		}

		for ii := 0; ii < 10; ii++ {

			l := RemoteForwardRequest{"localhost", uint32(ii)}
			u.SupportedRemoteForwards[l] = true
		}

		if i%2 == 0 {
			err = EnableForwarding(userId, fmt.Sprintf("%d", i))
			if err != nil {
				return err
			}

		}

	}

	return nil
}

func TestMain(m *testing.M) {
	// Write code here to func TestMain(m *testing.M) {
	// Write code here to run before tests
	constructRoutes()
	// Run tests
	exitVal := m.Run()

	// Write code here to run after tests

	// Exit with exit value from tests
	os.Exit(exitVal)
}

func TestRouteCollision(t *testing.T) {
	//Basic test, add a user that already has a routing rule in the table
	err := EnableForwarding("user0", "0")
	if err == nil {
		t.Fatalf("This should error as there is a collision")
	}

	u, err := AddUser("testCollisionUser", ssh.ServerConn{})
	if err != nil {
		t.Fatal(err)
	}

	u.SupportedRemoteForwards[RemoteForwardRequest{"localhost", 1}] = true

	err = EnableForwarding("testCollisionUser", "0")
	if err == nil {
		t.Fatalf("Enabling forwarding for new user that collides with another users forwards should fail")
	}

	err = EnableForwarding("testCollisionUser", "1")
	if err != nil {
		t.Fatalf("Enabling forwarding should work as table 1 has no entries")
	}

	err = EnableForwarding("testCollisionUser", "1")
	if err == nil {
		t.Fatalf("Enabling user twice should fail")
	}

}

func TestGetDestination(t *testing.T) {
	s, err := GetDestination("0", RemoteForwardRequest{"localhost", 0})
	if err != nil {
		t.Fatal(err)
	}

	if s == nil {
		t.Fatal("The resulting dest should not be nil")
	}

	_, err = GetDestination("0", RemoteForwardRequest{"localhost", 2134})
	if err == nil {
		t.Fatal("Should fail, as no forwarding rule localhost:2134 for target client 0")
	}

	_, err = GetDestination("1", RemoteForwardRequest{"localhost", 0})
	if err == nil {
		t.Fatal("Should fail, as no taget 1 and not default route")
	}

}

func TestUserLeft(t *testing.T) {
	conn := ssh.ServerConn{}
	u, err := AddUser("testRemoveUser", conn)
	if err != nil {
		t.Fatal(err)
	}

	rf := RemoteForwardRequest{"localhost", 1222}

	u.SupportedRemoteForwards[rf] = true

	err = EnableForwarding(u.IdString, "3")
	if err != nil {
		t.Fatal(err)
	}

	m, err := GetDestination("3", rf)
	if err != nil {
		t.Fatal(err)
	}

	if m != conn {
		t.Fatal("input ssh conn != output dest")
	}

	RemoveUser(u.IdString)

	_, err = GetDestination("3", rf)
	if err == nil {
		t.Fatal("Should fail as user is no longer avaiable as destination")
	}

}

func TestTargetLeft(t *testing.T) {
	_, err := GetDestination("0", RemoteForwardRequest{"localhost", 0})
	if err != nil {
		t.Fatal(err)
	}

	RemoveSource("0")

	_, err = GetDestination("0", RemoteForwardRequest{"localhost", 0})
	if err == nil {
		t.Fatal("Source should be removed")
	}
}

func TestRemoveForward(t *testing.T) {
	conn := ssh.ServerConn{}
	u, err := AddUser("testRemoveForward", conn)
	if err != nil {
		t.Fatal(err)
	}

	u.SupportedRemoteForwards[RemoteForwardRequest{"localhost", 1222}] = true
	u.SupportedRemoteForwards[RemoteForwardRequest{"localhost", 1223}] = true

	err = EnableForwarding(u.IdString, "3")
	if err != nil {
		t.Fatal(err)
	}

	err = EnableForwarding(u.IdString, "5")
	if err != nil {
		t.Fatal(err)
	}

	tc := RemoveFoward(RemoteForwardRequest{"localhost", 1222}, u)
	if len(tc) != 2 {
		t.Fatalf("Should have closed forward on 5 and 3, but tc length was %d", len(tc))
	}
}
