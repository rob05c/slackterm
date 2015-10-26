package main

import (
	//	"fmt"
	"io/ioutil"
	"log"
	"os"
)

const tokenFile = `slack_token`

func getToken() (string, error) {
	f, err := ioutil.ReadFile(tokenFile)
	return string(f), err
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
		log.Panicln(err)
	}

	putChannelIdChan, getChannelIdChan, getChannelNameChan := StartChannelIdManager(slackToken)
	getUserNameChan := StartUserManager(slackToken)
	getMessagesChan, putMessageChan := StartMessagesManager(slackToken, getUserNameChan)
	updateMsgsChan, sendMsgChan := StartSlackRtmHandler(slackToken, putMessageChan)

	EnterTheGui(slackToken, putChannelIdChan, getChannelIdChan, getUserNameChan, getMessagesChan, getChannelNameChan, updateMsgsChan, sendMsgChan)
}
