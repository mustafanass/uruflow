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

type FrameType byte

const (
	FrameHello     FrameType = 0x01
	FrameChallenge FrameType = 0x02
	FrameProof     FrameType = 0x03
	FrameWelcome   FrameType = 0x04
	FrameReject    FrameType = 0x05

	FrameRequest  FrameType = 0x10
	FrameResponse FrameType = 0x11
	FrameEvent    FrameType = 0x12

	FramePing    FrameType = 0x20
	FramePong    FrameType = 0x21
	FrameGoodbye FrameType = 0x22
)

var frameNames = map[FrameType]string{
	FrameHello:     "HELLO",
	FrameChallenge: "CHALLENGE",
	FrameProof:     "PROOF",
	FrameWelcome:   "WELCOME",
	FrameReject:    "REJECT",
	FrameRequest:   "REQUEST",
	FrameResponse:  "RESPONSE",
	FrameEvent:     "EVENT",
	FramePing:      "PING",
	FramePong:      "PONG",
	FrameGoodbye:   "GOODBYE",
}

func (t FrameType) String() string {
	if name, ok := frameNames[t]; ok {
		return name
	}
	return "UNKNOWN"
}
