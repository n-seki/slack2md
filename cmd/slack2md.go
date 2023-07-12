package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/slack-go/slack"
	"github.com/spf13/cobra"
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

var cmd = &cobra.Command{
	Use:   "slack2md",
	Short: "save slack messages as Markdown file",
	Run: func(cmd *cobra.Command, args []string) {
		token, err := cmd.Flags().GetString("token")
		if err != nil {
			log.Fatal(err)
		}
		channels, err := cmd.Flags().GetStringArray("channels")
		if err != nil {
			log.Fatal(err)
		}
		users, err := cmd.Flags().GetStringArray("users")
		if err != nil {
			log.Fatal(err)
		}
		output, err := cmd.Flags().GetString("output")
		if err != nil {
			log.Fatal(err)
		}
		Slack2md(token, channels, users, output)
	},
}

func init() {
	cobra.OnInitialize()
	cmd.PersistentFlags().StringP("token", "t", "", "slack api token (required)")
	cmd.MarkPersistentFlagRequired("token")
	cmd.PersistentFlags().StringArrayP("channels", "c", nil, "include channel id (required)")
	cmd.MarkPersistentFlagRequired("channels")
	cmd.PersistentFlags().StringArrayP("users", "u", nil, "include user id (option)")
	cmd.PersistentFlags().StringP("output", "o", "", "output file (required)")
	cmd.MarkPersistentFlagRequired("output")
}

func Execute() error {
	return cmd.Execute()
}

func Slack2md(
	token string,
	includeChannels []string,
	includeUsers []string,
	output string,
) {
	slackMessages, err := getSlackMessages(token, includeChannels, includeUsers)
	if err != nil {
		log.Fatal(err)
	}
	f, err := os.Create(output)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	err = makeMarkdown(slackMessages, f)
	if err != nil {
		log.Fatal(err)
	}
}

func getSlackMessages(
	token string,
	includeChannels []string,
	includeUsers []string,
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
	latest := strconv.FormatInt(time.Now().Add(time.Hour*-24).Unix(), 10)
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
		messages, err := getMessages(*api, channelID, latest, includeUsers)
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
		if includeUsers != nil && !slices.Contains(includeUsers, m.User) {
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

func makeMarkdown(slackMessages []SlackMessage, output *os.File) error {
	for _, slackMessage := range slackMessages {
		_, err := output.WriteString("# " + slackMessage.channelName + "\n")
		if err != nil {
			return err
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
		if err != nil {
			return err
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
	Style *slack.RichTextSectionTextStyle `json:"style,omitempty"`
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
	Type string `json:"type"`
	Text string `json:"text"`
}

func convertRichTextQuoteToMd(elem slack.RichTextElement) (string, error) {
	rtu := elem.(*slack.RichTextUnknown)
	var rtq RichTextQuote
	err := json.Unmarshal([]byte(rtu.Raw), &rtq)
	if err != nil {
		return "", err
	}
	ss := []string{}
	for _, elem := range rtq.Elements {
		for _, s := range strings.Split(elem.Text, "\n") {
			ss = append(ss, "> "+s)
		}
	}
	return strings.Join(ss, "  \n"), nil
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
