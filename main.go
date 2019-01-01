// go-include creates a .go file containing the text or binary (base64)
// content of files specified by the glob pattern.
package main

import (
	"bytes"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"
)

// TODO: option for export/not variable names
// TODO: option for converting path separators to underscores (ex res_gopher)
// TODO: option for adding extention (ex gopher_png)
// TODO: generate convenience functions for converting some binary data to usable types
// ✓TODO: change 'in' flag to be the rest of the cmd line args
// ✓TODO: make default output stdout, specified by "-"
// ✓TODO: make default input stdin when no globs are given on the cmd line

func main() {

	// process flags
	// glob := flag.String("i", "", "glob specifing files to include.")
	outfile := flag.String("o", "-", "filename of generated output. Default is stdout.")
	filetype := flag.String("t", "text", "type of file(s) input. one of: [text, bin]")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(),
			`go-include creates a .go file containing the text or binary (base64) content of 
files specified by the glob pattern.

USAGE: go-include [-o FILE] [-t (text|bin)] [FILE|GLOB]...
`)
		flag.PrintDefaults()
	}
	flag.Parse()

	// get list of filenames, check for errors (including no matches)
	files := []string{}
	for _, glob := range flag.Args() {
		filenames, err := filepath.Glob(glob)
		if err != nil {
			fmt.Fprintf(os.Stderr, "include: error: with glob '%s': %s\n", glob, err.Error())
			continue
		}
		if len(filenames) > 0 {
			files = append(files, filenames...)
		} else {
			fmt.Fprintf(os.Stderr, "include: found no files matching '%s'\n", glob)
		}
	}

	// if no args were given, read from stdin
	if len(flag.Args()) == 0 {
		files = append(files, os.Stdin.Name())
	}

	// exit if no files are found AND something was specified on the command line
	if len(files) < 1 {
		fmt.Fprintf(os.Stderr, "include: error: found no files matching glob(s) %s\n", flag.Args())
		return
	}

	// get name of package from which go:generate was run and use that for
	// the package of the generated resource
	pkgName := "main"
	if v, b := os.LookupEnv("GOPACKAGE"); b {
		pkgName = v
	}

	// build data structure sent to final output template
	d := data{
		Time:    time.Now().Format(time.RFC3339),
		Package: pkgName,
		Files:   []file{}}
	for _, f := range files {
		_, name := filepath.Split(f)
		name = strings.TrimSuffix(name, filepath.Ext(name))
		name = strings.Title(name)

		contentBytes, err := ioutil.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "include: error: could not open %s\n", f)
			continue
		}

		// convert binary data to base64 or "escape" backticks in text data
		var content string
		switch *filetype {
		case "bin":
			content = base64.StdEncoding.EncodeToString(contentBytes)
		case "text":
			content = string(contentBytes)
			content = strings.Replace(content, "`", "` + \"`\" + `", -1)
		}

		comment := fmt.Sprintf("%s was sourced from %s file %s", name, *filetype, f)

		d.Files = append(d.Files, file{Name: name, Content: content, Comment: comment})
	}

	// execute code generation template and put it in buf
	buf := new(bytes.Buffer)
	code := template.Must(template.New("").Parse(temp))
	code.Execute(buf, d)

	if *outfile != "-" {
		// add .go extension if necessary
		if !strings.HasSuffix(*outfile, ".go") {
			*outfile += ".go"
		}
		// write code to file
		ioutil.WriteFile(*outfile, buf.Bytes(), 0644)
	} else {
		fmt.Fprint(os.Stdout, buf.String())
	}
}

// data and template used above

type data struct {
	Time    string
	Package string
	Files   []file
}

type file struct {
	Name    string
	Content string
	Comment string
}

var temp = `// Generated code. DO NOT EDIT.
// Generated on {{ .Time }}

package {{ .Package }}

{{ range .Files -}}
{{ if ne .Comment "" }}{{ printf "// %s" .Comment }}{{ end }}
{{ printf "const %s = ` + "`%s`" + ` \n" .Name .Content }}
{{ end -}}`
