/*
Package wecom provides notification support for WeChat Work (WeCom) webhook bots.

This package implements the Shoutrrr service interface for sending notifications
to WeChat Work groups via webhook bots. It supports:

- Text message notifications up to 4096 characters
- User mentions using @username or mobile numbers
- Custom webhook bot integration
- Comprehensive error handling and validation

# URL Format

	wecom://WEBHOOK_KEY

# Example Usage

	import "github.com/nicholas-fedor/shoutrrr"

	// Send a simple message
	url := "wecom://693axxx6-7aoc-4bc4-97a0-0ec2sifa5aaa"
	err := shoutrrr.Send(url, "Hello from Shoutrrr!")

	// Send with mentions
	url := "wecom://693axxx6-7aoc-4bc4-97a0-0ec2sifa5aaa?mentioned_list=@all"
	err := shoutrrr.Send(url, "Alert message")

# Setup

1. Create a webhook bot in WeChat Work
2. Copy the webhook URL key (the part after ?key=)
3. Use the key in your Shoutrrr URL: wecom://YOUR_KEY

For more information, see: https://developer.work.weixin.qq.com/document/path/99110
*/
package wecom
