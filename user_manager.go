package main

import (
	"log"
)

func slackUserIdMap(users []SlackUser) map[string]SlackUser {
	m := make(map[string]SlackUser)
	for _, user := range users {
		m[user.Id] = user
	}
	return m
}

type UserNameRequest struct {
	Id    string
	Reply chan<- string
}

func GetUserName(id string, getUserNameChan chan<- UserNameRequest) string {
	replyChan := make(chan string)
	getUserNameChan <- UserNameRequest{id, replyChan}
	return <-replyChan
}

// userManager manages user data, and returns it via channels
// TODO(add terminate channel, to safely shut down the manager)
func userManager(token string, getName <-chan UserNameRequest) {
	userSlice, err := GetSlackUsers(token)
	if err != nil {
		log.Panicln(err) // TODO(fix to return error, not panic)
	}
	users := slackUserIdMap(userSlice)
	users["me"] = SlackUser{Id: "me", Name: "me", Deleted: false} // TODO(get self user from API)
	for {
		select {
		case g := <-getName:
			user, ok := users[g.Id]
			if !ok {
				user, err := GetSlackUser(token)
				if err != nil {
					// TODO(log or return error)
					g.Reply <- ""
					continue
				}
				g.Reply <- user.Name
				continue
			}
			g.Reply <- user.Name
		}
	}
}

func StartUserManager(token string) chan<- UserNameRequest {
	getChan := make(chan UserNameRequest)
	go userManager(token, getChan)
	return getChan
}
