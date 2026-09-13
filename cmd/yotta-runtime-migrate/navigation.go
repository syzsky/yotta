package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/yottaapp/yotta/internal/artifact"
	"github.com/yottaapp/yotta/internal/nodes"
	"github.com/yottaapp/yotta/internal/storage"
	"github.com/yottaapp/yotta/internal/storage/catalog"
	"github.com/yottaapp/yotta/internal/workflow/schema"
)

// This explicit local conversion supports terminal Move v1 nodes. More complex
// outgoing branches need author review; the desktop has no legacy adapter.
func migrateNavigation(raw []byte, b nodes.Builtins) ([]byte, error) {
	if _, diagnostics := schema.ParseSource(raw); len(diagnostics) != 0 {
		return nil, fmt.Errorf("invalid source: %v", diagnostics)
	}
	var source map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&source); err != nil {
		return nil, err
	}
	used := map[string]bool{}
	for _, v := range source["variables"].([]any) {
		used[v.(map[string]any)["name"].(string)] = true
	}
	for _, g := range source["graphs"].([]any) {
		for _, n := range g.(map[string]any)["nodes"].([]any) {
			used[n.(map[string]any)["id"].(string)] = true
		}
	}
	value := func(v any) map[string]any { return map[string]any{"kind": "value", "value": v} }
	changed := false
	for _, g := range source["graphs"].([]any) {
		graph := g.(map[string]any)
		for _, n := range append([]any(nil), graph["nodes"].([]any)...) {
			node := n.(map[string]any)
			ref := node["nodeRef"].(map[string]any)
			if ref["nodeTypeId"] != nodes.MoveCharacterNodeID || ref["version"] != "1.0.0" {
				continue
			}
			id := node["id"].(string)
			for _, e := range graph["edges"].([]any) {
				if e.(map[string]any)["from"].(map[string]any)["nodeId"] == id {
					return nil, fmt.Errorf("move %s has outgoing branches; reconnect its position source explicitly", id)
				}
			}
			prefix := ""
			for seq := 1; ; seq++ {
				prefix = fmt.Sprintf("position-%d", seq)
				collision := false
				for _, suffix := range []string{"", "-monitor", "-get", "-parse", "-write"} {
					collision = collision || used[prefix+suffix]
				}
				if !collision {
					break
				}
			}
			for _, suffix := range []string{"", "-monitor", "-get", "-parse", "-write"} {
				used[prefix+suffix] = true
			}
			old := node["config"].(map[string]any)
			parse := map[string]any{}
			for _, k := range []string{"axisHeading", "axisSign", "xField", "yField", "headingField", "validField", "timeField"} {
				if v, ok := old[k]; ok {
					parse[k] = v
				}
			}
			config := map[string]any{"position-variable": prefix}
			for _, k := range []string{"slot", "forwardKey", "turnSign"} {
				if v, ok := old[k]; ok {
					config[k] = v
				}
			}
			node["config"] = config
			def, _ := b.Definition(nodes.MoveCharacterNodeID)
			node["nodeRef"] = def.Contract.NodeRef()
			bindings := node["bindings"].(map[string]any)
			delete(bindings, "pulse")
			bindings["interval"], bindings["slow-distance"] = value(100), map[string]any{"kind": "default"}
			makeNode := func(suffix, kind string, cfg, bind map[string]any, x, y int) map[string]any {
				def, _ := b.Definition(kind)
				for _, p := range def.Contract.Machine().Ports.DataInputs {
					if _, ok := bind[p.ID]; !ok && p.Default != nil {
						bind[p.ID] = map[string]any{"kind": "default"}
					}
				}
				return map[string]any{"id": prefix + suffix, "nodeRef": def.Contract.NodeRef(), "config": cfg, "bindings": bind, "position": map[string]int{"x": x, "y": y}}
			}
			slot := any("position")
			if v, ok := old["source"]; ok {
				slot = v
			}
			path := any("/v1/position")
			if v, ok := old["path"]; ok {
				path = v
			}
			graph["nodes"] = append(graph["nodes"].([]any),
				makeNode("-monitor", nodes.MonitorNodeID, map[string]any{}, map[string]any{"interval-milliseconds": value(50)}, 160, 420),
				makeNode("-get", nodes.HTTPGetNodeID, map[string]any{"slot": slot}, map[string]any{"path": value(path)}, 480, 600),
				makeNode("-parse", nodes.ParseWorldPositionNodeID, parse, map[string]any{}, 800, 600),
				makeNode("-write", nodes.StateWriteNodeID, map[string]any{"variable": prefix}, map[string]any{}, 1120, 600))
			for _, e := range graph["edges"].([]any) {
				to := e.(map[string]any)["to"].(map[string]any)
				if to["nodeId"] == id && to["portId"] == "in" {
					to["nodeId"] = prefix + "-monitor"
				}
			}
			edge := func(channel, from, out, to, in string) map[string]any {
				return map[string]any{"channel": channel, "from": map[string]string{"nodeId": from, "portId": out}, "to": map[string]string{"nodeId": to, "portId": in}}
			}
			graph["edges"] = append(graph["edges"].([]any), edge("exec", prefix+"-monitor", "main", id, "in"), edge("exec", prefix+"-monitor", "tick", prefix+"-get", "in"), edge("exec", prefix+"-get", "completed", prefix+"-parse", "in"), edge("data", prefix+"-get", "body", prefix+"-parse", "source"), edge("exec", prefix+"-parse", "done", prefix+"-write", "in"), edge("data", prefix+"-parse", "position", prefix+"-write", "value"))
			source["variables"] = append(source["variables"].([]any), map[string]any{"name": prefix, "type": map[string]any{"kind": "ref", "ref": b.WorldPositionType.TypeRef()}, "default": b.WorldPositionType.Authoring().Examples[0]})
			changed = true
		}
	}
	if !changed {
		return raw, nil
	}
	converted, err := artifact.Marshal(source)
	if err != nil {
		return nil, err
	}
	_, canonical, _, diagnostics, err := schema.CanonicalSource(converted)
	if err != nil {
		return nil, err
	}
	if len(diagnostics) != 0 {
		return nil, fmt.Errorf("converted source: %v", diagnostics)
	}
	return canonical, nil
}

func migrateNavigationProfile(root string, write bool) error {
	ctx := context.Background()
	roots, err := storage.Resolve(root)
	if err != nil {
		return err
	}
	if _, err := os.Stat(roots.ManifestFile()); err != nil {
		return err
	}
	profile, err := storage.Open(ctx, storage.OpenOptions{Root: roots.Root})
	if err != nil {
		return err
	}
	defer profile.Close()
	foundation, err := catalog.Open(ctx, profile.Roots)
	if err != nil {
		return err
	}
	defer foundation.Close()
	records, err := foundation.Workflows().List(ctx)
	if err != nil {
		return err
	}
	b, err := nodes.Build()
	if err != nil {
		return err
	}
	type change struct {
		id             string
		previous, next artifact.Digest
		raw            []byte
	}
	var changes []change
	for _, record := range records {
		raw, err := migrateNavigation(record.Artifact, b)
		if err != nil {
			return fmt.Errorf("workflow %s: %w", record.WorkflowID, err)
		}
		if bytes.Equal(raw, record.Artifact) {
			continue
		}
		_, previous, err := schema.CanonicalSourceArtifact(record.Artifact)
		if err != nil || previous != record.Hash {
			return errors.New("stored workflow hash mismatch")
		}
		_, next, err := schema.CanonicalSourceArtifact(raw)
		if err != nil {
			return err
		}
		changes = append(changes, change{record.WorkflowID, previous, next, raw})
	}
	if len(changes) == 0 {
		fmt.Println("navigation sources already current")
		return nil
	}
	if !write {
		fmt.Printf("validated %d navigation sources; use --write to back up and convert\n", len(changes))
		return nil
	}
	backup := filepath.Join(roots.Backups, "before-navigation-v2-"+time.Now().UTC().Format("20060102T150405.000000000Z"))
	if _, err := foundation.Backup(ctx, backup); err != nil {
		return err
	}
	path := filepath.ToSlash(filepath.Join(roots.Catalog, catalog.ContentFilename))
	if len(path) > 1 && path[1] == ':' {
		path = "/" + path
	}
	uri := url.URL{Scheme: "file", Path: path}
	db, err := sql.Open("sqlite", uri.String()+"?mode=rw")
	if err != nil {
		return err
	}
	defer db.Close()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Resource references, edit revisions, timestamps and all unrelated rows stay
	// intact; this conversion only expands terminal navigation into explicit nodes.
	for _, c := range changes {
		result, err := tx.ExecContext(ctx, "UPDATE workflow_sources SET source_hash=?, artifact=? WHERE workflow_id=? AND source_hash=?", c.next.String(), c.raw, c.id, c.previous.String())
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil || n != 1 {
			return errors.New("workflow changed during migration")
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	fmt.Printf("converted %d navigation sources; backup: %s\n", len(changes), backup)
	return nil
}
