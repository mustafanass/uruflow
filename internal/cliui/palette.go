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

package cliui

const (
	BrandNavy  = "#0B2444"
	BrandBlue  = "#1D3E6B"
	BrandGold  = "#EDC35D"
	BrandIvory = "#F5F1E9"
	BrandSteel = "#9FB3D1"

	ANSIAccent      = "38;2;237;195;93"
	ANSIAccentBold  = "1;38;2;237;195;93"
	ANSIBorder      = "38;2;159;179;209"
	ANSIMuted       = "38;2;159;179;209"
	ANSIText        = "38;2;245;241;233"
	ANSITextBold    = "1;38;2;245;241;233"
	ANSISelected    = "1;38;2;245;241;233;48;2;29;62;107"
	ANSISuccess     = "38;2;110;231;183"
	ANSIError       = "38;2;251;113;133"
	ANSIWarning     = ANSIAccent
	ANSIInformation = ANSIBorder
	PlainWordmark   = "uruflow"
)

func Paint(color bool, code, value string) string {
	if !color {
		return value
	}
	return "\x1b[" + code + "m" + value + "\x1b[0m"
}

func Wordmark(color bool) string {
	return Paint(color, ANSITextBold, "uru") + Paint(color, ANSIAccentBold, "flow")
}
