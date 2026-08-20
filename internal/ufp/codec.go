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
	"bufio"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"sync"
	"time"
)

const (
	MagicHigh      byte = 0x55
	MagicLow       byte = 0x46
	Version        byte = 0x03
	HeaderSize          = 8
	MaxPayloadSize      = 16 * 1024 * 1024
)

var (
	ErrInvalidMagic    = errors.New("ufp: invalid magic bytes")
	ErrInvalidVersion  = errors.New("ufp: unsupported protocol version")
	ErrPayloadTooLarge = errors.New("ufp: payload exceeds maximum size")
	ErrShortHeader     = errors.New("ufp: short frame header")
)

type Frame struct {
	Type    FrameType
	Payload []byte
}

func EncodeHeader(frameType FrameType, payloadLen uint32) []byte {
	header := make([]byte, HeaderSize)
	header[0] = MagicHigh
	header[1] = MagicLow
	header[2] = Version
	header[3] = byte(frameType)
	binary.BigEndian.PutUint32(header[4:], payloadLen)
	return header
}

func DecodeHeader(header []byte) (FrameType, uint32, error) {
	if len(header) < HeaderSize {
		return 0, 0, ErrShortHeader
	}
	if header[0] != MagicHigh || header[1] != MagicLow {
		return 0, 0, ErrInvalidMagic
	}
	if header[2] != Version {
		return 0, 0, ErrInvalidVersion
	}
	payloadLen := binary.BigEndian.Uint32(header[4:])
	if payloadLen > MaxPayloadSize {
		return 0, 0, ErrPayloadTooLarge
	}
	return FrameType(header[3]), payloadLen, nil
}

type Reader struct {
	conn   net.Conn
	buffer *bufio.Reader
}

func NewReader(conn net.Conn) *Reader {
	return &Reader{conn: conn, buffer: bufio.NewReader(conn)}
}

func (r *Reader) Read() (*Frame, error) {
	header := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r.buffer, header); err != nil {
		return nil, err
	}

	frameType, payloadLen, err := DecodeHeader(header)
	if err != nil {
		return nil, err
	}

	var payload []byte
	if payloadLen > 0 {
		payload = make([]byte, payloadLen)
		if _, err := io.ReadFull(r.buffer, payload); err != nil {
			return nil, err
		}
	}

	return &Frame{Type: frameType, Payload: payload}, nil
}

func (r *Reader) ReadWithTimeout(timeout time.Duration) (*Frame, error) {
	if err := r.conn.SetReadDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	defer r.conn.SetReadDeadline(time.Time{})
	return r.Read()
}

type Writer struct {
	conn   net.Conn
	buffer *bufio.Writer
	mu     sync.Mutex
}

func NewWriter(conn net.Conn) *Writer {
	return &Writer{conn: conn, buffer: bufio.NewWriter(conn)}
}

func (w *Writer) Write(frameType FrameType, payload []byte) error {
	if len(payload) > MaxPayloadSize {
		return ErrPayloadTooLarge
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.buffer.Write(EncodeHeader(frameType, uint32(len(payload)))); err != nil {
		return err
	}
	if len(payload) > 0 {
		if _, err := w.buffer.Write(payload); err != nil {
			return err
		}
	}
	return w.buffer.Flush()
}

func (w *Writer) WriteWithTimeout(frameType FrameType, payload []byte, timeout time.Duration) error {
	if err := w.conn.SetWriteDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	defer w.conn.SetWriteDeadline(time.Time{})
	return w.Write(frameType, payload)
}
