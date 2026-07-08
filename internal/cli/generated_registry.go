// Copyright 2026 hello-keith. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"

	"github.com/spf13/cobra"
)

type generatedEndpointRegistration struct {
	endpoint   string
	newCommand func(*rootFlags) *cobra.Command
}

var generatedEndpointRegistrations []generatedEndpointRegistration

func registerGeneratedEndpoint(endpoint string, newCommand func(*rootFlags) *cobra.Command) {
	generatedEndpointRegistrations = append(generatedEndpointRegistrations, generatedEndpointRegistration{
		endpoint:   endpoint,
		newCommand: newCommand,
	})
}

func installGeneratedEndpoints(root *cobra.Command, flags *rootFlags) {
	for _, registration := range generatedEndpointRegistrations {
		installGeneratedEndpoint(root, flags, registration)
	}
}

func installGeneratedEndpoint(root *cobra.Command, flags *rootFlags, registration generatedEndpointRegistration) {
	if root == nil || registration.newCommand == nil {
		return
	}
	segments := endpointSegments(registration.endpoint)
	cmd := registration.newCommand(flags)
	if len(segments) == 0 {
		root.AddCommand(cmd)
		return
	}
	parent := root
	for _, segment := range segments[:len(segments)-1] {
		child := findChildCommand(parent, segment)
		if child == nil {
			child = newGeneratedParentCommand(segment, flags)
			parent.AddCommand(child)
		}
		parent = child
	}
	parent.AddCommand(cmd)
}

func endpointSegments(endpoint string) []string {
	parts := strings.Split(endpoint, ".")
	segments := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			segments = append(segments, part)
		}
	}
	return segments
}

func findChildCommand(parent *cobra.Command, name string) *cobra.Command {
	for _, child := range parent.Commands() {
		if child.Name() == name {
			return child
		}
	}
	return nil
}

func newGeneratedParentCommand(name string, flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:    name,
		Short:  "Manage " + strings.ReplaceAll(name, "-", " "),
		Hidden: true,
		RunE:   parentNoSubcommandRunE(flags),
	}
}
