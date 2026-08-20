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
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrClosed          = errors.New("ufp: connection closed")
	ErrUnexpectedFrame = errors.New("ufp: unexpected frame")
)

type Handler interface {
	HandleRequest(request *Request) (any, error)
	HandleEvent(event *Event) error
}

type Conn struct {
	conn    net.Conn
	reader  *Reader
	writer  *Writer
	counter atomic.Uint32
	closed  atomic.Bool
	mu      sync.Mutex
	pending map[uint32]chan *Response
}

func NewConn(conn net.Conn) *Conn {
	return &Conn{
		conn:    conn,
		reader:  NewReader(conn),
		writer:  NewWriter(conn),
		pending: make(map[uint32]chan *Response),
	}
}

func (c *Conn) RemoteAddr() string { return c.conn.RemoteAddr().String() }

func (c *Conn) Close() error {
	if c.closed.Swap(true) {
		return nil
	}
	c.mu.Lock()
	for id, waiter := range c.pending {
		close(waiter)
		delete(c.pending, id)
	}
	c.mu.Unlock()
	return c.conn.Close()
}

func (c *Conn) Closed() bool { return c.closed.Load() }

func (c *Conn) send(frameType FrameType, data any) error {
	if c.closed.Load() {
		return ErrClosed
	}
	payload, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.writer.WriteWithTimeout(frameType, payload, WriteTimeout)
}

func (c *Conn) SendFrame(frameType FrameType) error {
	if c.closed.Load() {
		return ErrClosed
	}
	return c.writer.WriteWithTimeout(frameType, nil, WriteTimeout)
}

func (c *Conn) SendEvent(topic string, data any) error {
	event, err := NewEvent(topic, data)
	if err != nil {
		return err
	}
	return c.send(FrameEvent, event)
}

func (c *Conn) SendResponse(response *Response) error {
	return c.send(FrameResponse, response)
}

func (c *Conn) SendGoodbye(reason string) error {
	return c.send(FrameGoodbye, Goodbye{Reason: reason})
}

func (c *Conn) Request(ctx context.Context, method string, data any) (*Response, error) {
	id := c.counter.Add(1)
	request, err := NewRequest(id, method, data)
	if err != nil {
		return nil, err
	}

	waiter := make(chan *Response, 1)
	c.mu.Lock()
	c.pending[id] = waiter
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	if err := c.send(FrameRequest, request); err != nil {
		return nil, err
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case response, ok := <-waiter:
		if !ok {
			return nil, ErrClosed
		}
		return response, nil
	}
}

func (c *Conn) Serve(ctx context.Context, handler Handler) error {
	defer c.Close()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		frame, err := c.reader.ReadWithTimeout(IdleTimeout)
		if err != nil {
			return err
		}

		switch frame.Type {
		case FramePing:
			if err := c.SendFrame(FramePong); err != nil {
				return err
			}

		case FramePong:

		case FrameGoodbye:
			return nil

		case FrameResponse:
			var response Response
			if err := json.Unmarshal(frame.Payload, &response); err != nil {
				continue
			}
			c.resolve(&response)

		case FrameEvent:
			var event Event
			if err := json.Unmarshal(frame.Payload, &event); err != nil {
				continue
			}
			if err := handler.HandleEvent(&event); err != nil {
				return err
			}

		case FrameRequest:
			var request Request
			if err := json.Unmarshal(frame.Payload, &request); err != nil {
				continue
			}
			go c.dispatch(handler, &request)

		default:
			return ErrUnexpectedFrame
		}
	}
}

func (c *Conn) Heartbeat(ctx context.Context) {
	ticker := time.NewTicker(PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if c.SendFrame(FramePing) != nil {
				return
			}
		}
	}
}

func (c *Conn) dispatch(handler Handler, request *Request) {
	result, err := handler.HandleRequest(request)
	if err != nil {
		c.SendResponse(NewResponseError(request.ID, err.Error()))
		return
	}

	response, err := NewResponse(request.ID, result)
	if err != nil {
		c.SendResponse(NewResponseError(request.ID, err.Error()))
		return
	}
	c.SendResponse(response)
}

func (c *Conn) resolve(response *Response) {
	c.mu.Lock()
	waiter, ok := c.pending[response.ID]
	if ok {
		delete(c.pending, response.ID)
	}
	c.mu.Unlock()

	if ok {
		waiter <- response
		close(waiter)
	}
}
