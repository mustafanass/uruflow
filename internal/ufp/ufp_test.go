/*
 * Copyright (C) 2026 Mustafa Naseer (Mustafa Gaeed)
 *
 * This file is part of uruflow.
 *
 * uruflow is free software: you can redistribute it and/or modify
 * it under the terms of the MIT License as described in the
 * LICENSE file distributed with this project.
 *
 * uruflow is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * MIT License for more details.
 *
 * You should have received a copy of the MIT License
 * along with uruflow. If not, see the LICENSE file in the project root.
 */

package ufp

import (
	"bytes"
	"context"
	"net"
	"testing"
)

func TestHeaderRoundTrip(t *testing.T) {
	header := EncodeHeader(FrameRequest, 512)
	frameType, length, err := DecodeHeader(header)
	if err != nil {
		t.Fatalf("decode header: %v", err)
	}
	if frameType != FrameRequest || length != 512 {
		t.Fatalf("got %s/%d, want REQUEST/512", frameType, length)
	}
}

func TestHeaderRejectsForeignFrames(t *testing.T) {
	cases := map[string][]byte{
		"magic":   {0x00, 0x00, Version, 0x10, 0, 0, 0, 0},
		"version": {MagicHigh, MagicLow, 0xFF, 0x10, 0, 0, 0, 0},
		"oversize": append([]byte{MagicHigh, MagicLow, Version, 0x10},
			byte(0xFF), byte(0xFF), byte(0xFF), byte(0xFF)),
	}
	for name, header := range cases {
		if _, _, err := DecodeHeader(header); err == nil {
			t.Fatalf("%s: expected rejection", name)
		}
	}
}

func TestProofIsDeterministicAndBoundToContext(t *testing.T) {
	nonce := bytes.Repeat([]byte{0x7}, NonceSize)
	secret := []byte("shared")

	if !bytes.Equal(ComputeProof(AuthContext, secret, nonce), ComputeProof(AuthContext, secret, nonce)) {
		t.Fatal("same inputs must produce the same proof")
	}
	if VerifyProof(AuthContext, []byte("wrong"), nonce, ComputeProof(AuthContext, secret, nonce)) {
		t.Fatal("a wrong secret must not verify")
	}
	if VerifyProof("other-context-v1", secret, nonce, ComputeProof(AuthContext, secret, nonce)) {
		t.Fatal("a proof must not verify under another context")
	}
	if VerifyProof(AuthContext, secret, bytes.Repeat([]byte{0x8}, NonceSize), ComputeProof(AuthContext, secret, nonce)) {
		t.Fatal("a proof must not replay against another nonce")
	}
}

type echoHandler struct{ events chan *Event }

func (h *echoHandler) HandleRequest(request *Request) (any, error) {
	return Accepted{JobID: request.Method}, nil
}

func (h *echoHandler) HandleEvent(event *Event) error {
	h.events <- event
	return nil
}

func TestHandshakeAndEnvelopes(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	const secret = "agent-key"
	accepted := make(chan *Identity, 1)
	handler := &echoHandler{events: make(chan *Event, 1)}

	go func() {
		conn, identity, err := Accept(server, func(string) (string, string, bool) {
			return secret, "builder-01", true
		})
		if err != nil {
			accepted <- nil
			return
		}
		accepted <- identity
		go conn.Serve(context.Background(), handler)
	}()

	hello := Hello{AgentID: "a1", Hostname: "box", Version: "2.0.0", Roles: []Role{RoleBuilder}}
	conn, welcome, err := Dial(client, hello, secret)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	if welcome.Name != "builder-01" {
		t.Fatalf("welcome name = %q", welcome.Name)
	}

	identity := <-accepted
	if identity == nil || !HasRole(identity.Roles, RoleBuilder) {
		t.Fatal("server did not accept the builder role")
	}

	go conn.Serve(context.Background(), handler)

	response, err := conn.Request(context.Background(), MethodBuildRun, BuildRequest{JobID: "j1"})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	var payload Accepted
	if err := response.Decode(&payload); err != nil || payload.JobID != MethodBuildRun {
		t.Fatalf("response = %+v, err = %v", payload, err)
	}

	if err := conn.SendEvent(TopicJobLog, JobLog{JobID: "j1", Line: "building"}); err != nil {
		t.Fatalf("send event: %v", err)
	}
	event := <-handler.events
	var jobLog JobLog
	if err := event.Decode(&jobLog); err != nil || jobLog.Line != "building" {
		t.Fatalf("event = %+v, err = %v", jobLog, err)
	}
}

func TestHandshakeRejectsWrongSecret(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go Accept(server, func(string) (string, string, bool) { return "right", "agent", true })

	hello := Hello{AgentID: "a1", Roles: []Role{RoleRunner}}
	if _, _, err := Dial(client, hello, "wrong"); err == nil {
		t.Fatal("expected the handshake to fail on a wrong secret")
	}
}

func TestHandshakeRejectsRolelessAgent(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go Accept(server, func(string) (string, string, bool) { return "key", "agent", true })

	if _, _, err := Dial(client, Hello{AgentID: "a1"}, "key"); err == nil {
		t.Fatal("expected the handshake to fail without a role")
	}
}
