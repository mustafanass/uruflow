/*
 * Copyright (C) 2026 Mustafa Naseer (Mustafa Gaeed)
 *
 * This file is part of uruflow.
 *
 * uruflow is free software: you can redistribute it and/or modify
 * it under the terms of the MIT License as described in the
 * LICENSE file distributed with this project.
 */

package console

import (
	"context"
	"os"
)

type Size struct {
	Width  int
	Height int
}

type Handler func(context.Context, *os.File, []string, <-chan Size) error
