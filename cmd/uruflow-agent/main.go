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

package main

import (
	"fmt"
	"os"

	"github.com/urustack/uruflow/internal/cli"
)

func main() {
	if err := cli.ExecuteAgent(); err != nil {
		fmt.Fprintf(os.Stderr, "uruflow-agent: %v\n", err)
		os.Exit(1)
	}
}
