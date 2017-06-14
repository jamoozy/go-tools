package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/go-yaml/yaml"
)

func isStdIO(v string) bool {
	return v == "-"
}

var (
	iname = flag.String("i", "-", `Name of file to read from. "-" for stdin.`)
	oname = flag.String("o", "-", `Name of file to generate. "-" for stdout.`)
	ename = flag.String("e", "=", `Name of the file to log errors to. "-" for stderr.`)
)

func main() {
	flag.Parse()

	var (
		e   io.Writer
		i   io.Reader
		o   io.Writer
		err error
	)

	if isStdIO(*ename) {
		e = os.Stderr
	} else if e, err = os.Create(*ename); err != nil {
		fmt.Printf("Cannot open file %q: %s", *ename, err.Error())
		return
	}

	if isStdIO(*iname) {
		i = os.Stdin
	} else if i, err = os.Open(*iname); err != nil {
		fmt.Fprintf(e, err.Error())
		return
	}

	if isStdIO(*oname) {
		o = os.Stdout
	} else if o, err = os.Create(*oname); err != nil {
		fmt.Fprintf(e, err.Error())
		return
	}

	m, err := read(i)
	if err != nil {
		fmt.Fprintf(e, err.Error())
		return
	}

	fmt.Fprintf(o, "package main\n\n")
	fmt.Fprintf(o, "import (\n")
	fmt.Fprintf(o, ")\n\n")
	if _, err := printStruct(o, "", "TopLevelElement", m); err != nil {
		fmt.Fprintf(e, err.Error())
		return
	}
}

func printStruct(o io.Writer, prefix, name string, m map[string]interface{}) (string, error) {
	var (
		buf bytes.Buffer
		err error
	)

	fmt.Fprintf(o, "type %s struct {\n", strings.Title(name))
	for k, v := range m {
		var t string
		switch v := v.(type) {
		case float32, float64,
			int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64:
			t = "float64"

		case string:
			t = "string"

		case []interface{}:
			t, err = printSlice(&buf, prefix+"\t", k, v)
			if err != nil {
				return "", err
			}
			t = "[]" + t

		case map[string]interface{}:
			t, err = printStruct(&buf, prefix+"\t", k, v)
			if err != nil {
				return "", err
			}

		default:
			return "", fmt.Errorf("Unrecognized type: %T", v)
		}

		if _, err = fmt.Fprintf(o, "\t%s%s %s `json:%q`\n", prefix, strings.Title(k), t, k); err != nil {
			return "", err
		}
	}
	fmt.Fprintf(o, "} `json:%q`\n\n", name)

	if _, err = io.Copy(o, &buf); err != nil {
		return "", err
	}

	return strings.Title(name), nil
}

func printSlice(o io.Writer, prefix, name string, s []interface{}) (string, error) {
	var (
		t, last string
		buf     bytes.Buffer
		err     error
	)

	singularize := func(i string) string {
		return i
	}

	for _, v := range s {
		switch v := v.(type) {
		case float32, float64,
			int, int8, int16, int32, int64,
			uint, uint8, uint16, uint32, uint64:
			last = t
			t = "float64"

		case string:
			last = t
			t = "string"

		case []interface{}:
			last = t
			t, err = printSlice(&buf, prefix+"\t", name, v)
			if err != nil {
				return "", err
			}
			t = "[]" + t

		case map[string]interface{}:
			last = t
			t, err = printStruct(&buf, prefix+"\t", singularize(name), v)
			if err != nil {
				return "", err
			}

		default:
			return "", fmt.Errorf("Unrecognized type: %T", v)
		}
	}

	if last != "" && last != t {
		return "interface{}", nil
	}

	return t, nil
}

func read(i io.Reader) (map[string]interface{}, error) {
	b, err := ioutil.ReadAll(i)
	if err != nil {
		return nil, err
	}

	parsers := map[string]func(i []byte, o interface{}) error{
		"json": json.Unmarshal,
		"yaml": yaml.Unmarshal,
	}
	names := make([]string, 0, len(parsers))

	var m map[string]interface{}
	for t, f := range parsers {
		if err := f(b, &m); err == nil {
			return m, nil
		}
		names = append(names, t)
	}
	return nil, fmt.Errorf("Could not parse after trying parsers for: %q", names)
}
