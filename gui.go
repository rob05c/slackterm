package main

import (
	"fmt"
	"github.com/jroimartin/gocui"
	"hash/fnv"
	"io"
	"log"
	"math"
	"strings"
)

// EnterTheGui creates the GUI and enters a loop. This function does not return
// until the user sends the kill signal C-c
// TODO(make start a goroutine and return with a channel to kill it)
func EnterTheGui(slackToken string,
	putChannelIdChan chan<- PutChannelInfo,
	getChannelIdChan chan<- ChannelIdRequest,
	getUserNameChan chan<- UserNameRequest,
	getMessagesChan chan<- MessageRequest,
	getChannelNameChan chan<- ChannelNameRequest,
	updateMsgsChan <-chan string,
	sendMsgChan chan<- PutRtmMsg) {

	g := gocui.NewGui()
	if err := g.Init(); err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	g.SetLayout(layout)
	layout(g) // draw once, to create views

	// if err := g.Flush(); err != nil {
	// 	log.Panicln(err)
	// }

	if err := g.SetCurrentView("channels"); err != nil {
		log.Panicln(err)
	}

	if err := populateChannels(g, slackToken, putChannelIdChan); err != nil {
		log.Panicln(err)
	}

	setKeybindings(g, slackToken, getChannelIdChan, getUserNameChan, getMessagesChan, sendMsgChan)

	go guiUpdater(g, updateMsgsChan, getUserNameChan, getChannelNameChan, getMessagesChan, slackToken)

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}
}

func layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	maxY = maxY - 1

	const channelsWidth = 30
	const inputHeight = 1
	const messageNamesWidth = 20 // TODO(dynamically get widest name?)

	// the -1 everywhere is subtracting borders

	log.Println("DEBUG setting view")
	if v, err := g.SetView("channels", 0, 0, channelsWidth-1, maxY-inputHeight); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.SelFgColor = gocui.AttrReverse
		v.SelBgColor = gocui.AttrReverse
		//		v.SelFgColor = gocui.ColorGreen | gocui.AttrBold
		//		v.SelBgColor = gocui.ColorBlack | gocui.AttrBold
		v.Highlight = true
	}

	//	messagesWidth := maxX - channelsWidth
	if v, err := g.SetView("messages-names", channelsWidth, 0, channelsWidth+messageNamesWidth, maxY-inputHeight); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.FgColor = gocui.ColorRed | gocui.AttrBold
		v.BgColor = gocui.ColorBlack | gocui.AttrBold
		v.Highlight = false
		v.Frame = false
	}

	if v, err := g.SetView("messages", channelsWidth+messageNamesWidth, 0, maxX-1, maxY-inputHeight); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
	}

	if v, err := g.SetView("input", 0, maxY-inputHeight-1, maxX-1, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Editable = true
		v.Autoscroll = true
		v.Wrap = true
		v.SetCursor(0, 0)
		v.SetOrigin(0, 0)
	}

	return nil
}

func quit(g *gocui.Gui, v *gocui.View) error {
	return gocui.ErrQuit
}

func populateChannels(g *gocui.Gui, token string, putChannelIdChan chan<- PutChannelInfo) error {
	channelsView, err := g.View("channels")
	if err != nil {
		return err
	}

	slackChannels, err := GetSlackChannels(token)
	if err != nil {
		return err
	}

	for _, channel := range slackChannels {
		putChannelIdChan <- PutChannelInfo{channel.Id, channel.Name}
	}

	for _, channel := range slackChannels {
		fmt.Fprintln(channelsView, channel.Name)
	}

	return nil
}

func cursorDown(g *gocui.Gui, v *gocui.View) error {
	if v == nil {
		return nil
	}
	cx, cy := v.Cursor()

	nextLine, err := v.Line(cy + 1)
	if err != nil {
		return err
	}
	if nextLine == "" {
		return nil
	}

	if err := v.SetCursor(cx, cy+1); err != nil {
		ox, oy := v.Origin()
		if err := v.SetOrigin(ox, oy+1); err != nil {
			return err
		}
	}
	return nil
}

func cursorUp(g *gocui.Gui, v *gocui.View) error {
	if v == nil {
		return nil
	}
	ox, oy := v.Origin()
	cx, cy := v.Cursor()
	if err := v.SetCursor(cx, cy-1); err != nil && oy > 0 {
		if err := v.SetOrigin(ox, oy-1); err != nil {
			return err
		}
	}
	return nil
}

func selectChannel(g *gocui.Gui, v *gocui.View, token string, getChannelIdChan chan<- ChannelIdRequest, getUserNameChan chan<- UserNameRequest, getMessagesChan chan<- MessageRequest) error {
	log.Println("selectChannel called")
	_, cy := v.Cursor()
	channel, err := v.Line(cy)
	if err != nil {
		log.Println("selectChannel returning err")
		return err
	}
	log.Println("selectChannel calling getChannelId with " + channel)
	channelId := GetChannelId(channel, getChannelIdChan)
	log.Println("selectChannel calling populateMessages")
	err = populateMessages(g, token, channelId, getUserNameChan, getMessagesChan)
	log.Println("selectChannel returning")
	return err
}

// TODO(make asynchronous, so the GUI doesn't hang)
func populateMessages(g *gocui.Gui,
	token string,
	channelId string,
	getUserNameChan chan<- UserNameRequest,
	getMessagesChan chan<- MessageRequest) error {
	log.Println("populateMessages called")
	v, err := g.View("messages")
	if err != nil {
		return err
	}
	vn, err := g.View("messages-names")
	if err != nil {
		return err
	}

	v.Clear()
	vn.Clear()
	fmt.Fprintln(v, "Loading Messages...")
	//	g.Flush()

	msgs := GetMessages(channelId, getMessagesChan)

	_, vHeight := v.Size()
	lastMsgsI := int(math.Max(math.Min(float64(vHeight-1), float64(len(msgs)-1)), 0))
	msgs = msgs[:lastMsgsI]
	v.Clear()
	vn.Clear()

	blankHeight := vHeight - len(msgs)
	for i := 0; i < blankHeight; i++ {
		fmt.Fprintln(v, "")
		fmt.Fprintln(vn, "")
	}

	padName := func(name string, width int) string {
		if name == "" {
			return name
		}

		for len(name) < width {
			name = " " + name
		}
		return name
	}

	// consistentHashColorName colors each name with a consistent, unique color
	consistentHashColorName := func(name string) string {
		h := fnv.New32a()
		io.WriteString(h, name)
		hNum := (h.Sum32() % 7) + 1
		return fmt.Sprintf("\033[1;3%dm%s\033[0m", hNum, name)
		return name
	}

	vnWidth, _ := vn.Size()

	for i := len(msgs) - 1; i >= 0; i-- {
		// For now, strip newlines, to work with the dumb logic printing the number of messages as the screen height
		msg := msgs[i]
		msgtxt := strings.Replace(strings.TrimRight(msg.Text, " \n\t"), "\n", "", -1) // TODO(print newlines [which requires accounting for them when getting the number of lines to print])
		fmt.Fprintln(vn, consistentHashColorName(padName(msg.UserName, vnWidth)))
		fmt.Fprintln(v, msgtxt)
		//		g.Flush()
	}

	// // debug
	// //	fmt.Fprintln(v, channelId)
	//	g.Flush()
	log.Println("populateMessages returning")
	return nil
}

func nextView(g *gocui.Gui, v *gocui.View) error {
	g.Cursor = false
	v.Highlight = false
	log.Println("nextView: " + v.Name())
	var err error
	switch v.Name() {
	case "channels":
		err = g.SetCurrentView("input")
		g.Cursor = true
	case "input":
		err = g.SetCurrentView("channels")
		g.CurrentView().Highlight = true
	}
	//	g.Flush()
	return err
}

// TODO(strip newlines only at cursor position)
func inputEnter(g *gocui.Gui, v *gocui.View, getChannelIdChan chan<- ChannelIdRequest, sendMsgChan chan<- PutRtmMsg) error {
	text := strings.Replace(strings.TrimRight(v.Buffer(), " \n\t"), "\n", "", -1)
	log.Println("Entered Text: X" + text + "X")

	channelName, err := getSelectedChannelName(g)
	if err != nil {
		return err
	}
	channelId := GetChannelId(channelName, getChannelIdChan)

	sendMsgChan <- PutRtmMsg{channelId, text}

	v.Clear()
	v.SetCursor(0, 0)
	v.SetOrigin(0, 0)
	return nil
}

func setKeybindings(g *gocui.Gui, token string, getChannelIdChan chan<- ChannelIdRequest, getUserNameChan chan<- UserNameRequest, getMessagesChan chan<- MessageRequest, sendMsgChan chan<- PutRtmMsg) {
	if err := g.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, quit); err != nil {
		log.Panicln(err)
	}
	if err := g.SetKeybinding("channels", gocui.KeyArrowDown, gocui.ModNone, cursorDown); err != nil {
		log.Panicln(err)
	}
	if err := g.SetKeybinding("channels", gocui.KeyArrowUp, gocui.ModNone, cursorUp); err != nil {
		log.Panicln(err)
	}
	if err := g.SetKeybinding("channels", gocui.KeyCtrlP, gocui.ModNone, cursorUp); err != nil {
		log.Panicln(err)
	}
	if err := g.SetKeybinding("channels", gocui.KeyCtrlN, gocui.ModNone, cursorDown); err != nil {
		log.Panicln(err)
	}
	if err := g.SetKeybinding("channels", gocui.KeyTab, gocui.ModNone, nextView); err != nil {
		log.Panicln(err)
	}
	if err := g.SetKeybinding("input", gocui.KeyTab, gocui.ModNone, nextView); err != nil {
		log.Panicln(err)
	}
	if err := g.SetKeybinding("input", gocui.KeyEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return inputEnter(g, v, getChannelIdChan, sendMsgChan)
	}); err != nil {
		log.Panicln(err)
	}

	if err := g.SetKeybinding("channels", gocui.KeyEnter, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return selectChannel(g, v, token, getChannelIdChan, getUserNameChan, getMessagesChan)
	}); err != nil {
		log.Panicln(err)
	}
}

func getSelectedChannelName(g *gocui.Gui) (string, error) {
	v, err := g.View("channels")
	if err != nil {
		return "", err
	}
	_, cy := v.Cursor()
	name, err := v.Line(cy)
	if err != nil {
		return "", err
	}
	return name, nil
}

func guiUpdater(g *gocui.Gui,
	updateMsgsChan <-chan string,
	getUserNameChan chan<- UserNameRequest,
	getChannelNameChan chan<- ChannelNameRequest,
	getMessagesChan chan<- MessageRequest,
	slackToken string) {
	for {
		log.Println("guiUpdater listening")
		select {
		case channelId := <-updateMsgsChan:
			log.Println("guiUpdater not listening")
			log.Println("gui updater got " + channelId)
			channelName := GetChannelName(channelId, getChannelNameChan)
			log.Println("guiupdater name " + channelName)
			selectedChannelName, err := getSelectedChannelName(g)
			log.Println("guiupdater selectedname " + selectedChannelName)
			if err != nil {
				log.Panicln(err)
			}
			if channelName != selectedChannelName {
				log.Println("guiupdater continuing")
				continue
			}
			log.Println("guiupdater populating messages")
			err = populateMessages(g, slackToken, channelId, getUserNameChan, getMessagesChan)
			log.Println("guiupdater populated msgs")
			if err != nil {
				log.Panicln(err)
			}
		}
	}
}
