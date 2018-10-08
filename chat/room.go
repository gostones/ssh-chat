package chat

import (
	"errors"
	"fmt"
	"io"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/gostones/ssh-chat/chat/message"
	"github.com/gostones/ssh-chat/set"
)

const historyLen = 1
const roomBuffer = 10

// The error returned when a message is sent to a room that is already
// closed.
var ErrRoomClosed = errors.New("room closed")

// The error returned when a user attempts to join with an invalid name, such
// as empty string.
var ErrInvalidName = errors.New("invalid name")

// Member is a User with per-Room metadata attached to it.
type Member struct {
	*message.User
}

// type RPCLink struct {
// 	Name     string // service name
// 	HostPort string
// 	Port     int //rps port
// 	From     *message.User
// 	To       *message.User
// }

// Room definition, also a Set of User Items
type Room struct {
	topic     string
	history   *message.History
	broadcast chan message.Message
	commands  Commands
	closed    bool
	closeOnce sync.Once

	Members *set.Set
	Ops     *set.Set

	//Links *set.Set
}

// NewRoom creates a new room.
func NewRoom() *Room {
	broadcast := make(chan message.Message, roomBuffer)

	return &Room{
		broadcast: broadcast,
		history:   message.NewHistory(historyLen),
		commands:  *defaultCommands,

		Members: set.New(),
		Ops:     set.New(),
		//Links:   set.New(),
	}
}

// SetCommands sets the room's command handlers.
func (r *Room) SetCommands(commands Commands) {
	r.commands = commands
}

// Close the room and all the users it contains.
func (r *Room) Close() {
	r.closeOnce.Do(func() {
		r.closed = true
		r.Members.Each(func(_ string, item set.Item) error {
			item.Value().(*Member).Close()
			return nil
		})
		r.Members.Clear()
		close(r.broadcast)
	})
}

// SetLogging sets logging output for the room's history
func (r *Room) SetLogging(out io.Writer) {
	r.history.SetOutput(out)
}

// HandleMsg reacts to a message, will block until done.
func (r *Room) HandleMsg(m message.Message) {
	switch m := m.(type) {
	case *message.CommandMsg:
		cmd := *m
		err := r.commands.Run(r, cmd)
		if err != nil {
			m := message.NewSystemMsg(fmt.Sprintf(`"err: %s"`, err), cmd.From())
			go r.HandleMsg(m)
		}
	case message.MessageTo:
		user := m.To()
		user.Send(m)
	default:
		fromMsg, skip := m.(message.MessageFrom)
		var skipUser *message.User
		if skip {
			skipUser = fromMsg.From()
		}

		//r.history.Add(m)
		r.Members.Each(func(_ string, item set.Item) (err error) {
			user := item.Value().(*Member).User

			if fromMsg != nil && user.Ignored.In(fromMsg.From().ID()) {
				// Skip because ignored
				return
			}

			if skip && skipUser == user {
				// Skip self
				return
			}
			if _, ok := m.(*message.AnnounceMsg); ok {
				if user.Config().Quiet {
					// Skip announcements
					return
				}
			}
			user.Send(m)
			return
		})
	}
}

// Serve will consume the broadcast room and handle the messages, should be
// run in a goroutine.
func (r *Room) Serve() {
	for m := range r.broadcast {
		go r.HandleMsg(m)
	}
}

// Send message, buffered by a chan.
func (r *Room) Send(m message.Message) {
	r.broadcast <- m
}

// History feeds the room's recent message history to the user's handler.
func (r *Room) History(u *message.User) {
	for _, m := range r.history.Get(historyLen) {
		u.Send(m)
	}
}

// Join the room as a user, will announce.
func (r *Room) Join(u *message.User) (*Member, error) {
	// TODO: Check if closed
	if u.ID() == "" {
		return nil, ErrInvalidName
	}
	member := &Member{u}
	err := r.Members.Add(set.Itemize(u.ID(), member))
	if err != nil {
		return nil, err
	}
	r.History(u)
	s := fmt.Sprintf(`{"type": "presence", "msg":{ "who": "%s", "status": "joined", "connected": "%d"}}`, u.Name(), r.Members.Len())
	r.Send(message.NewPresenceMsg(s))
	return member, nil
}

//
// func (r *Room) sendMessage(target *message.User, content string, from *message.User) error {

// 	m := message.NewPrivateMsg(content, from, target)
// 	r.Send(&m)

// 	txt := fmt.Sprintf("[Sent PM to %s]", target.Name())
// 	ms := message.NewSystemMsg(txt, from)
// 	r.Send(ms)
// 	target.SetReplyTo(from)

// 	return nil
// }

// //
// func (r *Room) CreateRPC(name string, hostPort string, port int, from *message.User) error {
// 	member, ok := r.MemberByID(name)
// 	if !ok {
// 		return errors.New("Not found: " + name)
// 	}
// 	to := member.User

// 	if err := r.sendMessage(to, fmt.Sprintf(`{"cmd":"rpc", "host_port":"%v", "remote_port":"%v"}`, hostPort, port), from); err != nil {
// 		return err
// 	}

// 	key := fmt.Sprintf("%v:%v", to.ID(), port)
// 	link := RPCLink{
// 		Name:     name,
// 		Port:     port,
// 		HostPort: hostPort,
// 		To:       to,
// 		From:     from,
// 	}
// 	r.Links.Add(set.Itemize(key, link))
// 	return nil
// }

// //
// func (r *Room) BackoffCreateRPC(name string, hostPort string, port int, from *message.User) {
// 	sleep := BackoffDuration()
// 	for {
// 		rc := r.CreateRPC(name, hostPort, port, from)
// 		if rc == nil {
// 			return
// 		}
// 		sleep(rc)
// 	}
// }

// //
// func (r *Room) CreateTun(name string, hostPort string, port int, from *message.User) error {
// 	member, ok := r.MemberByID(name)
// 	if !ok {
// 		return errors.New("Not found: " + name)
// 	}
// 	to := member.User

// 	return r.sendMessage(to, fmt.Sprintf(`{"cmd":"tun", "host_port":"%v", "remote_port":"%v"}`, hostPort, port), from)
// }

// //
// func (r *Room) recreateLink(u message.Identifier) error {
// 	items := r.Links.ListPrefix(u.ID() + ":")
// 	for _, item := range items {
// 		l := item.Value().(*RPCLink)
// 		logger.Printf("Recreating rpc: %v\n", l)
// 		go r.BackoffCreateRPC(l.Name, l.HostPort, l.Port, l.From)
// 	}

// 	return nil
// }

// Leave the room as a user, will announce. Mostly used during setup.
func (r *Room) Leave(u message.Identifier) error {
	err := r.Members.Remove(u.ID())
	if err != nil {
		return err
	}
	r.Ops.Remove(u.ID())
	s := fmt.Sprintf(`{"type": "presence", "msg": {"who": "%s", "status": "left"}}`, u.Name())
	r.Send(message.NewPresenceMsg(s))

	//check and recreate rpc service
	//go r.recreateLink(u)
	//

	return nil
}

// Update member
func (r *Room) UpdateService(u *message.User) error {
	member := &Member{u}
	err := r.Members.Replace(u.ID(), set.Itemize(u.ID(), member))

	return err
}

// Rename member with a new identity. This will not call rename on the member.
func (r *Room) Rename(oldID string, u message.Identifier) error {
	if u.ID() == "" {
		return ErrInvalidName
	}
	err := r.Members.Replace(oldID, set.Itemize(u.ID(), u))
	if err != nil {
		return err
	}

	s := fmt.Sprintf(`{"type": "nick", "msg": {"who": "%s", "alias": "%s"}}`, oldID, u.ID())
	r.Send(message.NewMsg(s))
	return nil
}

// Member returns a corresponding Member object to a User if the Member is
// present in this room.
func (r *Room) Member(u *message.User) (*Member, bool) {
	m, ok := r.MemberByID(u.ID())
	if !ok {
		return nil, false
	}
	// Check that it's the same user
	if m.User != u {
		return nil, false
	}
	return m, true
}

// MemberByID returns the member by ID or a random member by prefix
func (r *Room) MemberByID(id string) (*Member, bool) {
	//prefix name/*
	ids := strings.Split(id, "/")
	if len(ids) == 1 || ids[1] == "" || ids[1] == "*" {
		items := r.Members.ListPrefix(ids[0] + "/")
		if len(items) > 0 {
			rand.Seed(time.Now().UnixNano())
			m := items[rand.Intn(len(items))]
			return m.Value().(*Member), true
		}
	}
	m, err := r.Members.Get(id)
	if err != nil {
		return nil, false
	}
	return m.Value().(*Member), true
}

// IsOp returns whether a user is an operator in this room.
func (r *Room) IsOp(u *message.User) bool {
	return r.Ops.In(u.ID())
}

// Topic of the room.
func (r *Room) Topic() string {
	return r.topic
}

// SetTopic will set the topic of the room.
func (r *Room) SetTopic(s string) {
	r.topic = s
}

// NamesPrefix lists all members' names with a given prefix, used to query
// for autocompletion purposes.
func (r *Room) NamesPrefix(prefix string) []string {
	items := r.Members.ListPrefix(prefix)
	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.Value().(*Member).User.Name()
	}
	return names
}
