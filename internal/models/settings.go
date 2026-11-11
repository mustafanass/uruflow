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

package models

import (
	"net"
	"net/url"
	"strconv"
	"time"
)

const (
	SettingWebhook  = "webhook"
	SettingRegistry = "registry"
	SettingAgents   = "agents"

	DefaultRetainReleases = 10
	MaxRetainReleases     = 1000

	WebhookSecretOverlap = 24 * time.Hour

	DeliveryStarted = "started"
	DeliveryQueued  = "queued"
	DeliveryIgnored = "ignored"
	DeliveryFailed  = "failed"
)

type Setting struct {
	Name      string
	Value     string
	Revision  int64
	UpdatedAt time.Time
}

type WebhookSettings struct {
	Enabled        bool       `json:"enabled"`
	Host           string     `json:"host,omitempty"`
	Port           int        `json:"port"`
	Path           string     `json:"path"`
	TLS            bool       `json:"tls"`
	Secret         []byte     `json:"secret"`
	SecretHint     string     `json:"secret_hint"`
	SecretAt       time.Time  `json:"secret_at"`
	Previous       []byte     `json:"previous,omitempty"`
	PreviousUntil  *time.Time `json:"previous_until,omitempty"`
	Certificate    string     `json:"certificate,omitempty"`
	CertificateKey []byte     `json:"certificate_key,omitempty"`
}

type RegistrySettings struct {
	RetainReleases int       `json:"retain_releases"`
	Password       []byte    `json:"password"`
	PasswordAt     time.Time `json:"password_at"`
}

type AgentSettings struct {
	MetricsSec   int `json:"metrics_sec"`
	ReconnectSec int `json:"reconnect_sec"`
}

type WebhookDelivery struct {
	Provider   string    `json:"provider"`
	DeliveryID string    `json:"delivery_id"`
	ReceivedAt time.Time `json:"received_at"`
	Repository string    `json:"repository"`
	Branch     string    `json:"branch"`
	Commit     string    `json:"commit"`
	Result     string    `json:"result"`
	Detail     string    `json:"detail"`
}

func SecretHint(secret string) string {
	if len(secret) < 4 {
		return ""
	}
	return secret[len(secret)-4:]
}

func (w WebhookSettings) URL(advertise string) string {
	scheme := "https"
	if !w.TLS {
		scheme = "http"
	}
	host := net.JoinHostPort(advertise, strconv.Itoa(w.Port))
	return (&url.URL{Scheme: scheme, Host: host, Path: w.Path}).String()
}
