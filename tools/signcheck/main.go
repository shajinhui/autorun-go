package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"autorun-go/api"
)

type input struct {
	Query map[string]string `json:"query"`
	Body  json.RawMessage   `json:"body"`
}

func main() {
	var inPath string
	var bodyPath string
	var queryPath string
	var rawBody string

	flag.StringVar(&inPath, "in", "", "Path to a JSON file with {query, body}")
	flag.StringVar(&bodyPath, "body", "", "Path to a raw body JSON file")
	flag.StringVar(&queryPath, "query", "", "Path to a query JSON file (object of string->string)")
	flag.StringVar(&rawBody, "raw", "", "Raw body JSON string (use with care)")
	flag.Parse()

	var query map[string]string
	var bodyStr string

	if inPath != "" {
		data, err := os.ReadFile(inPath)
		if err != nil {
			fatal(err)
		}
		var in input
		if err := json.Unmarshal(data, &in); err != nil {
			fatal(err)
		}
		query = in.Query
		bodyStr = string(in.Body)
	} else {
		if queryPath != "" {
			data, err := os.ReadFile(queryPath)
			if err != nil {
				fatal(err)
			}
			if err := json.Unmarshal(data, &query); err != nil {
				fatal(err)
			}
		}
		if bodyPath != "" {
			b, err := os.ReadFile(bodyPath)
			if err != nil {
				fatal(err)
			}
			bodyStr = string(b)
		} else if rawBody != "" {
			bodyStr = rawBody
		}
	}

	sign := api.GenerateSign(query, bodyStr)

	fmt.Println("SIGN:")
	fmt.Println(sign)
	fmt.Println()
	fmt.Println("QUERY:")
	printJSON(os.Stdout, query)
	fmt.Println()
	fmt.Println("BODY:")
	fmt.Println(bodyStr)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}

func printJSON(w io.Writer, v any) {
	if v == nil {
		fmt.Fprintln(w, "null")
		return
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		fmt.Fprintln(w, "<invalid json>")
		return
	}
	fmt.Fprintln(w, string(b))
}
