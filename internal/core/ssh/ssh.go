package ssh

import (
	"fmt"
	"os"
	"strings"

	"github.com/scarlass/tera-askep/internal/core/configs"
	"golang.org/x/crypto/ssh"
)

type SSHClient struct {
	host string
	port int

	conf   *ssh.ClientConfig
	client *ssh.Client
}

func New(conf configs.SSHConfig) (*SSHClient, error) {
	client := &SSHClient{
		host: conf.Host,
		port: conf.Port,
		conf: &ssh.ClientConfig{
			User: conf.User,
			Auth: []ssh.AuthMethod{
				ssh.Password(conf.Password),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		},
	}

	return client, client.open()
}

func (sc *SSHClient) open() error {
	dial, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", sc.host, sc.port), sc.conf)
	if err != nil {
		return fmt.Errorf("when connecting to ssh: %w", err)
	}

	sc.client = dial
	return nil
}
func (sc *SSHClient) session() (*ssh.Session, error) {
	ss, err := sc.client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("create session: %w", err)
	}
	return ss, nil
}

func (sc *SSHClient) Close() error {
	if sc != nil {
		return sc.client.Close()
	}
	return nil
}

func (sc *SSHClient) Exec(cmd string, args ...string) (err error) {
	var ss *ssh.Session
	if ss, err = sc.session(); err != nil {
		return err
	}

	defer ss.Close()
	ss.Stderr = os.Stderr
	ss.Stdout = os.Stdout

	return ss.Run(fmt.Sprintf("%s %s", cmd, strings.Join(args, " ")))
}
