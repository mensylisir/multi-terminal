package ssh

import (
	"fmt"
	"io"
	"time"

	"golang.org/x/crypto/ssh"
)

type Client struct {
	Config *ssh.ClientConfig
	Client *ssh.Client
}

func NewClientConfig(user string, authMethods ...ssh.AuthMethod) *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         30 * time.Second,
	}
}

func (c *Client) Connect(addr string) error {
	client, err := ssh.Dial("tcp", addr, c.Config)
	if err != nil {
		return fmt.Errorf("ssh dial failed: %w", err)
	}
	c.Client = client
	return nil
}

func (c *Client) NewSession() (*ssh.Session, error) {
	if c.Client == nil {
		return nil, fmt.Errorf("client not connected")
	}
	return c.Client.NewSession()
}

func (c *Client) OpenPTY(cols, rows int) (io.ReadWriteCloser, error) {
	session, err := c.NewSession()
	if err != nil {
		return nil, err
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	err = session.RequestPty("xterm-256color", cols, rows, modes)
	if err != nil {
		session.Close()
		return nil, err
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		session.Close()
		return nil, err
	}

	stdout, err := session.StdoutPipe()
	if err != nil {
		session.Close()
		return nil, err
	}

	stderr, err := session.StderrPipe()
	if err != nil {
		session.Close()
		return nil, err
	}

	if err := session.Shell(); err != nil {
		session.Close()
		return nil, err
	}

	return &ptySession{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		session: session,
	}, nil
}

type ptySession struct {
	stdin   io.WriteCloser
	stdout  io.Reader
	stderr  io.Reader
	session *ssh.Session
}

func (p *ptySession) Read(b []byte) (int, error) {
	return p.stdout.Read(b)
}

func (p *ptySession) Write(b []byte) (int, error) {
	return p.stdin.Write(b)
}

func (p *ptySession) Close() error {
	p.stdin.Close()
	return p.session.Close()
}

// SetReadDeadline sets the read deadline - no-op since ssh.Session doesn't support it
func (p *ptySession) SetReadDeadline(t time.Time) error {
	return nil
}

// SetWriteDeadline sets the write deadline - no-op since ssh.Session doesn't support it
func (p *ptySession) SetWriteDeadline(t time.Time) error {
	return nil
}
