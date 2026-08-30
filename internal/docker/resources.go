/*
 * Copyright (C) 2026 Mustafa Naseer (Mustafa Gaeed)
 *
 * This file is part of uruflow.
 *
 * uruflow is free software: you can redistribute it and/or modify
 * it under the terms of the MIT License as described in the LICENSE file.
 */

package docker

import (
	"context"
	"errors"
	"fmt"
	"net/url"
)

type NetworkResource struct {
	Name       string
	Driver     string
	External   bool
	Internal   bool
	Attachable bool
	Options    map[string]string
	Labels     map[string]string
}

type VolumeResource struct {
	Name     string
	Driver   string
	External bool
	Options  map[string]string
	Labels   map[string]string
}

func (c *Client) EnsureNetwork(ctx context.Context, network NetworkResource) error {
	if network.Name == "" {
		return fmt.Errorf("network name is required")
	}
	if err := c.get(ctx, "/networks/"+url.PathEscape(network.Name), nil); err == nil {
		return nil
	} else if !errors.Is(err, ErrNotFound) {
		return err
	} else if network.External {
		return fmt.Errorf("external network %s does not exist", network.Name)
	}
	body := map[string]any{"Name": network.Name, "CheckDuplicate": true, "Internal": network.Internal, "Attachable": network.Attachable, "Options": network.Options, "Labels": network.Labels}
	if network.Driver != "" {
		body["Driver"] = network.Driver
	}
	return c.post(ctx, "/networks/create", body, nil)
}

func (c *Client) EnsureVolume(ctx context.Context, volume VolumeResource) error {
	if volume.Name == "" {
		return fmt.Errorf("volume name is required")
	}
	if err := c.get(ctx, "/volumes/"+url.PathEscape(volume.Name), nil); err == nil {
		return nil
	} else if !errors.Is(err, ErrNotFound) {
		return err
	} else if volume.External {
		return fmt.Errorf("external volume %s does not exist", volume.Name)
	}
	body := map[string]any{"Name": volume.Name, "DriverOpts": volume.Options, "Labels": volume.Labels}
	if volume.Driver != "" {
		body["Driver"] = volume.Driver
	}
	return c.post(ctx, "/volumes/create", body, nil)
}
