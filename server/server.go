package server

import (
	"context"
	"errors"
	"goft/chat"
	"goft/components"
	"goft/postgres"
	sessionstore "goft/sessionStore"
	"goft/types"
	"goft/user"
	"goft/views"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"golang.org/x/crypto/bcrypt"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

var (
	ErrUnexpectedUser = errors.New("user data is not assigned")
)

type server struct {
	pg      postgres.Postgres
	room    *chat.Room
	session *sessionstore.Store
	http.Server
}

const (
	SERVER_READ_TIMEOUT     = 5
	SERVER_WRITE_TIMEOUT    = 10
	SERVER_SHUTDOWN_TIMEOUT = 1
)

func New(pg postgres.Postgres, room *chat.Room, session *sessionstore.Store) *server {
	r := chi.NewRouter()

	s := server{
		Server: http.Server{
			Handler:      r,
			Addr:         os.Getenv("HTTP_PORT"),
			ReadTimeout:  SERVER_READ_TIMEOUT * time.Second,
			WriteTimeout: SERVER_WRITE_TIMEOUT * time.Second,
		},
		pg:      pg,
		room:    room,
		session: session,
	}

	r.Use(middleware.Recoverer)
	r.Use(middleware.CleanPath)
	r.Use(middleware.StripSlashes)
	r.Use(middleware.Logger)

	s.routes(r)

	return &s
}

func (s *server) routes(r *chi.Mux) {
	r.Get("/", s.renderIndex)
	r.Get("/login", s.renderLogin)
	r.Post("/login", s.loginHandler)
	r.Get("/signup", s.renderSignup)
	r.Post("/signup", s.signupHandler)

	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)

		r.Get("/rooms", s.renderRooms)
		r.Get("/rooms/search", s.roomsSearchHandler)
		r.Get("/chat/{id}", s.renderChat)
		r.HandleFunc("/ws/{id}", s.chatroomHandler)
	})

	fs := http.FileServer(http.Dir("./static/"))
	r.Handle("/static/*", http.StripPrefix("/static", fs))
}

func (s *server) Start() chan error {
	errc := make(chan error, 1)

	l, err := net.Listen("tcp", os.Getenv("HTTP_PORT"))
	if err != nil {
		errc <- err
		return errc
	}

	log.Printf("Started listening on %s", os.Getenv("HTTP_PORT"))

	go func() {
		errc <- s.Serve(l)
	}()

	return errc
}

func (s *server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), SERVER_SHUTDOWN_TIMEOUT*time.Second)
	defer cancel()
	return s.Shutdown(ctx)
}

func (s *server) chatroomHandler(w http.ResponseWriter, r *http.Request) {
	roomID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.CloseNow()

	data, err := user.FromContext(r.Context())
	if err != nil {
		log.Println(ErrUnexpectedUser)
		return
	}

	err = s.room.AddClient(data, conn, r.Context(), roomID)
	if err != nil {
		if !errors.Is(err, chat.ErrDuplicatedSession) {
			log.Println(err)
			return
		}
	}
	defer s.room.RemoveClient(data.SessionID)

	type response struct {
		Message string `json:"message"`
		UserID  string `json:"user_id"`
		RoomID  string `json:"room_id"`
	}

	var res response

	for {
		err := wsjson.Read(r.Context(), conn, &res)
		if err != nil {
			if websocket.CloseStatus(err) != websocket.StatusNormalClosure &&
				websocket.CloseStatus(err) != websocket.StatusGoingAway {
				log.Println(err)
			}
			return
		}

		message, err := chat.NewMessage(res.Message, res.RoomID, res.UserID)
		if err != nil {
			log.Println(err)
			return
		}

		err = s.pg.CreateUserMessages(r.Context(), message)
		if err != nil {
			log.Println(err)
			return
		}

		err = s.room.MessageClients(message)
		if err != nil {
			log.Println(err)
			return
		}
	}
}

func getUserCookie(r *http.Request) (string, error) {
	cookie, err := r.Cookie("sessionID")
	if err != nil || cookie.Valid() != nil {
		return "", err
	}

	return cookie.Value, nil
}

func setUserCookie(w http.ResponseWriter, uuid string, expiry time.Time) {
	cookie := &http.Cookie{
		Name:     "sessionID",
		Value:    uuid,
		Expires:  expiry,
		Path:     "/",
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, cookie)
}

func (s *server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sessionID, err := getUserCookie(r)
		if err != nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		data, err := s.session.Get(r, sessionID)
		if err != nil {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		ctx := user.AddToContext(r.Context(), data)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// TODO:
// type AuthErrs struct {
// 	ErrUserNotExists bool
// 	ErrInvalidCred bool
// }

func (s *server) loginHandler(w http.ResponseWriter, r *http.Request) {
	name := r.PostFormValue("name")
	password := r.PostFormValue("password")

	user, err := user.New(name)
	if err != nil {
		log.Println(err)
		return
	}

	ID, err := s.pg.ValidateUser(r, user, password)
	if err != nil {
		data := map[string]bool{
			"ErrUserNotExists": errors.Is(err, postgres.ErrUserNotExists),
			"ErrInvalidCred":   errors.Is(err, bcrypt.ErrMismatchedHashAndPassword),
		}

		err = views.Login(data).Render(r.Context(), w)
		if err != nil {
			log.Println(err)
			return
		}

		return
	}

	user.ID = ID

	log.Printf("login user: id: %d, name: %s\n", user.ID, user.Name)

	s.session.Set(r, user.SessionID, user)
	log.Printf("after session.Set\n")
	setUserCookie(w, user.SessionID, user.Expiry)

	log.Printf("finished logging user\n")

	http.Redirect(w, r, "/rooms", http.StatusSeeOther)
}

func (s *server) roomsSearchHandler(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("search")
	var rooms []types.Room
	var err error

	if query != "" {
		rooms, err = s.pg.SearchRooms(r.Context(), query)
		if err != nil {
			log.Println(err)
			return
		}
	} else {
		rooms, err = s.pg.ListRoom(r.Context())
		if err != nil {
			log.Println(err)
			return
		}
	}

	// fmt.Printf("%+v\n", rooms)

	err = components.RoomsList(rooms).Render(r.Context(), w)
	if err != nil {
		log.Println(err)
		return
	}
}

func (s *server) signupHandler(w http.ResponseWriter, r *http.Request) {
	name := r.PostFormValue("name")
	password := r.PostFormValue("password")

	user, err := user.New(name)
	if err != nil {
		log.Println(err)
		return
	}

	err = s.pg.CreateUser(r, user, password)
	if err != nil {
		if strings.Contains(err.Error(), "unique constraint") {
			data := map[string]bool{
				"ErrDuplicatedUser": true,
			}

			err = views.Signup(data).Render(r.Context(), w)
			if err != nil {
				log.Println(err)
				return
			}
		}
		return
	}

	setUserCookie(w, user.SessionID, user.Expiry)

	http.Redirect(w, r, "/rooms", http.StatusSeeOther)
}

func (s *server) renderIndex(w http.ResponseWriter, r *http.Request) {
	sessionID, _ := getUserCookie(r)

	_, err := s.session.Get(r, sessionID)
	if err == nil {
		http.Redirect(w, r, "/rooms", http.StatusSeeOther)
		return
	}

	err = views.Index().Render(r.Context(), w)
	if err != nil {
		log.Println(err)
	}
}

func (s *server) renderRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := s.pg.ListRoom(r.Context())
	if err != nil {
		log.Println(err)
		return
	}

	err = views.Rooms(rooms).Render(r.Context(), w)
	if err != nil {
		log.Println(err)
	}
}

func (s *server) renderSignup(w http.ResponseWriter, r *http.Request) {
	err := views.Signup(nil).Render(r.Context(), w)
	if err != nil {
		log.Println(err)
	}
}

func (s *server) renderLogin(w http.ResponseWriter, r *http.Request) {
	err := views.Login(nil).Render(r.Context(), w)
	if err != nil {
		log.Println(err)
	}
}

func (s *server) renderChat(w http.ResponseWriter, r *http.Request) {
	roomID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	sessionID, err := getUserCookie(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	user, err := s.session.Get(r, sessionID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	messages, err := s.pg.GetRoomMessages(r.Context(), roomID)
	if err != nil {
		log.Println(err)
		return
	}

	room, err := s.pg.GetRoom(r.Context(), roomID)
	if err != nil {
		log.Println(err)
		return
	}

	err = views.Chat(user.ID, messages, roomID, room.Name).Render(r.Context(), w)
	if err != nil {
		log.Println(err)
		return
	}
}
