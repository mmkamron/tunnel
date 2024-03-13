package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func createSshConfig(username, keyFile string) *ssh.ClientConfig {
	knownHostsCallback, err := knownhosts.New(sshConfigPath("known_hosts"))
	if err != nil {
		log.Fatal(err)
	}

	key, err := os.ReadFile(keyFile)
	if err != nil {
		log.Fatalf("unable to read private key: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatalf("unable to parse private key: %v", err)
	}

	return &ssh.ClientConfig{
		User:              username,
		Auth:              []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback:   knownHostsCallback,
		HostKeyAlgorithms: []string{ssh.KeyAlgoED25519},
	}
}

func sshConfigPath(filename string) string {
	return filepath.Join(os.Getenv("HOME"), ".ssh", filename)
}

func main() {
	addr := flag.String("addr", "", "ssh server address to dial as <hostname>:<port>")
	username := flag.String("user", "", "username for ssh")
	keyFile := flag.String("keyfile", "", "file with private key for SSH authentication")
	remotePort := flag.String("rport", "", "remote port for tunnel")
	localPort := flag.String("lport", "", "local port for tunnel")
	flag.Parse()

	config := createSshConfig(*username, *keyFile)

	client, err := ssh.Dial("tcp", *addr, config)
	if err != nil {
		log.Fatal("failed to dial: ", err)
	}
	defer client.Close()

	listener, err := client.Listen("tcp", "localhost:"+*remotePort)
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()

	for {
		remote, err := listener.Accept()
		if err != nil {
			log.Fatal(err)
		}

		go func() {
			local, err := net.Dial("tcp", "localhost:"+*localPort)
			if err != nil {
				log.Fatal(err)
			}

			fmt.Println("tunnel established with", local.LocalAddr())
			runTunnel(local, remote)
		}()
	}
}

func runTunnel(local, remote net.Conn) {
	defer local.Close()
	defer remote.Close()
	done := make(chan struct{}, 2)

	go func() {
		io.Copy(local, remote)
		done <- struct{}{}
	}()

	go func() {
		io.Copy(remote, local)
		done <- struct{}{}
	}()

	<-done
}
