// demo-bundles exports isolated local-demo variants using Yotta's canonical source contract.
package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/yottaapp/yotta/internal/workflow/schema"
	"io"
	"log"
	"os"
	"path/filepath"
)

func main() {
	input := flag.String("template", "", "portable workflow bundle template")
	output := flag.String("output", "", "output directory")
	flag.Parse()
	if *input == "" || *output == "" {
		log.Fatal("template and output are required")
	}
	reader, err := zip.OpenReader(*input)
	if err != nil {
		log.Fatal(err)
	}
	defer reader.Close()
	files := map[string][]byte{}
	for _, file := range reader.File {
		r, err := file.Open()
		if err != nil {
			log.Fatal(err)
		}
		raw, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			log.Fatal(err)
		}
		files[file.Name] = raw
	}
	if err = os.MkdirAll(*output, 0755); err != nil {
		log.Fatal(err)
	}
	for i := 1; i <= 24; i++ {
		var source, manifest map[string]any
		if err = json.Unmarshal(files["workflow.json"], &source); err != nil {
			log.Fatal(err)
		}
		if err = json.Unmarshal(files["yotta-workflow-bundle.json"], &manifest); err != nil {
			log.Fatal(err)
		}
		id := fmt.Sprintf("local_demo_%02d", i)
		workflow, ok := source["workflow"].(map[string]any)
		if !ok {
			log.Fatal("template workflow missing")
		}
		workflow["id"] = id
		workflow["name"] = fmt.Sprintf("本地演示 · 面板练习 %02d", i)
		raw, err := json.Marshal(source)
		if err != nil {
			log.Fatal(err)
		}
		_, canonical, hash, diagnostics, err := schema.CanonicalSource(raw)
		if err != nil || len(diagnostics) > 0 {
			log.Fatalf("canonical source: %v %v", err, diagnostics)
		}
		manifest["workflowId"] = id
		manifest["sourceHash"] = hash
		manifestBytes, err := json.Marshal(manifest)
		if err != nil {
			log.Fatal(err)
		}
		var buffer bytes.Buffer
		writer := zip.NewWriter(&buffer)
		for _, file := range reader.File {
			raw := files[file.Name]
			if file.Name == "workflow.json" {
				raw = canonical
			}
			if file.Name == "yotta-workflow-bundle.json" {
				raw = manifestBytes
			}
			out, err := writer.Create(file.Name)
			if err != nil {
				log.Fatal(err)
			}
			if _, err = out.Write(raw); err != nil {
				log.Fatal(err)
			}
		}
		if err = writer.Close(); err != nil {
			log.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(*output, id+".yotta-workflow"), buffer.Bytes(), 0644); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("Exported 24 local demo workflow bundles")
}
