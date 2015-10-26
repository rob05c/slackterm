package main

import (
	//	"fmt"
	"log"
)

type TermMsg struct {
	UserName string
	Text     string
}

type MessageRequest struct {
	ChannelId string
	Reply     chan<- []TermMsg
}

func GetMessages(channelId string, getChan chan<- MessageRequest) []TermMsg {
	replyChan := make(chan []TermMsg)
	getChan <- MessageRequest{channelId, replyChan}
	return <-replyChan
}

// TODO(Remove and write to chan directly?
func PutMessage(msg SlackRtmMessage, putChan chan<- SlackRtmMessage) {
	putChan <- msg
}

// TODO(generic map pattern?)
func slackMessagesToTermMsgs(msgs []SlackMessage, getUserNameChan chan<- UserNameRequest) []TermMsg {
	var newmsgs []TermMsg
	for _, msg := range msgs {
		newmsgs = append(newmsgs, TermMsg{UserName: GetUserName(msg.User, getUserNameChan), Text: msg.Text})
	}
	return newmsgs
}

func messagesManager(token string, get <-chan MessageRequest, put <-chan SlackRtmMessage, getUserNameChan chan<- UserNameRequest) {
	messages := make(map[string][]TermMsg)
	for {
		select {
		case g := <-get:
			log.Println("messageManager get " + g.ChannelId)
			if _, ok := messages[g.ChannelId]; !ok {
				log.Println("messageManager get getting0 " + g.ChannelId)
				msgs, err := GetAllSlackMessages(token, g.ChannelId)
				log.Println("messageManager get got0 " + g.ChannelId)
				if err != nil {
					log.Panicln(err) // TODO(return error)
				}
				log.Println("messageManager creating term msgs0 " + g.ChannelId)
				messages[g.ChannelId] = slackMessagesToTermMsgs(msgs, getUserNameChan)
			}
			log.Printf("messageManager get len %d\n", len(messages[g.ChannelId]))
			g.Reply <- messages[g.ChannelId]
		case p := <-put:
			log.Println("messageManager put " + p.ChannelId)
			if _, ok := messages[p.ChannelId]; !ok {
				log.Println("messageManager put getting1 " + p.ChannelId)
				msgs, err := GetSlackMessagesUntil(token, p.ChannelId, p.Time)
				log.Println("messageManager get got1 " + p.ChannelId)
				if err != nil {
					log.Panicln(err) // TODO(return error)
				}
				log.Println("messageManager creating term msgs1 " + p.ChannelId)
				messages[p.ChannelId] = slackMessagesToTermMsgs(msgs, getUserNameChan)
			}
			// TODO(check time of new messages vs newest in map, to fix race condition potentially causing duplicate messages)
			log.Println("messageManager putting " + p.ChannelId + " " + p.Text)
			log.Printf("messageManager put len %d\n", len(messages[p.ChannelId]))
			messages[p.ChannelId] = append([]TermMsg{TermMsg{UserName: GetUserName(p.UserId, getUserNameChan), Text: p.Text}}, messages[p.ChannelId]...) // this is horribly inefficient, and could be made more efficient if necessary
			log.Printf("messageManager put new len %d\n", len(messages[p.ChannelId]))
		}
	}
}

func StartMessagesManager(token string, getUserNameChan chan<- UserNameRequest) (chan<- MessageRequest, chan<- SlackRtmMessage) {
	getChan := make(chan MessageRequest)
	putChan := make(chan SlackRtmMessage)
	go messagesManager(token, getChan, putChan, getUserNameChan)
	return getChan, putChan
}
