package main

import (
	"log"

	"github.com/n-seki/slack2md"
	"github.com/spf13/cobra"
)

var cmd = &cobra.Command{
	Use:   "slack2md",
	Short: "slack2md get Slack messages and convert them to Markdown format",
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
		slack2md.Slack2md(token, channels, users, output)
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

func main() {
	cmd.Execute()
}
