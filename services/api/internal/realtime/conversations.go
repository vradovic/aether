package realtime

import "github.com/gorilla/websocket"

type MessageRouter struct {
	participants map[string][]Participant // conversation id to participants
	register     chan Participant
}

type ParticipantRegistrationRequest struct {
	participant   Participant
	conversations []string
}

type Participant struct {
	conn *websocket.Conn
	send chan string
}

func (c Conversation) run() {
	for {
		select {
		case msg := <-c.in:

		}
	}
}

/*
3 websocket message types:
1. message -> contains the sender and conversation id
2. create_conversation -> contains the sender and contacts (sender and provided users must be contacts)
3. leave_conversation ->  contains sender and conversation id
*/
