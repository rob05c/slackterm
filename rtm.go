package main

import (
	"encoding/json"
	"errors"
	//	"fmt" // debug
	"golang.org/x/net/websocket"
	"log"
	"net/http"
)

type SlackRtmUserInfo struct {
	Id   string `json:"id"`
	Name string `json:"name"`
	//	Prefs SlackRtmUserPrefs `json:"prefs"`
	Created        int    `json:"created"`
	ManualPresence string `json:"manual_presence"`
}

type SlackRtmTeamInfo struct {
	Id                string `json:"id"`
	Name              string `json:"name"`
	EmailDomain       string `json:"email_domain"`
	Domain            string `json:"domain"`
	MsgEditWindowMins int    `json:"msg_edit_window_mins"`
	OverStorageLimit  bool   `json:"over_storage_limit"`
	//	Prefs SlackRtmTeamPrefs `json:"prefs"`
	Plan string `json:"plan"`
}

type SlackIm struct {
	Id            string `json:"id"`
	IsIm          bool   `json:"is_im"`
	User          string `json:"user"`
	Created       int    `json:"created"`
	IsUserDeleted bool   `json:"is_user_deleted"`
}

type SlackRtmStart struct {
	Ok       bool             `json:"ok"`
	Url      string           `json:"url"`
	Self     SlackRtmUserInfo `json:"self"`
	Team     SlackRtmTeamInfo `json:"team"`
	Users    []SlackUser      `json:"users"`
	Channels []SlackChannel   `json:"channels"`
	Ims      []SlackIm        `json:"ims"`
	//	Bots []SlackBot `json:"bots"`
}

type SlackRtmType struct {
	Type string `json:"type"`
}

type SlackRtmMessage struct {
	Type      string `json:"type"` // TODO(Remove? Unnecessary?)
	ChannelId string `json:"channel"`
	UserId    string `json:"user"`
	Text      string `json:"text"`
	Time      string `json:"ts"`
}

func slackRtmStart(token string) (SlackRtmStart, error) {
	response, err := http.Get(`https://slack.com/api/rtm.start?token=` + token)
	if err != nil {
		return SlackRtmStart{}, err
	}

	if response.Status != `200 OK` {
		return SlackRtmStart{}, errors.New("Unexpected Response: " + response.Status)
	}

	var slackRtmStart SlackRtmStart
	if err = json.NewDecoder(response.Body).Decode(&slackRtmStart); err != nil {
		return SlackRtmStart{}, err
	}
	if !slackRtmStart.Ok {
		return SlackRtmStart{}, errors.New("Slack Rtm Start response not ok")
	}
	return slackRtmStart, nil
}

type SlackRtmHello struct {
	Type string `json:"type"`
}

type SlackRtmReplytoMsg struct {
	Ok      bool   `json:"ok"`
	ReplyTo *int   `json:"reply_to"`
	Time    string `json:"ts"`
	Text    string `json:"text"`
}

func ConnectToSlackRtm(token string) (*websocket.Conn, error) {
	startmsg, err := slackRtmStart(token)
	if err != nil {
		return nil, err
	}
	if !startmsg.Ok {
		return nil, errors.New("Slack Rtm Start Not Ok!")
	}

	origin := "http://localhost/"
	ws, err := websocket.Dial(startmsg.Url, "", origin)
	if err != nil {
		return nil, err
	}

	return ws, nil
}

// TODO(handle sent message ack [which requires storing the msg and id somewhere])
func handleSlackRtmMessage(type_ string, data []byte, putChan chan<- SlackRtmMessage, updateMsgsChan chan<- string, replyHandlerReceivedMsg chan<- SlackRtmReplytoMsg) {
	tryHandleReplyto := func() bool {
		var replyMsg SlackRtmReplytoMsg
		if err := json.Unmarshal(data, &replyMsg); err != nil {
			log.Panicln(err) // TODO(return error)
		}
		if replyMsg.ReplyTo == nil {
			return false
		}
		log.Printf("handleSlackRtmMessage tryHandleReplyto was reply, printing: %v\n", replyMsg)
		replyHandlerReceivedMsg <- replyMsg
		return true
	}

	// I want first class types!!1
	switch type_ {
	//	case `hello`:
	//		fmt.Printf("Received Hello: %s\n", string(data))
	case `message`:
		var msg SlackRtmMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Panicln(err) // TODO(return error)
		}
		PutMessage(msg, putChan)
		log.Println("handleSlackRtmMessage sending update msg " + string(data))
		updateMsgsChan <- msg.ChannelId
		log.Println("handleSlackRtmMessage sent update msg")
	default:
		if tryHandleReplyto() {
			return
		}
		log.Printf("Received Unhandled Type: %s\n", string(data))
	}
}

type PutRtmMsg struct {
	ChannelId string
	Msg       string
}

type SlackRtmSendMessage struct {
	Id        int    `json:"id"`
	Type      string `json:"type"`
	ChannelId string `json:"channel"`
	Text      string `json:"text"`
}

func SlackRtmSendHandler(ws *websocket.Conn, put <-chan PutRtmMsg, replyHandlerSentMsg chan<- SlackRtmSendMessage) {
	nextid := 0
	for {
		select {
		case p := <-put:
			// this could be sent off in a goroutine, for performance
			sendmsg := SlackRtmSendMessage{Id: nextid, Type: "message", ChannelId: p.ChannelId, Text: p.Msg}
			websocket.JSON.Send(ws, sendmsg)
			replyHandlerSentMsg <- sendmsg
			nextid++
		}
	}
}

func SlackRtmReceiveHandler(ws *websocket.Conn, putChan chan<- SlackRtmMessage, updateMsgsChan chan<- string, replyHandlerReceivedMsg chan<- SlackRtmReplytoMsg) {
	for {
		var data []byte
		err := websocket.Message.Receive(ws, &data)
		if err != nil {
			log.Panicln(err) // TODO(return error)
		}
		var msgType SlackRtmType
		if err = json.Unmarshal(data, &msgType); err != nil {
			log.Panicln(err) // TODO(return error)
		}
		handleSlackRtmMessage(msgType.Type, data, putChan, updateMsgsChan, replyHandlerReceivedMsg)
	}
}

func SlackRtmSentReplyHandler(sentMsg <-chan SlackRtmSendMessage, receivedReply <-chan SlackRtmReplytoMsg, putChan chan<- SlackRtmMessage, updateMsgsChan chan<- string) {
	sents := make(map[int]SlackRtmSendMessage)
	for {
		select {
		case s := <-sentMsg:
			if _, ok := sents[s.Id]; ok {
				log.Printf("SlackRtmSentReplyHandler got sentMsg id which already exists: %v\n", s) // TODO(panic?)
				continue
			}
			sents[s.Id] = s
		case r := <-receivedReply:
			if r.ReplyTo == nil {
				log.Printf("SlackRtmSentReplyHandler got nil reply: %v\n", r) // TODO(panic?)
				continue
			}
			if _, ok := sents[*r.ReplyTo]; !ok {
				log.Printf("SlackRtmSentReplyHandler got receivedReply id which does not exist: %v\n", r) // TODO(panic? Print anyway?)
				continue
			}
			sendmsg := sents[*r.ReplyTo]
			msg := SlackRtmMessage{Type: sendmsg.Type, ChannelId: sendmsg.ChannelId, UserId: "me", Text: sendmsg.Text, Time: r.Time}
			PutMessage(msg, putChan)
			updateMsgsChan <- msg.ChannelId
			delete(sents, *r.ReplyTo)
		}
	}
}

func SlackRtmHandler(token string, putChan chan<- SlackRtmMessage, updateMsgsChan chan<- string, sendMsgChan <-chan PutRtmMsg) {
	ws, err := ConnectToSlackRtm(token)
	if err != nil {
		log.Panicln(err)
	}

	replyHandlerSentMsg := make(chan SlackRtmSendMessage)
	replyHandlerReceivedReply := make(chan SlackRtmReplytoMsg)
	go SlackRtmSentReplyHandler(replyHandlerSentMsg, replyHandlerReceivedReply, putChan, updateMsgsChan)
	go SlackRtmSendHandler(ws, sendMsgChan, replyHandlerSentMsg)
	SlackRtmReceiveHandler(ws, putChan, updateMsgsChan, replyHandlerReceivedReply) // don't go, so this function doesn't return.
}

// StartSlackRtmHandler starts the slack RTM handler goroutine, and returns a channel to
// which will be written the channel id of channels which recieve new messages.
// Also returns a chan to which will be written messages to send from user input.
func StartSlackRtmHandler(token string, putChan chan<- SlackRtmMessage) (<-chan string, chan<- PutRtmMsg) {
	updateMsgsChan := make(chan string)
	sendMsgChan := make(chan PutRtmMsg)
	go SlackRtmHandler(token, putChan, updateMsgsChan, sendMsgChan)
	return updateMsgsChan, sendMsgChan
}
