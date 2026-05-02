package api

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

type ChatMonitor struct {
	Channel     string
	conn        net.Conn
	mu          sync.Mutex
	messages    []string
	maxMessages int
	running     bool
}

func NewChatMonitor(channel string) *ChatMonitor {
	return &ChatMonitor{
		Channel:     strings.ToLower(channel),
		messages:    make([]string, 0),
		maxMessages: 5, // Mantém apenas as últimas 5 mensagens
	}
}

func (c *ChatMonitor) Start() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.mu.Unlock()

	go c.loop()
}

func (c *ChatMonitor) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.running = false
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *ChatMonitor) updateOverlay() {
	content := strings.Join(c.messages, "\n")
	os.WriteFile("chat_overlay.txt", []byte(content), 0644)
}

func (c *ChatMonitor) loop() {
	for {
		c.mu.Lock()
		if !c.running {
			c.mu.Unlock()
			return
		}
		c.mu.Unlock()

		conn, err := net.DialTimeout("tcp", "irc.chat.twitch.tv:6667", 5*time.Second)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		c.mu.Lock()
		c.conn = conn
		c.mu.Unlock()

		// Login anônimo da Twitch
		fmt.Fprintf(conn, "PASS SCHMOOPIIE\r\nNICK justinfan9999\r\nJOIN #%s\r\n", c.Channel)

		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := scanner.Text()

			// Responde ao PING para manter a conexão viva
			if strings.HasPrefix(line, "PING") {
				fmt.Fprintf(conn, "PONG :tmi.twitch.tv\r\n")
				continue
			}

			// Parse PRIVMSG
			// Ex: :mrjcrom!mrjcrom@mrjcrom.tmi.twitch.tv PRIVMSG #mrjcrom :Olá chat!
			if strings.Contains(line, " PRIVMSG #") {
				parts := strings.SplitN(line, " :", 2)
				if len(parts) == 2 {
					header := parts[0]
					msg := parts[1]

					userParts := strings.SplitN(header, "!", 2)
					user := strings.TrimPrefix(userParts[0], ":")

					formatted := fmt.Sprintf("%s: %s", user, msg)

					c.mu.Lock()
					c.messages = append(c.messages, formatted)
					if len(c.messages) > c.maxMessages {
						c.messages = c.messages[1:]
					}
					c.updateOverlay()
					c.mu.Unlock()
				}
			}
		}

		// Se o loop interno quebrar (desconexão), tenta conectar de novo
		time.Sleep(2 * time.Second)
	}
}
