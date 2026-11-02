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

package cli

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/mustafanass/uruflow/internal/config"
	"github.com/mustafanass/uruflow/internal/storage/sqlite"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/bcrypt"
)

func adminCommand() *cobra.Command {
	command := &cobra.Command{Use: "admin", Short: "Recover local administrator access"}
	var fromStdin bool
	reset := &cobra.Command{Use: "reset-password USERNAME", Short: "Reset an administrator password and revoke their sessions", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(configPath())
		if err != nil {
			return err
		}
		if _, err = os.Stat(cfg.DatabasePath()); err != nil {
			return fmt.Errorf("open existing server database: %w", err)
		}
		password := ""
		if fromStdin {
			data, err := io.ReadAll(io.LimitReader(cmd.InOrStdin(), 75))
			if err != nil {
				return err
			}
			password = strings.TrimSuffix(strings.TrimSuffix(string(data), "\n"), "\r")
			if !utf8.ValidString(password) || utf8.RuneCountInString(password) < 15 || len(password) > 72 || strings.ContainsAny(password, "\r\n\x00") || strings.EqualFold(password, args[0]) {
				return fmt.Errorf("password must contain at least 15 characters, at most 72 bytes, and no line breaks or NUL bytes")
			}
		} else {
			var data [24]byte
			if _, err = rand.Read(data[:]); err != nil {
				return err
			}
			password = base64.RawURLEncoding.EncodeToString(data[:])
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		store, err := sqlite.New(cfg.DatabasePath())
		if err != nil {
			return err
		}
		defer store.Close()
		if err = store.ResetConsoleAdministrator(args[0], hash); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Password reset for %s. All sessions were revoked.\n", args[0])
		if !fromStdin {
			fmt.Fprintf(cmd.OutOrStdout(), "New password: %s\n", password)
		}
		return nil
	}}
	reset.Flags().BoolVar(&fromStdin, "password-stdin", false, "Read the replacement password from standard input instead of generating one")
	command.AddCommand(reset)
	return command
}
