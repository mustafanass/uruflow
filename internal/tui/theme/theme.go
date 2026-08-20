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

package theme

import "github.com/charmbracelet/lipgloss"

var (
	Primary   = lipgloss.AdaptiveColor{Light: "#0D9488", Dark: "#2DD4BF"}
	Accent    = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#F5A524"}
	Success   = lipgloss.AdaptiveColor{Light: "#15803D", Dark: "#4ADE80"}
	Warning   = lipgloss.AdaptiveColor{Light: "#B45309", Dark: "#FBBF24"}
	Danger    = lipgloss.AdaptiveColor{Light: "#B91C1C", Dark: "#F87171"}
	Info      = lipgloss.AdaptiveColor{Light: "#1D4ED8", Dark: "#60A5FA"}
	Text      = lipgloss.AdaptiveColor{Light: "#1F2430", Dark: "#E4E9F2"}
	Muted     = lipgloss.AdaptiveColor{Light: "#5B6472", Dark: "#8A94A8"}
	Dim       = lipgloss.AdaptiveColor{Light: "#8C93A1", Dark: "#5A6478"}
	Border    = lipgloss.AdaptiveColor{Light: "#D2D7E0", Dark: "#2B3446"}
	BorderLit = lipgloss.AdaptiveColor{Light: "#0D9488", Dark: "#33405A"}
)

var (
	Base    = lipgloss.NewStyle().Foreground(Text)
	Title   = lipgloss.NewStyle().Foreground(Text).Bold(true)
	Brand   = lipgloss.NewStyle().Foreground(Primary).Bold(true)
	Heading = lipgloss.NewStyle().Foreground(Muted).Bold(true)
	Body    = lipgloss.NewStyle().Foreground(Text)
	Faint   = lipgloss.NewStyle().Foreground(Muted)
	Ghost   = lipgloss.NewStyle().Foreground(Dim)

	Good = lipgloss.NewStyle().Foreground(Success)
	Warn = lipgloss.NewStyle().Foreground(Warning)
	Bad  = lipgloss.NewStyle().Foreground(Danger)
	Note = lipgloss.NewStyle().Foreground(Info)
	Mark = lipgloss.NewStyle().Foreground(Accent)
	Lead = lipgloss.NewStyle().Foreground(Primary)

	Selected = lipgloss.NewStyle().Foreground(Primary).Bold(true)
	Column   = lipgloss.NewStyle().Foreground(Dim).Bold(true)
	Rule     = lipgloss.NewStyle().Foreground(Border)

	Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Border).
		Padding(0, 1)

	PanelActive = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Primary).
			Padding(0, 1)

	TabActive = lipgloss.NewStyle().Foreground(Primary).Bold(true).Padding(0, 1)
	TabIdle   = lipgloss.NewStyle().Foreground(Dim).Padding(0, 1)

	Key  = lipgloss.NewStyle().Foreground(Primary).Bold(true)
	Hint = lipgloss.NewStyle().Foreground(Dim)
)
