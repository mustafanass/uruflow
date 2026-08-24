//go:build !unix

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

import "errors"

type Server struct{}

func Listen(string, Handler) (*Server, error) {
	return nil, errors.New("the uruflow server console requires a Unix host")
}

func (*Server) Close() error { return nil }

func Attach(string) error {
	return errors.New("the uruflow server console requires a Unix host")
}
