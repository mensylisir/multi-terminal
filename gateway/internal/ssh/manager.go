package ssh

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

type Manager struct {
	clients sync.Map
}

func NewManager() *Manager {
	m := &Manager{}
	return m
}

func (m *Manager) keepaliveLoop() {
	ticker := time.NewTicker(15 * time.Second)
	for range ticker.C {
		m.clients.Range(func(key, value any) bool {
			client := value.(*Client)
			if client.Client != nil {
				client.Client.SendRequest("keepalive@gateway.com", true, nil)
			}
			return true
		})
	}
}

func (m *Manager) GetOrCreate(host string, config *ssh.ClientConfig) (*Client, error) {
	v, ok := m.clients.Load(host)
	if ok {
		return v.(*Client), nil
	}
	client := &Client{Config: config}
	if err := client.Connect(host); err != nil {
		return nil, err
	}
	m.clients.Store(host, client)
	return client, nil
}

func (m *Manager) Close(host string) error {
	v, ok := m.clients.Load(host)
	if !ok {
		return fmt.Errorf("client not found")
	}
	client := v.(*Client)
	if client.Client != nil {
		client.Client.Close()
	}
	m.clients.Delete(host)
	return nil
}
