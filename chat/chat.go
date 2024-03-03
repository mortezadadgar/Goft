package chat

import (
	"context"
	"errors"
	"fmt"
	"goft/components"
	"goft/user"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"nhooyr.io/websocket"
)

var (
	ErrMessageEmpty      = errors.New("message cannot be empty")
	ErrDuplicatedSession = errors.New("duplicated session")
)

type client struct {
	user   user.User
	roomID int
	conn   *websocket.Conn
	ctx    context.Context
}

type Room struct {
	clients   map[string]client
	muClients sync.RWMutex
}

type Message struct {
	Text   string
	UserID int
	RoomID int
	time   time.Time
}

func NewMessage(text string, roomID string, userID string) (Message, error) {
	content := strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if text == "" {
		return Message{}, ErrMessageEmpty
	}

	parsedRoomID, err := strconv.Atoi(roomID)
	if err != nil {
		return Message{}, fmt.Errorf("failed to parse roomID, %v", err)
	}

	parsedUserID, err := strconv.Atoi(userID)
	if err != nil {
		return Message{}, fmt.Errorf("failed to parse userID, %v", err)
	}

	return Message{
		Text:   content,
		UserID: parsedUserID,
		RoomID: parsedRoomID,
		time:   time.Now().UTC(),
	}, nil

}

func New() *Room {
	return &Room{
		clients: make(map[string]client),
	}
}

func (r *Room) AddClient(user user.User, conn *websocket.Conn, ctx context.Context, roomID int) error {
	_, found := r.GetClient(user.SessionID)
	if found {
		return ErrDuplicatedSession
	}

	r.muClients.Lock()
	r.clients[user.SessionID] = client{
		user:   user,
		conn:   conn,
		ctx:    ctx,
		roomID: roomID,
	}
	r.muClients.Unlock()

	return nil
}

func (r *Room) GetClient(ID string) (client, bool) {
	r.muClients.RLock()
	defer r.muClients.RUnlock()
	client, found := r.clients[ID]
	return client, found
}

func (r *Room) RemoveClient(ID string) {
	r.muClients.Lock()
	defer r.muClients.Unlock()
	delete(r.clients, ID)
}

func (r *Room) MessageClients(message Message) error {
	r.muClients.RLock()
	defer r.muClients.RUnlock()

	var wg sync.WaitGroup
	errChan := make(chan error)
	done := make(chan struct{})

	for _, c := range r.clients {
		if c.roomID != message.RoomID {
			continue
		}

		wg.Add(1)
		go func(c client) {
			defer wg.Done()
			w, err := c.conn.Writer(c.ctx, websocket.MessageText)
			if err != nil {
				errChan <- err
			}

			err = components.Message(message.Text).Render(context.Background(), w)
			if err != nil {
				errChan <- err
			}

			if err := w.Close(); err != nil {
				errChan <- err
			}

			log.Printf("send message: %s to user: %+v\n", message.Text, c.user)
		}(c)
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case err := <-errChan:
		return err
	case <-done:
		return nil
	}
}
