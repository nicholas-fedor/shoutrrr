// Package signalgrid provides notification support for Signalgrid.
//
// URL format:
//
//	signalgrid://CLIENT_KEY@CHANNEL
//
// Optional query parameters:
//
//	title
//	type
//	critical
//
// Example:
//
//	signalgrid://clientkey@channeltoken?title=Server%20Alert&type=CRIT
package signalgrid
