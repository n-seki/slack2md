package main

import (
	"log"
	"path/filepath"

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
		configPath, err := cmd.Flags().GetString("config")
		if err != nil {
			log.Fatal(err)
		}
		absConfigPath := ""
		if len(configPath) > 0 {
			absConfigPath, err = filepath.Abs(configPath)
			if err != nil {
				log.Fatal(err)
			}
		}
		slack2md.Slack2md(token, absConfigPath)
	},
}

func init() {
	cobra.OnInitialize()
	cmd.PersistentFlags().StringP("token", "t", "", "slack api token (required)")
	cmd.MarkPersistentFlagRequired("token")
	cmd.PersistentFlags().String("config", "", "Path to config yaml (requred)")
	cmd.MarkPersistentFlagRequired("config")
}

func main() {
	cmd.Execute()
}
