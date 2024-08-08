package slack2md

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-yaml"
	"github.com/slack-go/slack"
	"golang.org/x/exp/slices"
)

type SlackMessage struct {
	channelID   string
	channelName string
	messages    []Message
}

type Message struct {
	msg    slack.Msg
	relies []slack.Msg
}

type Slack2mdConfg struct {
	Output         string          `yaml:"output"`
	Since          int             `yaml:"since"`
	Users          []string        `yaml:"users"`
	ChannelConfigs []ChannelConfig `yaml:"channels"`
}

type ChannelConfig struct {
	Id       string   `yaml:"id"`
	Header   string   `yaml:"header"`
	NoHeader bool     `yaml:"no_header"`
	Users    []string `yaml:"users"`
}

func (config Slack2mdConfg) getIncludeChannels() []string {
	channels := []string{}
	for _, channelConfig := range config.ChannelConfigs {
		channels = append(channels, channelConfig.Id)
	}
	return channels
}

func (config Slack2mdConfg) getIncludeUsers() map[string][]string {
	users := make(map[string][]string)
	for _, channelConfig := range config.ChannelConfigs {
		if len(channelConfig.Users) > 0 {
			users[channelConfig.Id] = channelConfig.Users
		} else {
			users[channelConfig.Id] = config.Users
		}
	}
	return users
}

func (config Slack2mdConfg) getChannelConfig() map[string]ChannelConfig {
	configs := make(map[string]ChannelConfig)
	for _, channelConfig := range config.ChannelConfigs {
		configs[channelConfig.Id] = channelConfig
	}
	return configs
}

func Slack2md(
	token string,
	configPath string,
) {
	config, err := readConfig(configPath)
	if err != nil {
		log.Fatal(err)
	}
	slackMessages, err := getSlackMessages(token, config.getIncludeChannels(), config.getIncludeUsers(), config.Since)
	if err != nil {
		log.Fatal(err)
	}
	f, err := os.Create(config.Output)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	err = makeMarkdown(slackMessages, f, config.getChannelConfig())
	if err != nil {
		log.Fatal(err)
	}
}

func getSlackMessages(
	token string,
	includeChannels []string,
	includeUsers map[string][]string,
	since int,
) ([]SlackMessage, error) {
	api := slack.New(token)

	conversationsParam := slack.GetConversationsParameters{
		ExcludeArchived: false,
		Types:           []string{"public_channel,private_channel"},
	}
	channels, _, err := api.GetConversations(&conversationsParam)
	if err != nil {
		return nil, err
	}
	latest := strconv.FormatInt(time.Now().AddDate(0, 0, -since).Unix(), 10)
	if err != nil {
		return nil, err
	}

	allChannelName := map[string]string{}
	for _, c := range channels {
		allChannelName[c.ID] = c.Name
	}

	slackMessages := []SlackMessage{}
	for _, channelID := range includeChannels {
		if _, ok := allChannelName[channelID]; !ok {
			continue
		}
		messages, err := getMessages(*api, channelID, latest, includeUsers[channelID])
		if err != nil {
			return nil, err
		}
		if len(messages) == 0 {
			continue
		}
		slackMessage := SlackMessage{
			channelID:   channelID,
			channelName: allChannelName[channelID],
			messages:    messages,
		}
		slackMessages = append(slackMessages, slackMessage)
	}
	return slackMessages, nil
}

func getMessages(
	api slack.Client,
	channelID string,
	latest string,
	includeUsers []string,
) ([]Message, error) {
	param := slack.GetConversationHistoryParameters{
		ChannelID: channelID,
		Oldest:    latest,
	}
	conversation, err := api.GetConversationHistory(&param)
	if err != nil {
		return nil, err
	}
	if !conversation.Ok {
		return nil, fmt.Errorf("GetConversationHistory Failed: " + conversation.Error)
	}

	messages := []Message{}

	l := len(conversation.Messages)
	for i := (l - 1); i >= 0; i-- {
		m := conversation.Messages[i]
		if includeUsers != nil && !slices.Contains(includeUsers, m.User) && !slices.Contains(includeUsers, m.BotID) {
			continue
		}
		message := Message{msg: m.Msg}
		if m.ReplyCount != 0 {
			param := slack.GetConversationRepliesParameters{
				ChannelID: channelID,
				Timestamp: m.Timestamp,
			}
			reps, _, _, err := api.GetConversationReplies(&param)
			if err != nil {
				return nil, err
			}
			for _, reply := range reps {
				message.relies = append(message.relies, reply.Msg)
			}
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func makeMarkdown(
	slackMessages []SlackMessage,
	output *os.File,
	channelConfig map[string]ChannelConfig,
) error {
	for _, slackMessage := range slackMessages {
		config := channelConfig[slackMessage.channelID]
		if !config.NoHeader {
			header := slackMessage.channelName
			if len(config.Header) > 0 {
				header = config.Header
			}
			_, err := output.WriteString("# " + header + "\n")
			if err != nil {
				return err
			}
		}

		for _, message := range slackMessage.messages {
			md, err := convertToMd(message.msg)
			if err != nil {
				return nil
			}
			for _, line := range md {
				_, err = output.WriteString(line)
				if err != nil {
					return err
				}
			}
			for _, reply := range message.relies {
				// exclude root message
				if message.msg.Timestamp == reply.Timestamp {
					continue
				}
				md, err = convertToMd(reply)
				if err != nil {
					return err
				}
				for _, line := range md {
					_, err = output.WriteString(line)
					if err != nil {
						return nil
					}
				}
			}
		}
	}
	return nil
}

func convertToMd(msg slack.Msg) ([]string, error) {
	md := []string{}
	for _, block := range msg.Blocks.BlockSet {
		switch block.BlockType() {
		case slack.MBTRichText:
			richTextBlock := block.(*slack.RichTextBlock)
			for _, elem := range richTextBlock.Elements {
				switch elem.RichTextElementType() {
				case slack.RTEUnknown:
					rtu := elem.(*slack.RichTextUnknown)
					md = append(md, rtu.Raw)
				case slack.RTESection:
					for _, secElem := range elem.(*slack.RichTextSection).Elements {
						switch secElem.RichTextSectionElementType() {
						case slack.RTSELink:
							part := convertRichTextSectionLinkToMd(secElem)
							md = append(md, part...)
						case slack.RTSEText:
							part := convertRichTextSectionTextToMd(secElem)
							md = append(md, part)
						default:
							fmt.Printf("unknown section element: %+#v\n", secElem)
						}
					}
				case slack.RTEPreformatted:
					part, err := convertRichTextPreformattedToMd(elem)
					if err != nil {
						return nil, err
					}
					md = append(md, part...)
				case slack.RTEList:
					part, err := convertRichTextListToMd(elem)
					if err != nil {
						return nil, err
					}
					md = append(md, part...)
				case slack.RTEQuote:
					part, err := convertRichTextQuoteToMd(elem)
					if err != nil {
						return nil, err
					}
					md = append(md, part)
				default:
					fmt.Printf("unknown rich text block: %+#v", elem)
				}
			}
		}
	}
	md = append(md, "\n\n")
	return md, nil
}

// RichTextElement

type RichTextPreformatted struct {
	Type     string                        `json:"type"`
	Elements []RichTextPreformattedElement `json:"elements"`
}

type RichTextPreformattedElement struct {
	Type   string `json:"type"`
	Text   string `json:"text"`
	Url    string `json:"url"`
	Border int64  `json:"border"`
}

func convertRichTextPreformattedToMd(elem slack.RichTextElement) ([]string, error) {
	md := []string{}
	rtu := elem.(*slack.RichTextUnknown)
	var rtf RichTextPreformatted
	err := json.Unmarshal([]byte(rtu.Raw), &rtf)
	if err != nil {
		return nil, err
	}
	md = append(md, "```\n")
	for _, preformatted := range rtf.Elements {
		switch preformatted.Type {
		case "link":
			md = append(md, preformatted.Url)
		default:
			md = append(md, preformatted.Text)
		}
	}
	md = append(md, "\n```")
	return md, nil
}

type RichTextList struct {
	Type     string                `json:"type"`
	Elements []RichTextListSection `json:"elements"`
	Style    string                `json:"style"`
	Indent   int                   `json:"indent"`
	Border   int                   `json:"border"`
}

type RichTextListSection struct {
	Type     string                       `json:"type"`
	Elements []RichTextListSectionElement `json:"elements"`
}

type RichTextListSectionElement struct {
	Type  string                          `json:"type"`
	Text  string                          `json:"text"`
	Url   string                          `json:"url"`
	Style *slack.RichTextSectionTextStyle `json:"style"`
}

func convertRichTextListToMd(elem slack.RichTextElement) ([]string, error) {
	rtu := elem.(*slack.RichTextUnknown)
	var rtl RichTextList
	err := json.Unmarshal([]byte(rtu.Raw), &rtl)
	if err != nil {
		return nil, err
	}
	space := ""
	for i := 0; i < rtl.Indent; i++ {
		space = space + "   "
	}
	head := "* "
	if rtl.Style == "ordered" {
		head = "1. "
	}
	list := []string{}
	for _, elem := range rtl.Elements {
		content := ""
		for _, el := range elem.Elements {
			switch el.Type {
			case "text":
				content = content + decorate(el.Text, el.Style)
			case "link":
				content = content + el.Url
			}
		}
		list = append(list, space+head+content+"\n")
	}
	return list, nil
}

type RichTextQuote struct {
	Type     string                 `json:"type"`
	Elements []RichTextQuoteElement `json:"elements"`
}

type RichTextQuoteElement struct {
	Type  string                          `json:"type"`
	Text  string                          `json:"text"`
	Style *slack.RichTextSectionTextStyle `json:"style"`
}

func convertRichTextQuoteToMd(elem slack.RichTextElement) (string, error) {
	rtu := elem.(*slack.RichTextUnknown)
	var rtq RichTextQuote
	err := json.Unmarshal([]byte(rtu.Raw), &rtq)
	if err != nil {
		return "", err
	}
	text := "> "
	for _, elem := range rtq.Elements {
		text = text + decorate(elem.Text, elem.Style)
	}
	return strings.Replace(text, "\n", "  \n> ", -1) + "\n\n", nil
}

// RichTextSectionElement

func convertRichTextSectionLinkToMd(elem slack.RichTextSectionElement) []string {
	md := []string{}
	link := elem.(*slack.RichTextSectionLinkElement)
	md = append(md, link.URL+"  ")
	return md
}

func convertRichTextSectionTextToMd(elem slack.RichTextSectionElement) string {
	text := elem.(*slack.RichTextSectionTextElement)
	deco := strings.Replace(text.Text, "\n", "  \n", -1)
	return decorate(deco, text.Style)
}

func decorate(text string, style *slack.RichTextSectionTextStyle) string {
	deco := text
	if style != nil {
		if style.Code {
			deco = "`" + deco + "`"
		}
		if style.Strike {
			deco = "~~" + deco + "~~"
		}
		if style.Italic {
			deco = "*" + deco + "*"
		}
		if style.Bold {
			deco = "**" + deco + "**"
		}
	}
	return deco
}

func readConfig(path string) (Slack2mdConfg, error) {
	bytes, err := os.ReadFile(path)
	if err != nil {
		return Slack2mdConfg{}, err
	}
	config := Slack2mdConfg{}
	err = yaml.Unmarshal(bytes, &config)
	if err != nil {
		return Slack2mdConfg{}, err
	}
	if len(config.Output) == 0 {
		err := errors.New("missiing yaml field: output")
		return Slack2mdConfg{}, err
	}
	return config, nil
}
