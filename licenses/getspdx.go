// Copyright 2020 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build ignore

// Getspdx converts SPDX license definitions into license regular expressions (LREs).
//
// Usage:
//
//	go run getspdx.go [-f] name...
//
// Getspdx converts each JSON file into an LRE file id.lre, where id is the
// "licenseId" filed in the JSON file. If the "isDeprecatedField" in a JSON file
// is set to true, getspdx skips that file.
//
// Getspdx is only intended to provide a good start for the LRE for a given license.
// The result of the conversion still needs manual adjustment over time to deal
// with real-world variation (the SPDX patterns are not particularly forgiving).
//
// If id.lre already exists, getspdx skips the conversion instead of overwriting id.lre.
// If the -f flag is given, getspdx overwrites id.lre.
//
// As a special case, the name "all" means all non-deprecated SPDX licenses.
//
// Getcc expects to find the SPDX database checked out in _spdx,
// which you can do using:
//
//	git clone https://github.com/spdx/license-list-data _spdx

package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// spdx is the SPDX JSON data structure
type spdx struct {
	IsDeprecatedLicenseID   bool
	LicenseText             string
	StandardLicenseTemplate string
	Name                    string
	LicenseComments         string
	LicenseID               string
	SeeAlso                 []string
	IsOSIApproved           bool
}

var forceOverwrite = flag.Bool("f", false, "force overwrite")

func usage() {
	fmt.Fprintf(os.Stderr, "usage: go run getspdx.go [-f] name\n")
	os.Exit(2)
}

// Do not write these out even though they don't exist.
// They are generated by templates in other files,
// such as BSD.lre and MIT.lre, or we handle them differently,
// or we don't want them.
var exclude = fieldMap(`
	AGPL-1.0-only
	AGPL-1.0-or-later
	AGPL-3.0-only
	AGPL-3.0-or-later
	CAL-1.0-Combined-Work-Exception
	BSD-1-Clause
	BSD-1-Clause-Clear
	BSD-2-Clause
	BSD-2-Clause-FreeBSD
	BSD-2-Clause-Patent
	BSD-2-Clause-Views
	BSD-3-Clause
	BSD-3-Clause-Attribution
	BSD-3-Clause-Clear
	BSD-3-Clause-LBNL
	BSD-3-Clause-No-Nuclear-License
	BSD-3-Clause-No-Nuclear-License-2014
	BSD-3-Clause-No-Nuclear-Warranty
	BSD-3-Clause-NoTrademark
	BSD-3-Clause-Open-MPI
	BSD-4-Clause
	BSD-4-Clause-UC
	BSD-Protection
	BSD-Source-Code
	GFDL-1.1-invariants-only
	GFDL-1.1-invariants-or-later
	GFDL-1.1-no-invariants-only
	GFDL-1.1-no-invariants-or-later
	GFDL-1.1-only
	GFDL-1.1-or-later
	GFDL-1.2-invariants-only
	GFDL-1.2-invariants-or-later
	GFDL-1.2-no-invariants-only
	GFDL-1.2-no-invariants-or-later
	GFDL-1.2-only
	GFDL-1.2-or-later
	GFDL-1.3-invariants-only
	GFDL-1.3-invariants-or-later
	GFDL-1.3-no-invariants-only
	GFDL-1.3-no-invariants-or-later
	GFDL-1.3-only
	GFDL-1.3-or-later
	GPL-1.0-only
	GPL-1.0-or-later
	GPL-2.0-Or-3.0
	GPL-2.0-only
	GPL-2.0-or-later
	GPL-3.0-only
	GPL-3.0-or-later
	HPND-sell-variant
	LGPL-2.0-only
	LGPL-2.0-or-later
	LGPL-2.1-only
	LGPL-2.1-or-later
	LGPL-3.0-only
	LGPL-3.0-or-later
	MIT
	MIT-0
	MITNFA
	MIT-NoAd
	MPL-2.0-no-copyleft-exception
	OFL-1.0-RFN
	OFL-1.0-no-RFN
	OFL-1.1-RFN
	OFL-1.1-no-RFN
`)

func fieldMap(s string) map[string]bool {
	m := make(map[string]bool)
	for _, f := range strings.Fields(s) {
		m[f] = true
	}
	return m
}

var exitStatus int
var isAll bool

func main() {
	log.SetPrefix("getspdx: ")
	log.SetFlags(0)
	flag.Usage = usage
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		usage()
	}

	if info, err := os.Stat("_spdx"); err != nil || !info.IsDir() {
		log.Fatalf("expected SPDX database in _spdx; check out with:\n\tgit clone https://github.com/spdx/license-list-data _spdx")
	}

	if len(args) == 1 && args[0] == "all" {
		list, err := filepath.Glob("_spdx/json/details/*.json")
		if err != nil {
			log.Fatal(err)
		}
		args = nil
		for _, file := range list {
			args = append(args, strings.TrimSuffix(filepath.Base(file), ".json"))
		}
		isAll = true
	}

	for _, file := range args {
		convert(file)
	}

	cmd := exec.Command("go", "generate")
	cmd.Dir = ".."
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}

	os.Exit(exitStatus)
}

func convert(file string) {
	if !strings.HasSuffix(file, ".json") {
		file = "_spdx/json/details/" + file + ".json"
	}

	data, err := ioutil.ReadFile(file)
	if err != nil {
		log.Print(err)
		exitStatus = 1
		return
	}

	var info spdx
	if err := json.Unmarshal(data, &info); err != nil {
		log.Printf("%s: %v", file, err)
		exitStatus = 1
		return
	}

	id := info.LicenseID

	if info.IsDeprecatedLicenseID {
		if !isAll {
			log.Printf("%s: deprecated\n", id)
		}
		return
	}

	// println("FILE", info.LicenseID)
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "//**\n%s\nhttps://spdx.org/licenses/%s.json\n", info.Name, info.LicenseID)
	for _, url := range info.SeeAlso {
		fmt.Fprintf(&buf, "%s\n", url)
	}
	fmt.Fprintf(&buf, "**//\n\n")

	buf.WriteString(templateToLRE(file, info.StandardLicenseTemplate))

	if exclude[id] {
		return
	}
	target := id + ".lre"
	if _, err := os.Stat(target); err == nil && !*forceOverwrite {
		return
	}

	if err := ioutil.WriteFile(target, buf.Bytes(), 0666); err != nil {
		log.Print(err)
		exitStatus = 1
		return
	}

	i := strings.Index(file, "json/details")
	if i < 0 {
		log.Printf("cannot find spdx text")
		exitStatus = 1
		return
	}
	text, err := ioutil.ReadFile(file[:i] + "text/" + id + ".txt")
	if err != nil {
		log.Print(err)
		exitStatus = 1
		return
	}
	if err := ioutil.WriteFile("../testdata/licenses/"+id+".txt", text, 0666); err != nil {
		log.Print(err)
		exitStatus = 1
		return
	}
	if _, err := os.Stat("../testdata/" + id + ".t1"); err != nil {
		data := []byte(fmt.Sprintf("0%%\nscan\n100%%\n%s 100%% 0,$\n\n%s", id, text))
		if err := ioutil.WriteFile("../testdata/"+id+".t1", data, 0666); err != nil {
			log.Print(err)
			exitStatus = 1
			return
		}
	}
}

var (
	trailingSpaceRE = regexp.MustCompile(`(?m)[ \t]+$`)
	wordRE          = regexp.MustCompile(`[\d\w]+`)
	blankRE         = regexp.MustCompile(`\n([.," \t]*\n){2,}`)
)

func words(s string) []string {
	return wordRE.FindAllString(s, -1)
}

func wordCount(s string) int {
	return len(words(s))
}

// templateToLRE converts the SPDX template t to an LRE.
func templateToLRE(file string, t string) string {
	var buf bytes.Buffer

	start := 0
	var optStart []int
	for i := 0; i < len(t); {
		switch {
		case strings.HasPrefix(t[i:], "(("),
			strings.HasPrefix(t[i:], "||"),
			strings.HasPrefix(t[i:], "))"),
			strings.HasPrefix(t[i:], "//"),
			strings.HasPrefix(t[i:], "??"),
			strings.HasPrefix(t[i:], "__"):
			wrap(&buf, t[start:i+1])
			c := t[i]
			for i < len(t) && t[i] == c {
				i++
			}
			start = i

		case strings.HasPrefix(t[i:], "<<") && !strings.HasPrefix(t[i:], "<<<"):
			wrap(&buf, t[start:i])
			j := strings.Index(t[i:], ">>")
			if j < 0 {
				panic("bad template")
			}
			tag := t[i : i+j+2]
			i += j + 2
			if findAttr(tag, "original") == "name" {
				wrap(&buf, "name")
				start = i
				continue
			}
			for i < len(t) && t[i] == ' ' {
				i++
			}
			start = i
			switch {
			default:
				panic(tag)
			case tag == "<<beginOptional>>":
				optStart = append(optStart, buf.Len())
				indentNL(&buf)
				buf.WriteString("(( ")
			case tag == "<<endOptional>>":
				start := optStart[len(optStart)-1]
				optStart = optStart[:len(optStart)-1]
				if bytes.IndexByte(buf.Bytes()[start:], '\n') >= 0 {
					indentNL(&buf)
				} else {
					buf.WriteString(" ")
				}
				buf.WriteString("))??")
				indentNL(&buf)
				w := words(string(buf.Bytes()[start:]))
				// Don't emit options for punctuation,
				// which we don't match anyway,
				// and don't emit options for plural suffixes
				// like name((s))??.
				if len(w) == 0 || len(w) == 1 && w[0] == "s" {
					buf.Truncate(start)
				}
			case strings.HasPrefix(tag, `<<var;`):
				name := findAttr(tag, "name")
				original := findAttr(tag, "original")
				indentNL(&buf)
				switch name {
				case "copyright":
					fmt.Fprintf(&buf, "//** Copyright **//\n")

				default:
					n := wordCount(original)
					if n < 1 {
						n = 1
					}
					if name == "bullet" && n < 5 {
						if wordCount(original) > 0 {
							fmt.Fprintf(&buf, "(( %s ))??", original)
						} else {
							fmt.Fprintf(&buf, "%s ", original)
							continue
						}
						break
					}
					if name != "bullet" && n < 5 {
						n = 5
					}
					if original != "" {
						fmt.Fprintf(&buf, "//** %s **//", original)
						indentNL(&buf)
					}
					fmt.Fprintf(&buf, "__%d__", n)
				}
				indentNL(&buf)
			}

		default:
			i++
		}
	}
	wrap(&buf, t[start:])

	data := buf.Bytes()
	data = trailingSpaceRE.ReplaceAll(data, nil)
	data = blankRE.ReplaceAll(data, []byte("\n\n"))

	// We don't match the Copyright part (that's implied at the start),
	// so if there's anything ahead of it, cut it out and start afterward.
	i := bytes.Index(data, []byte("//**Copyright**//"))
	if i >= 0 && i < 100 {
		cut := bytes.TrimSpace(data[:i])
		if bytes.HasPrefix(cut, []byte("((")) && bytes.HasSuffix(cut, []byte("))??")) {
			goto NoCut
		}
		if len(cut) > 0 {
			fmt.Fprintf(os.Stderr, "%s: warning: %q before copyright notice\n", file, cut)
		}
	}
NoCut:

	for len(data) > 0 && data[0] == '\n' {
		data = data[1:]
	}
	if len(data) > 0 && data[len(data)-1] != '\n' {
		data = append(data, '\n')
	}

	return string(data)
}

func findAttr(tag, name string) string {
	i := strings.Index(tag, name+`="`)
	if i < 0 {
		return ""
	}
	tag = tag[i+len(name)+2:]
	j := strings.Index(tag, `"`)
	if j < 0 {
		return ""
	}
	return tag[:j]
}

// wrap adds literal text to the buffer buf, wrapping long lines.
// Wrapping is important for reading future diffs in the LRE files.
func wrap(buf *bytes.Buffer, text string) {
	all := buf.Bytes()
	i := len(all)
	for i > 0 && all[i-1] != '\n' {
		i--
	}
	buf.Truncate(i)
	lines := strings.SplitAfter(text, "\n")
	lines[0] = string(all[i:]) + lines[0]
	for _, line := range lines {
		i := 0
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		indent := line[:i]
		line = line[i:]
		const Target = 80
		for len(line) > Target-len(indent) {
			j := Target - len(indent)
			for j >= 0 && line[j] != ' ' && line[j] != '\t' {
				j--
			}
			if j < 0 {
				j = Target - len(indent)
				for j < len(line) && line[j] != ' ' && line[j] != '\t' {
					j++
				}
				if j == len(line) {
					break
				}
			}
			buf.WriteString(indent)
			buf.WriteString(line[:j])
			buf.WriteString("\n")
			for j < len(line) && (line[j] == ' ' || line[j] == '\t') {
				j++
			}
			line = line[j:]
		}
		buf.WriteString(indent)
		buf.WriteString(line)
	}
}

func indentNL(buf *bytes.Buffer) {
	all := buf.Bytes()
	i := len(all)
	for i > 0 && all[i-1] != '\n' {
		i--
	}
	j := i
	for j < len(all) && (all[j] == ' ' || all[j] == '\t') {
		j++
	}
	buf.WriteByte('\n')
	buf.Write(all[i:j])
}