/*
 * Copyright (C) 2026 Mustafa Naseer (Mustafa Gaeed)
 * This file is part of uruflow and is licensed under the MIT License.
 */

package projects

import "testing"

func TestInterpolateComposeFormsAndPreservesSecrets(t *testing.T) {
	variables := map[string]string{"TAG": "1.5.5", "PASSWORD": "${secret:db_password}"}
	got, err := Interpolate("image:${TAG:-latest} ${MISSING:-fallback} ${PASSWORD} $$HOME", variables)
	if err != nil {
		t.Fatal(err)
	}
	want := "image:1.5.5 fallback ${secret:db_password} $HOME"
	if got != want {
		t.Fatalf("interpolated = %q, want %q", got, want)
	}
	if _, err := Interpolate("${REQUIRED:?required value is absent}", variables); err == nil {
		t.Fatal("required variable was accepted")
	}
}

func TestResolveVariablesRejectsCycles(t *testing.T) {
	if _, err := resolveVariables(map[string]string{"A": "${B}", "B": "${A}"}); err == nil {
		t.Fatal("cycle was accepted")
	}
	resolved, err := resolveVariables(map[string]string{"A": "${B}", "B": "value", "SECRET": "${secret:key}"})
	if err != nil {
		t.Fatal(err)
	}
	if resolved["A"] != "value" || resolved["SECRET"] != "${secret:key}" {
		t.Fatalf("resolved = %#v", resolved)
	}
}

func TestBuildServicesLoadsNativeRuntimeModel(t *testing.T) {
	services, err := buildServices(map[string]Service{
		"init": {Image: "example/init@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Mode: "job", Command: Command{Exec: []string{"migrate"}}, Networks: map[string]NetworkAttachment{"data": {Aliases: []string{"migration"}}}, Resources: Resources{Memory: "128M"}, Security: Security{NoNewPrivileges: true}, Logging: Logging{Driver: "json-file"}, Timeout: "2m"},
		"api":  {Dockerfile: "Dockerfile", DependsOn: map[string]string{"init": "completed"}, Healthcheck: &Healthcheck{Type: "command", Command: Command{Shell: "curl -f http://localhost/health"}, Interval: "5s", Timeout: "2s"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(services) != 2 || services[1].Name != "init" || services[1].Job.Timeout.String() != "2m0s" {
		t.Fatalf("services = %#v", services)
	}
	if services[0].Healthcheck == nil || services[0].Healthcheck.Type != "command" {
		t.Fatalf("healthcheck = %#v", services[0].Healthcheck)
	}
}

func TestEnvironmentValidationUsesDotEnvVariables(t *testing.T) {
	content := "branch: main\nbuilder: builder-01\nrunners: [web-01]\nresources:\n  networks:\n    edge:\n      name: ${NETWORK:?required}\nservices:\n  api:\n    image: ${IMAGE:?required}\n    networks:\n      edge: {}\n"
	variables := map[string]string{
		"NETWORK": "shared-edge",
		"IMAGE":   "example/api@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	if err := ValidateEnvironmentYAMLWithVariables(content, variables); err != nil {
		t.Fatal(err)
	}
	if err := ValidateEnvironmentYAML(content); err == nil {
		t.Fatal("required values were accepted without the environment")
	}
}
