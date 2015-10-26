package main

import (
	_ "errors" // debug
	"log"
)

type ChannelIdRequest struct {
	Name  string
	Reply chan string
}

type ChannelNameRequest struct {
	Id    string
	Reply chan string
}

type PutChannelInfo struct {
	Id   string
	Name string
}

func GetChannelId(name string, getChannelIdChan chan<- ChannelIdRequest) string {
	replyChan := make(chan string)
	getChannelIdChan <- ChannelIdRequest{name, replyChan}
	return <-replyChan
}

func GetChannelName(id string, getChannelNameChan chan<- ChannelNameRequest) string {
	replyChan := make(chan string)
	getChannelNameChan <- ChannelNameRequest{id, replyChan}
	return <-replyChan
}

// TODO(remove, and write to chan directly?)
func PutChannelId(name, id string, putChannelIdChan chan<- PutChannelInfo) {
	putChannelIdChan <- PutChannelInfo{name, id}
}

// channelIdManager acts like a CSP map, with put and get operations via channels
// TODO(add terminate channel, to safely shut down the manager)
// TODO(load group and personal channels)
func channelIdManager(token string, put <-chan PutChannelInfo, get <-chan ChannelIdRequest, getName <-chan ChannelNameRequest) {
	// TODO(create name and id types?)
	channels := make(map[string]string)     // map[name]id
	channelNames := make(map[string]string) // map[id]name
	for {
		select {
		case p := <-put:
			channels[p.Name] = p.Id
			channelNames[p.Id] = p.Name
		case g := <-get:
			id, ok := channels[g.Name]
			if !ok {
				g.Reply <- "not found" // debug
				continue
				// Shouldn't be possible to have an name without an id.
				// If it was, we could iterate names via channels.list
				//				log.Panicln(errors.New("channel id requested for name that doesn't exist")) // debug
			}
			g.Reply <- id
		case gn := <-getName:
			name, ok := channelNames[gn.Id]
			if !ok {
				newName, err := GetSlackChannel(token, gn.Id)
				if err != nil {
					log.Println("channelIdManager error getting channel: " + err.Error())
					gn.Reply <- "" // TODO(fix to get group and private channels, and resume panicing)
					continue
				}
				name = newName.Name
				channels[name] = gn.Id
				channelNames[gn.Id] = name
			}
			gn.Reply <- name
		}
	}
}

func StartChannelIdManager(token string) (chan<- PutChannelInfo, chan<- ChannelIdRequest, chan<- ChannelNameRequest) {
	putChan := make(chan PutChannelInfo)
	getChan := make(chan ChannelIdRequest)
	getNameChan := make(chan ChannelNameRequest)
	go channelIdManager(token, putChan, getChan, getNameChan)
	return putChan, getChan, getNameChan
}
