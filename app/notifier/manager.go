package notifier

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/pkg/errors"

	"backend/app/models"
	"backend/pkg/log"
	"backend/pkg/uuid"
)

const (
	// time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// time allowed to read the next pong message from the peer
	pongWait = 30 * time.Second

	// send pings to peer with this period, must be less than pongWait
	pingPeriod = (pongWait * 8) / 10
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
)

type unsubscribeHandler func(*subscription)

type subscription struct {
	id            string
	clientID      string
	conn          *websocket.Conn
	send          chan interface{}
	onUnsubscribe unsubscribeHandler
}

func (s *subscription) read() {
	defer func() {
		if s.onUnsubscribe != nil {
			s.onUnsubscribe(s)
		}
		_ = s.conn.Close()
	}()

	_ = s.conn.SetReadDeadline(time.Now().Add(pongWait))
	s.conn.SetPongHandler(func(string) error {
		_ = s.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, _, err := s.conn.ReadMessage()
		if err != nil { // failed to read pong or other message
			break
		}
	}
}

func (s *subscription) write() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = s.conn.Close()
	}()

	for {
		select {
		case message, ok := <-s.send:
			_ = s.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok { // the channel was closed by notifier
				_ = s.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := s.conn.WriteJSON(message); err != nil {
				return
			}
		case <-ticker.C:
			_ = s.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := s.conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}

type subscriptions map[string]*subscription

type Manager struct {
	clients       map[string]subscriptions
	notifications chan *models.Notification
	register      chan *subscription
	unregister    chan *subscription
}

func NewManager() *Manager {
	return &Manager{
		clients:       make(map[string]subscriptions),
		notifications: make(chan *models.Notification),
		register:      make(chan *subscription),
		unregister:    make(chan *subscription),
	}
}

func (m *Manager) Subscribe(ctx context.Context, sub *models.NewSubscription) error {
	sub.ClientID = strings.ToLower(sub.ClientID)
	log.AddFields(ctx, "subscriber", sub.ClientID)

	conn, err := upgrader.Upgrade(sub.ResponseWriter, sub.Request, nil)
	if err != nil {
		return errors.Wrap(err, "failed to upgrade a connection")
	}

	m.register <- &subscription{
		id:       uuid.NewUUID(),
		clientID: sub.ClientID,
		conn:     conn,
		send:     make(chan interface{}),
		onUnsubscribe: func(s *subscription) {
			m.unregister <- s
		},
	}
	return nil
}

func (m *Manager) Notify(ctx context.Context, notification *models.Notification) {
	notification.ClientID = strings.ToLower(notification.ClientID)
	log.Infow("notify by ws", "notification", notification)
	m.notifications <- notification
}

func (m *Manager) Start() {
	log.Info("starting notifier service")
	for {
		select {
		case sub := <-m.register:
			subs, ok := m.clients[sub.clientID]
			if !ok {
				subs = make(subscriptions)
				m.clients[sub.clientID] = subs
			}
			subs[sub.id] = sub
			go sub.read()
			go sub.write()
		case sub := <-m.unregister:
			if subs, ok := m.clients[sub.clientID]; ok {
				if _, ok := subs[sub.id]; ok {
					delete(subs, sub.id)
					close(sub.send)
				}
			}
		case notification := <-m.notifications:
			subs, ok := m.clients[notification.ClientID]
			if !ok {
				break
			}

			for _, s := range subs {
				s.send <- notification.Message
			}
		}
	}
}
