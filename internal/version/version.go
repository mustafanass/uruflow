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

package version

import (
	"strconv"
	"strings"
)

const Current = "3.0.6"

func AtLeast(current, minimum string) bool {
	have, ok := parse(current)
	if !ok {
		return false
	}
	want, ok := parse(minimum)
	if !ok {
		return false
	}
	for index := range have {
		if have[index] != want[index] {
			return have[index] > want[index]
		}
	}
	return true
}

func parse(value string) ([3]int, bool) {
	var parts [3]int
	fields := strings.SplitN(strings.TrimPrefix(value, "v"), ".", 3)
	if len(fields) != 3 {
		return parts, false
	}
	for index, field := range fields {
		number, err := strconv.Atoi(field)
		if err != nil || number < 0 {
			return parts, false
		}
		parts[index] = number
	}
	return parts, true
}
