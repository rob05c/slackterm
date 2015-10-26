package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

type SlackValue struct {
	Value   string `json: "value"`
	Creator string `json: "creator"`
	LastSet int    `json: "last_set"`
}

type SlackChannel struct {
	Id         string     `json: "id"`
	Name       string     `json: "name"`
	Created    int64      `json: "created"`
	Creator    string     `json: "creator"`
	IsArchived bool       `json: "is_archived"`
	IsMember   bool       `json: "is_member"`
	NumMembers int        `json: "num_members"`
	Topic      SlackValue `json: "topic"`
	Purpose    SlackValue `json: "purpose"`
}

type SlackChannelRequest struct {
	Ok      bool         `json:"ok"`
	Channel SlackChannel `json:"channel"`
}

type SlackChannels struct {
	Ok       bool           `json:"ok"`
	Channels []SlackChannel `json:"channels"`
}

type SlackMessage struct {
	Type    string `json:"type"`
	Time    string `json:"ts"`
	User    string `json:"user"`
	Text    string `json:"text"`
	Starred bool   `json:"is_starred"`
}

type SlackHistory struct {
	Ok       bool           `json:"ok"`
	Latest   string         `json:"latest"`
	Messages []SlackMessage `json:"messages"`
	HasMore  bool           `json:"has_more"`
}

type SlackProfile struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	RealName  string `json:"real_name"`
	Email     string `json:"email"`
	Skype     string `json:"skype"`
	Phone     string `json:"phone"`
}

type SlackUser struct {
	Id      string       `json: "id"`
	Name    string       `json: "name"`
	Deleted bool         `json: "deleted"`
	Color   string       `json: "color"`
	Profile SlackProfile `json: "profile"`
	Admin   bool         `json:"is_admin"`
	Owner   bool         `json:"is_owner"`
}

type SlackUsers struct {
	Ok      bool        `json:"ok"`
	Members []SlackUser `json:"members"`
}

type SlackUserInfo struct {
	Ok   bool      `json:"ok"`
	User SlackUser `json:"user"`
}

func GetSlackChannels(token string) ([]SlackChannel, error) {
	response, err := http.Get(`https://slack.com/api/channels.list?token=` + token)
	if err != nil {
		return nil, err
	}

	if response.Status != `200 OK` {
		return nil, errors.New("Unexpected Response: " + response.Status)
	}

	var slackChannels SlackChannels
	if err = json.NewDecoder(response.Body).Decode(&slackChannels); err != nil {
		return nil, err
	}
	if !slackChannels.Ok {
		return slackChannels.Channels, errors.New("Slack Channels response not ok")
	}
	return slackChannels.Channels, nil
}

func GetSlackChannel(token, channelId string) (SlackChannel, error) {
	response, err := http.Get(`https://slack.com/api/channels.info?token=` + token + `&channel=` + channelId)
	if err != nil {
		return SlackChannel{}, err
	}

	if response.Status != `200 OK` {
		return SlackChannel{}, errors.New("Unexpected Response: " + response.Status)
	}

	var slackChannel SlackChannelRequest
	if err = json.NewDecoder(response.Body).Decode(&slackChannel); err != nil {
		return SlackChannel{}, err
	}
	if !slackChannel.Ok {
		return slackChannel.Channel, errors.New("Slack Channel response not ok")
	}
	return slackChannel.Channel, nil
}

// GetSlackMessages gets all slack messages on the given channel.
func GetAllSlackMessages(token string, channel string) ([]SlackMessage, error) {
	return GetSlackMessages(token, channel, "", "")
}

// GetSlackMessagesSince gets the slack messages sent after oldest
func GetSlackMessagesSince(token string, channel string, oldest string) ([]SlackMessage, error) {
	return GetSlackMessages(token, channel, oldest, "")
}

// GetSlackMessagesUntil gets the slack messages up to latest
func GetSlackMessagesUntil(token string, channel string, latest string) ([]SlackMessage, error) {
	return GetSlackMessages(token, channel, "", latest)
}

// GetSlackMessagesSince gets the slack messages sent until until
func GetSlackMessages(token, channel, oldest, latest string) ([]SlackMessage, error) {
	var messages []SlackMessage
	for {
		getstr := `https://slack.com/api/channels.history?token=` + token + `&channel=` + channel + `&latest=` + latest + `&oldest=` + oldest
		log.Println("GetSlackMessages get " + latest + " to " + oldest)
		response, err := http.Get(getstr)
		log.Println("GetSlackMessages got")
		if err != nil {
			return messages, err
		}
		if response.Status != `200 OK` {
			return messages, errors.New("Unexpected Response: " + response.Status)
		}
		var history SlackHistory
		if err = json.NewDecoder(response.Body).Decode(&history); err != nil {
			return messages, err
		}

		if !history.Ok {
			// TODO(fix groups [which are send via RTM as 'channel's])
			// return messages, errors.New("Slack History response not ok: " + getstr)
			return messages, nil
		}
		messages = append(messages, history.Messages...)

		if !history.HasMore {
			break
		}
		if len(messages) == 0 {
			break // TODO(log?)
		}
		latest = messages[len(messages)-1].Time
	}

	return messages, nil
}

func GetSlackUsers(token string) ([]SlackUser, error) {
	response, err := http.Get(`https://slack.com/api/users.list?token=` + token)
	if err != nil {
		return nil, err
	}

	if response.Status != `200 OK` {
		return nil, errors.New("Unexpected Response: " + response.Status)
	}

	var users SlackUsers
	if err = json.NewDecoder(response.Body).Decode(&users); err != nil {
		return nil, err
	}
	if !users.Ok {
		return users.Members, errors.New("Slack Users response not ok")
	}
	return users.Members, nil
}

func GetSlackUser(token string) (SlackUser, error) {
	response, err := http.Get(`https://slack.com/api/users.list?token=` + token)
	if err != nil {
		return SlackUser{}, err
	}

	if response.Status != `200 OK` {
		return SlackUser{}, errors.New("Unexpected Response: " + response.Status)
	}

	var info SlackUserInfo
	if err = json.NewDecoder(response.Body).Decode(&info); err != nil {
		return SlackUser{}, err
	}
	if !info.Ok {
		return info.User, errors.New("Slack User Info response not ok")
	}
	return info.User, nil
}
