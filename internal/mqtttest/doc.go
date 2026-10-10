// Package mqtttest provides an in-process MQTT broker for tests.
//
// The broker listens on a loopback port, answers CONNECT, and counts the PUBLISH
// and DISCONNECT packets it receives, so tests can check that a client delivered
// a message and closed its connection cleanly without contacting a real broker.
package mqtttest
