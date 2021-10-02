# JIMBO Bot

## Description

This is a small Discord bot to get user interaction for things like events and polls. This bot uses the [github.com/bwmarrin/discordgo](http://github.com/bwmarrin/discordgo) package as the interface to Discord. Slash commands are used over `!commands` because of the [impending privledged intent changes](https://support-dev.discord.com/hc/en-us/articles/4404772028055-Message-Content-Access-Deprecation-for-Verified-Bots) to the Discord API. For persistant data storage this project uses sqlite3.

## Commands

| Command | Implemented |
| :-- | :-: |
| /event create | :heavy\_check\_mark: |
| /event get | :x: |

## How to run the Bot

1. Download the executable from the latest release.
2. Set your Bot token (located in the Bot tab of your selected discord app)

```
$ export TOKEN=<bot token here>
```

3. Run the executable

```
$ ./jimbo-bot
```