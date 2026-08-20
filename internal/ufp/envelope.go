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
	"errors"
)

var ErrEmptyPayload = errors.New("ufp: empty payload")

type Request struct {
	ID      uint32          `json:"id"`
	Method  string          `json:"method"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type Response struct {
	ID      uint32          `json:"id"`
	OK      bool            `json:"ok"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

type Event struct {
	Topic   string          `json:"topic"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func NewRequest(id uint32, method string, data any) (*Request, error) {
	payload, err := encode(data)
	if err != nil {
		return nil, err
	}
	return &Request{ID: id, Method: method, Payload: payload}, nil
}

func NewResponse(id uint32, data any) (*Response, error) {
	payload, err := encode(data)
	if err != nil {
		return nil, err
	}
	return &Response{ID: id, OK: true, Payload: payload}, nil
}

func NewResponseError(id uint32, message string) *Response {
	payload, _ := encode(ErrorPayload{Message: message})
	return &Response{ID: id, OK: false, Payload: payload}
}

func NewEvent(topic string, data any) (*Event, error) {
	payload, err := encode(data)
	if err != nil {
		return nil, err
	}
	return &Event{Topic: topic, Payload: payload}, nil
}

func (r *Request) Decode(v any) error  { return decode(r.Payload, v) }
func (r *Response) Decode(v any) error { return decode(r.Payload, v) }
func (e *Event) Decode(v any) error    { return decode(e.Payload, v) }

func (r *Response) ErrorMessage() string {
	var payload ErrorPayload
	if err := decode(r.Payload, &payload); err != nil || payload.Message == "" {
		return "unknown error"
	}
	return payload.Message
}

func encode(data any) (json.RawMessage, error) {
	if data == nil {
		return nil, nil
	}
	return json.Marshal(data)
}

func decode(payload json.RawMessage, v any) error {
	if len(payload) == 0 {
		return ErrEmptyPayload
	}
	return json.Unmarshal(payload, v)
}
