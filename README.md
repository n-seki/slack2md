# slack2md

`slack2md` is a cli tool.<br>
`slack2md` get Slack messages and convert them to Markdown format.

```
$ ./slack2md --help
slack2md get Slack messages and convert them to Markdown format

Usage:
  slack2md [flags]

Flags:
  -c, --channels stringArray   include channel id (option)
  -h, --help                   help for slack2md
  -o, --output string          output file (required)
  -t, --token string           slack api token (required)
  -u, --users stringArray      include user id (option)
```

Example:

```
./slack2md \
    --token your_slack_token_with_read_scope \
    --output 20230101.md \
    --channels slack_chanel_1_id \
    --channels slack_chanel_2_id \
    --users user_id
```

then `20230101.md` ceated with below content

```
# channel 1 name

Message

Reply


# channel 2 name

Message
```

# Supported
- Message
- Reply
- RichText
  - Preformatted
  - List(bullet, ordered)
  - Style
  - Link
  - Quote

# Not currently supported
- Paging
  - using api default limit(100 per channel)
- Specify retriving period 
  - slack2md get messages within just 24 hours.