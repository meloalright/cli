// Copyright (c) 2026 Lark Technologies Pte. Ltd.
// SPDX-License-Identifier: MIT

package wiki

import (
	"context"
	"fmt"
	"strings"

	"github.com/larksuite/cli/internal/output"
	"github.com/larksuite/cli/internal/validate"
	"github.com/larksuite/cli/shortcuts/common"
)

// WikiNodeList lists child nodes in a wiki space or under a parent node.
var WikiNodeList = common.Shortcut{
	Service:     "wiki",
	Command:     "+node-list",
	Description: "List wiki nodes in a space or under a parent node",
	Risk:        "read",
	Scopes:      []string{"wiki:node:retrieve"},
	AuthTypes:   []string{"user", "bot"},
	Flags: []common.Flag{
		{Name: "space-id", Desc: "wiki space ID; use my_library for the personal document library, or +space-list to discover other space IDs", Required: true},
		{Name: "parent-node-token", Desc: "parent node token; if omitted, lists the root-level nodes of the space"},
	},
	Tips: []string{
		"Use --parent-node-token to drill into a sub-directory.",
		"Run +space-list first to discover your space IDs, including the personal document library.",
		"--space-id my_library is a per-user alias and is only valid with --as user.",
	},
	Validate: func(ctx context.Context, runtime *common.RuntimeContext) error {
		spaceID := strings.TrimSpace(runtime.Str("space-id"))
		// my_library is a per-user personal-library alias; it has no meaning
		// for a tenant_access_token (--as bot), so reject early with a clear
		// hint instead of deferring to API-time errors. Matches the contract
		// used by +node-create and +move.
		if runtime.As().IsBot() && spaceID == wikiMyLibrarySpaceID {
			return output.ErrValidation("bot identity does not support --space-id my_library; use an explicit --space-id")
		}
		if err := validateOptionalResourceName(spaceID, "--space-id"); err != nil {
			return err
		}
		return validateOptionalResourceName(strings.TrimSpace(runtime.Str("parent-node-token")), "--parent-node-token")
	},
	DryRun: func(ctx context.Context, runtime *common.RuntimeContext) *common.DryRunAPI {
		spaceID := strings.TrimSpace(runtime.Str("space-id"))
		params := map[string]interface{}{"page_size": 50}
		if pt := strings.TrimSpace(runtime.Str("parent-node-token")); pt != "" {
			params["parent_node_token"] = pt
		}
		d := common.NewDryRunAPI()
		// When the caller passes my_library, +node-list must first resolve it
		// to the real per-user space_id before listing nodes, mirroring the
		// two-step orchestration used by +node-create.
		if spaceID == wikiMyLibrarySpaceID {
			d.Desc("2-step orchestration: resolve my_library -> list nodes").
				GET("/open-apis/wiki/v2/spaces/my_library").
				Desc("[1] Resolve my_library space ID").
				GET(fmt.Sprintf("/open-apis/wiki/v2/spaces/%s/nodes", "<resolved_space_id>")).
				Desc("[2] List nodes").
				Params(params).
				Set("space_id", "<resolved_space_id>")
			return d
		}
		return d.
			GET(fmt.Sprintf("/open-apis/wiki/v2/spaces/%s/nodes", validate.EncodePathSegment(spaceID))).
			Params(params).
			Set("space_id", spaceID)
	},
	Execute: func(ctx context.Context, runtime *common.RuntimeContext) error {
		spaceID := strings.TrimSpace(runtime.Str("space-id"))
		parentNodeToken := strings.TrimSpace(runtime.Str("parent-node-token"))

		// Resolve the my_library alias to the per-user real space_id before
		// listing, so the subsequent request hits a concrete space endpoint.
		if spaceID == wikiMyLibrarySpaceID {
			resolved, err := resolveMyLibrarySpaceID(runtime)
			if err != nil {
				return err
			}
			fmt.Fprintf(runtime.IO().ErrOut, "Resolved my_library to space %s\n", common.MaskToken(resolved))
			spaceID = resolved
		}

		var nodes []map[string]interface{}
		pageToken := ""
		for {
			params := map[string]interface{}{"page_size": 50}
			if parentNodeToken != "" {
				params["parent_node_token"] = parentNodeToken
			}
			if pageToken != "" {
				params["page_token"] = pageToken
			}
			data, err := runtime.CallAPI("GET",
				fmt.Sprintf("/open-apis/wiki/v2/spaces/%s/nodes", validate.EncodePathSegment(spaceID)),
				params, nil)
			if err != nil {
				return err
			}
			items, _ := data["items"].([]interface{})
			for _, item := range items {
				if m, ok := item.(map[string]interface{}); ok {
					nodes = append(nodes, wikiNodeListItem(m))
				}
			}
			next, _ := data["page_token"].(string)
			hasMore, _ := data["has_more"].(bool)
			if !hasMore || next == "" {
				break
			}
			pageToken = next
		}
		fmt.Fprintf(runtime.IO().ErrOut, "Found %d node(s)\n", len(nodes))
		runtime.Out(map[string]interface{}{
			"nodes": nodes,
			"total": len(nodes),
		}, nil)
		return nil
	},
}

func wikiNodeListItem(m map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"space_id":          common.GetString(m, "space_id"),
		"node_token":        common.GetString(m, "node_token"),
		"obj_token":         common.GetString(m, "obj_token"),
		"obj_type":          common.GetString(m, "obj_type"),
		"parent_node_token": common.GetString(m, "parent_node_token"),
		"node_type":         common.GetString(m, "node_type"),
		"title":             common.GetString(m, "title"),
		"has_child":         common.GetBool(m, "has_child"),
	}
}
