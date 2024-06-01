/*
Copyright 2024 Volodymyr Konstanchuk and the Componego Framework contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

// TODO: this is a temporary quick solution for code generation. This script needs to be improved.

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"text/template"

	"github.com/componego/meta-package/internal/utils"
)

// noinspection SpellCheckingInspection
const outputTemplate = `
// This code is generated automatically.
// Don't modify it manually.

//go:generate go run ./_codegen/require.go -o ./require.go -v {{ .version }}

package require

import (
    "github.com/stretchr/testify/require"
    "github.com/componego/componego/libs/vendor-proxy"
)

func InitPackage() error {
    r := vendor_proxy.Get("testify/require")
    // noinspection SpellCheckingInspection
    functions := map[string]any{
    {{- range $name := .functions }}
        "{{ $name }}": require.{{ $name -}},
    {{- end }}
    }
    for name, fn := range functions {
        if err := r.AddFunction(name, fn); err != nil {
		    return err
	    }
    }
    return nil
}
`

const emptyTemplate = `
package require

func InitPackage() error {
    return nil
}
`

func getPackagePath(packageName string) string {
	paths := strings.Split(os.Getenv("GOPATH"), ":")
	for _, path := range paths {
		path = filepath.Join(path, "pkg", "mod", packageName)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	utils.ErrorCheck(errors.New("package is not installed"))
	return ""
}

func getPublicFunctions(filename string) []string {
	content, err := os.ReadFile(filename)
	utils.ErrorCheck(err)
	parsedFile, err := parser.ParseFile(token.NewFileSet(), filepath.Base(filename), content, 0)
	utils.ErrorCheck(err)
	result := make([]string, 0, 10)
	ast.Inspect(parsedFile, func(node ast.Node) bool {
		if function, ok := node.(*ast.FuncDecl); ok && function.Name.IsExported() {
			result = append(result, function.Name.Name)
		}
		return true
	})
	sort.Strings(result)
	return result
}

func writeToFile(filename string, templateText string, vars any) {
	parsedTemplate, err := template.New("code").Parse(templateText)
	utils.ErrorCheck(err)
	outputFile, err := os.Create(filename)
	utils.ErrorCheck(err)
	defer func() {
		utils.ErrorCheck(outputFile.Close())
	}()
	var outputBuffer bytes.Buffer
	utils.ErrorCheck(parsedTemplate.Execute(&outputBuffer, vars))
	output, err := format.Source(outputBuffer.Bytes())
	utils.ErrorCheck(err)
	utils.ErrorCheck(os.WriteFile(filename, output, 0664))
}

func execCommand(command string, args ...string) {
	_, err := exec.Command(command, args...).Output()
	utils.ErrorCheck(err)
}

func main() {
	version := flag.String("v", "", "package version")
	outputFile := flag.String("o", "", "output file")
	flag.Parse()
	if *version == "" || *outputFile == "" {
		flag.Usage()
		utils.ErrorCheck(errors.New("missing params"))
	}
	outputFilename, err := filepath.Abs(*outputFile)
	utils.ErrorCheck(err)
	writeToFile(outputFilename, emptyTemplate, nil)
	execCommand("go", "mod", "tidy")
	// noinspection SpellCheckingInspection
	packageName := "github.com/stretchr/testify@v" + strings.TrimPrefix(*version, "v")
	packagePath := getPackagePath(packageName + "/require/require.go")
	publicFunctions := getPublicFunctions(packagePath)
	writeToFile(outputFilename, outputTemplate, map[string]any{
		"functions": publicFunctions,
		"version":   *version,
	})
	execCommand("go", "get", packageName)
	execCommand("go", "mod", "tidy")
	fmt.Println("done. Please check", outputFilename)
}
