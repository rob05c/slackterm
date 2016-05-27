package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"
)

const tokenFile = `slack_token`

func getToken() (string, error) {
	tokenBytes, err := ioutil.ReadFile(tokenFile)
	return strings.TrimSpace(string(tokenBytes)), err
}

func main() {
	f, err := os.OpenFile("log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Panicln("error opening file: %v", err)
	}
	defer f.Close()
	log.SetOutput(f)
	log.Println("Starting slackterm")

	slackToken, err := getToken()
	if err != nil {
		fmt.Printf("Failed to get Slack token: %v.\nTo run, create a slack token at https://api.slack.com/web#authentication and put it in a file named 'slack_token' in your path.\n", err)
		log.Printf("error getting Slack token: %v\n", err)
		return
	}

	putChannelIdChan, getChannelIdChan, getChannelNameChan := StartChannelIdManager(slackToken)
	getUserNameChan := StartUserManager(slackToken)
	getMessagesChan, putMessageChan := StartMessagesManager(slackToken, getUserNameChan)
	updateMsgsChan, sendMsgChan := StartSlackRtmHandler(slackToken, putMessageChan)

	EnterTheGui(slackToken, putChannelIdChan, getChannelIdChan, getUserNameChan, getMessagesChan, getChannelNameChan, updateMsgsChan, sendMsgChan)
}
