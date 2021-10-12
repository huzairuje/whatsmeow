// Copyright (c) 2021 Tulir Asokan
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package multidevice

import (
	waBinary "go.mau.fi/whatsmeow/binary"
)

type nodeHandler func(node *waBinary.Node) bool

func (cli *Client) handleStreamError(node *waBinary.Node) bool {
	if node.Tag != "stream:error" {
		return false
	}
	code, _ := node.Attrs["code"].(string)
	switch code {
	case "515":
		cli.Log.Debugln("Got 515 code, reconnecting")
		go func() {
			cli.Disconnect()
			err := cli.Connect()
			if err != nil {
				cli.Log.Errorln("Failed to reconnect after 515 code:", err)
			}
		}()
	}
	return true
}

func (cli *Client) handleDevicesNotification(node *waBinary.Node) bool {
	if node.Tag != "notification" || node.AttrGetter().String("type") != "account_sync" {
		return false
	}
	id, _ := node.Attrs["id"].(string)
	go func() {
		cli.Log.Debugln("Received device list update")
		err := cli.sendNode(waBinary.Node{
			Tag: "ack",
			Attrs: map[string]interface{}{
				"id":    id,
				"type":  "account_sync",
				"class": "notification",
				"to":    waBinary.NewJID(cli.Session.ID.User, waBinary.UserServer),
			},
		})
		if err != nil {
			cli.Log.Warnfln("Failed to send acknowledgement to device list notification %s: %v", id, err)
		}
	}()
	return true
}

type ConnectedEvent struct{}

func (cli *Client) handleConnectSuccess(node *waBinary.Node) bool {
	if node.Tag != "success" {
		return false
	}
	cli.Log.Infoln("Successfully authenticated")
	go func() {
		if !cli.Session.ServerHasPreKeys() {
			cli.uploadPreKeys()
		}
		err := cli.sendPassiveIQ(false)
		if err != nil {
			cli.Log.Warnln("Failed to send post-connect passive IQ:", err)
		}
		cli.dispatchEvent(&ConnectedEvent{})
	}()
	return true
}

func (cli *Client) sendPassiveIQ(passive bool) error {
	tag := "active"
	if passive {
		tag = "passive"
	}
	_, err := cli.sendIQ(InfoQuery{
		Namespace: "passive",
		Type:      "set",
		To:        waBinary.ServerJID,
		Content:   []waBinary.Node{{Tag: tag}},
	})
	if err != nil {
		return err
	}
	return nil
}