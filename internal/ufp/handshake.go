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
	"encoding/json"
	"fmt"
	"net"
)

const unknownAgentSecret = "uruflow-unknown-agent"

type Identity struct {
	AgentID  string
	Name     string
	Hostname string
	Version  string
	Platform string
	Roles    []Role
}

type SecretLookup func(agentID string) (secret string, name string, found bool)

func Accept(netConn net.Conn, lookup SecretLookup) (*Conn, *Identity, error) {
	conn := NewConn(netConn)

	frame, err := conn.readFrame(FrameHello)
	if err != nil {
		conn.reject("expected HELLO")
		return nil, nil, err
	}

	var hello Hello
	if err := json.Unmarshal(frame.Payload, &hello); err != nil {
		conn.reject("malformed HELLO")
		return nil, nil, err
	}

	roles := validRoles(hello.Roles)
	if len(roles) == 0 {
		conn.reject("agent declared no valid role")
		return nil, nil, fmt.Errorf("ufp: agent %s declared no valid role", hello.AgentID)
	}

	secret, name, found := lookup(hello.AgentID)
	if !found {
		secret = unknownAgentSecret
	}

	nonce, err := NewNonce()
	if err != nil {
		conn.reject("server nonce failure")
		return nil, nil, err
	}

	if err := conn.send(FrameChallenge, Challenge{Nonce: nonce}); err != nil {
		return nil, nil, err
	}

	frame, err = conn.readFrame(FrameProof)
	if err != nil {
		conn.reject("expected PROOF")
		return nil, nil, err
	}

	var proof Proof
	if err := json.Unmarshal(frame.Payload, &proof); err != nil {
		conn.reject("malformed PROOF")
		return nil, nil, err
	}

	if !VerifyProof(AuthContext, []byte(secret), nonce, proof.Proof) || !found {
		conn.reject("agent not registered or invalid key")
		return nil, nil, fmt.Errorf("ufp: agent %s rejected", hello.AgentID)
	}

	welcome := Welcome{AgentID: hello.AgentID, Name: name, ServerVersion: ProtocolName}
	if err := conn.send(FrameWelcome, welcome); err != nil {
		return nil, nil, err
	}

	return conn, &Identity{
		AgentID:  hello.AgentID,
		Name:     name,
		Hostname: hello.Hostname,
		Version:  hello.Version,
		Platform: hello.Platform,
		Roles:    roles,
	}, nil
}

func Dial(netConn net.Conn, hello Hello, secret string) (*Conn, *Welcome, error) {
	conn := NewConn(netConn)

	if err := conn.send(FrameHello, hello); err != nil {
		return nil, nil, err
	}

	frame, err := conn.readFrame(FrameChallenge)
	if err != nil {
		return nil, nil, err
	}

	var challenge Challenge
	if err := json.Unmarshal(frame.Payload, &challenge); err != nil {
		return nil, nil, err
	}

	proof := ComputeProof(AuthContext, []byte(secret), challenge.Nonce)
	if err := conn.send(FrameProof, Proof{Proof: proof}); err != nil {
		return nil, nil, err
	}

	frame, err = conn.readFrame(FrameWelcome)
	if err != nil {
		return nil, nil, err
	}

	var welcome Welcome
	if err := json.Unmarshal(frame.Payload, &welcome); err != nil {
		return nil, nil, err
	}

	return conn, &welcome, nil
}

func (c *Conn) readFrame(want FrameType) (*Frame, error) {
	frame, err := c.reader.ReadWithTimeout(HandshakeTimeout)
	if err != nil {
		return nil, err
	}

	if frame.Type == FrameReject {
		var reject Reject
		if json.Unmarshal(frame.Payload, &reject) == nil && reject.Reason != "" {
			return nil, fmt.Errorf("ufp: rejected by peer: %s", reject.Reason)
		}
		return nil, fmt.Errorf("ufp: rejected by peer")
	}

	if frame.Type != want {
		return nil, fmt.Errorf("ufp: expected %s, got %s", want, frame.Type)
	}

	return frame, nil
}

func (c *Conn) reject(reason string) {
	c.send(FrameReject, Reject{Reason: reason})
	c.Close()
}

func validRoles(declared []Role) []Role {
	roles := make([]Role, 0, len(declared))
	for _, role := range declared {
		if role.Valid() && !HasRole(roles, role) {
			roles = append(roles, role)
		}
	}
	return roles
}
