# slack2md

`slack2md` is a cli tool.<br>
`slack2md` get Slack messages and convert them to Markdown format.

```
$ ./slack2md --help
slack2md get Slack messages and convert them to Markdown format

Usage:
  slack2md [flags]

Flags:
      --config string   Path to config yaml (requred)
  -h, --help            help for slack2md
  -t, --token string    slack api token (required)
```

Config YAML format

```yaml
output: string # path/to/markdown (required)
since: int # since x days ago (default: 1)
users: string array # include user id
channels:
  - id: string # slack channel id
    header: string # markdown header (default: slack channel name)
    no_header: bool # not output markdown header (default: false)
    usres: string array # include user id (override global settings)
  - id: string
    header: string
    no_header: bool
    usres: string array
```

Example:

```yaml
output: ./20240808.md
since: 1
users: [user_id_1, user_id_2]
channels:
  - id: CXXXXXXA
    header: HEADER1
  - id: CXXXXXXB
```

```sh
./slack2md \
    --token your_slack_token_with_read_scope \
    --config config.yaml
```

then `20240808.md` ceated with below content

```md
# HEADER1

Message

Reply


# CXXXXXXB Channel Name

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