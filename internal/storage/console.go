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

package storage

import "time"

type ConsoleUser struct {
	Projects     []string  `json:"projects"`
	Username     string    `json:"username"`
	Role         string    `json:"role"`
	PasswordHash []byte    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}
type AuditEntry struct {
	ID        int64     `json:"id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Resource  string    `json:"resource"`
	Outcome   string    `json:"outcome"`
	CreatedAt time.Time `json:"created_at"`
}
type ConsoleStore interface {
	ListConsoleSessions() ([]ConsoleSession, error)
	SaveConsoleSession(ConsoleSession) error
	DeleteConsoleSession([32]byte) error
	DeleteConsoleUserSessions(string) error
	ListConsoleUsers() ([]ConsoleUser, error)
	GetConsoleUser(string) (*ConsoleUser, error)
	BootstrapConsoleUser(*ConsoleUser) (bool, error)
	CreateConsoleUser(*ConsoleUser) error
	DeleteConsoleUser(string) error
	ChangeConsolePassword(string, []byte, []byte) error
	ChangeConsoleRole(string, string) error
	ChangeConsoleProjects(string, []string) error
	AppendAudit(*AuditEntry) error
	ListAudit(int) ([]AuditEntry, error)
}

type ConsoleSession struct {
	Hash        [32]byte
	Username    string
	CSRF        string
	Expires     time.Time
	LastActive  time.Time
	Credentials [32]byte
}

func (u ConsoleUser) AllProjects() bool {
	if u.Role == "admin" {
		return true
	}
	for _, p := range u.Projects {
		if p == "*" {
			return true
		}
	}
	return false
}
func (u ConsoleUser) CanProject(name string) bool {
	if u.AllProjects() {
		return true
	}
	for _, p := range u.Projects {
		if p == name {
			return true
		}
	}
	return false
}
